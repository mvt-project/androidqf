package adb

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// PackageUser describes an enumerated Android user and package collection
// outcome. Running does not establish whether the user's storage is unlocked.
type PackageUser struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Flags   string `json:"flags,omitempty"`
	Running bool   `json:"running"`
	Status  string `json:"inventory_status"`
	Error   string `json:"error,omitempty"`
}

type PackageUsersReport struct {
	Users            []PackageUser `json:"users"`
	EnumerationError string        `json:"enumeration_error,omitempty"`
}

var packageUserLine = regexp.MustCompile(`^UserInfo\{([0-9]+):(.*):([0-9a-fA-F]+)\}(.*)$`)

func parsePackageUsers(out string) ([]PackageUser, error) {
	users := []PackageUser{}
	seen := map[int]bool{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "Users:" {
			continue
		}
		match := packageUserLine.FindStringSubmatch(line)
		if match == nil {
			return nil, fmt.Errorf("unrecognized user-list output %q", line)
		}
		id, err := strconv.Atoi(match[1])
		if err != nil || seen[id] {
			return nil, fmt.Errorf("invalid or duplicate user ID %q", match[1])
		}
		seen[id] = true
		users = append(users, PackageUser{ID: id, Name: match[2], Flags: match[3], Running: strings.Contains(match[4], "running")})
	}
	if len(users) == 0 {
		return nil, fmt.Errorf("user-list output contained no users")
	}
	sort.Slice(users, func(i, j int) bool { return users[i].ID < users[j].ID })
	return users, nil
}

// GetPackagesWithUsers scopes every package query to an explicit user. An
// unscoped Android query can collapse several users into a comma-separated UID
// list, and pm path otherwise defaults to a different user than the inventory.
// This method never starts, unlocks or switches Android users.
func (a *ADB) GetPackagesWithUsers(fast bool) ([]Package, PackageUsersReport, error) {
	packages := []Package{}
	report := PackageUsersReport{Users: []PackageUser{}}
	out, enumErr := a.Shell("pm", "list", "users")
	if enumErr == nil {
		report.Users, enumErr = parsePackageUsers(out)
	}
	var collectionErr error
	if enumErr != nil {
		report.Users = []PackageUser{}
		collectionErr = fmt.Errorf("enumerating Android users: %w: %s", enumErr, out)
		report.EnumerationError = collectionErr.Error()
		// Preserve collection of the known current user when an OEM refuses
		// enumeration. Report the missing coverage rather than assuming user 0.
		current, currentErr := a.Shell("am", "get-current-user")
		id, parseErr := strconv.Atoi(strings.TrimSpace(current))
		if currentErr != nil || parseErr != nil || id < 0 {
			return packages, report, errors.Join(collectionErr, fmt.Errorf("cannot determine current Android user: %q: %v", current, currentErr))
		}
		report.Users = []PackageUser{{ID: id, Running: true}}
	}
	for i := range report.Users {
		user := &report.Users[i]
		userPackages, err := a.getPackagesForUser(user.ID, fast)
		packages = append(packages, userPackages...)
		user.Status = "collected"
		if err != nil {
			user.Status = "partial"
			if len(userPackages) == 0 {
				user.Status = "failed"
			}
			user.Error = err.Error()
			collectionErr = errors.Join(collectionErr, fmt.Errorf("user %d: %w", user.ID, err))
		}
	}
	return packages, report, collectionErr
}

func (a *ADB) getPackagesForUser(userID int, fast bool) ([]Package, error) {
	userArg := strconv.Itoa(userID)
	attempts := []packageListAttempt{
		{args: []string{"pm", "list", "packages", "-U", "-u", "-i", "--user", userArg}, withInstaller: true},
		{args: []string{"pm", "list", "packages", "-U", "-u", "--user", userArg}},
	}
	var entries []packageListEntry
	var listErr error
	for _, attempt := range attempts {
		out, err := a.Shell(attempt.args...)
		if err == nil {
			if strings.TrimSpace(out) == "" {
				return []Package{}, nil
			}
			entries, err = parsePackageList(out, attempt.withInstaller)
		}
		if err == nil {
			listErr = nil
			break
		}
		listErr = fmt.Errorf("package inventory: %w: %s", err, out)
	}
	if listErr != nil {
		return nil, listErr
	}

	// Keep -u's historical records without claiming those packages remain
	// installed for this user. Permission failures leave Installed unknown.
	installed, installedErr := a.packageNamesForUser(userArg, "")
	disabled, disabledErr := a.packageNamesForUser(userArg, "-d")
	system, systemErr := a.packageNamesForUser(userArg, "-s")
	thirdParty, thirdPartyErr := a.packageNamesForUser(userArg, "-3")
	collectionErr := errors.Join(installedErr, disabledErr, systemErr, thirdPartyErr)
	packages := make([]Package, 0, len(entries))
	for _, entry := range entries {
		p := Package{UserID: userID, Name: entry.name, Installer: entry.installer, UID: entry.uid,
			Disabled: disabled[entry.name], System: system[entry.name], ThirdParty: thirdParty[entry.name], Files: []PackageFile{}}
		if installedErr == nil {
			value := installed[entry.name]
			p.Installed = &value
		}
		if p.Installed == nil || *p.Installed {
			files, err := a.getPackageFiles(entry.name, userID, fast)
			p.Files = files
			if err != nil {
				p.FilesError = err.Error()
				collectionErr = errors.Join(collectionErr, fmt.Errorf("package %s: %w", entry.name, err))
			}
		}
		packages = append(packages, p)
	}
	return packages, collectionErr
}

func (a *ADB) packageNamesForUser(userArg, filter string) (map[string]bool, error) {
	args := []string{"pm", "list", "packages", "--user", userArg}
	if filter != "" {
		args = append(args, filter)
	}
	out, err := a.Shell(args...)
	if err != nil {
		return nil, fmt.Errorf("package state %q: %w: %s", filter, err, out)
	}
	names := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "package:") || strings.ContainsAny(strings.TrimPrefix(line, "package:"), " \t") || line == "package:" {
			return nil, fmt.Errorf("unrecognized package-state output %q", line)
		}
		names[strings.TrimPrefix(line, "package:")] = true
	}
	return names, nil
}
