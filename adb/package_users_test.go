package adb

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type packageReply struct {
	Output string
	Fail   bool
}

func packageFixture(t *testing.T, replies map[string]packageReply) (*ADB, string) {
	t.Helper()
	client := newFakeADB(t, "")
	file := filepath.Join(t.TempDir(), "replies.json")
	data, err := json.Marshal(replies)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, data, 0o600); err != nil {
		t.Fatal(err)
	}
	calls := filepath.Join(t.TempDir(), "calls.txt")
	t.Setenv("ANDROIDQF_FAKE_PACKAGE_FIXTURE", file)
	t.Setenv("ANDROIDQF_FAKE_PACKAGE_CALLS", calls)
	return client, calls
}

func addUserReplies(replies map[string]packageReply, id, uid int, name string, disabled bool) {
	user := fmt.Sprint(id)
	replies["shell pm list packages -U -u -i --user "+user] = packageReply{Output: fmt.Sprintf("package:%s installer=null uid:%d\n", name, uid)}
	for _, filter := range []string{"", " -d", " -s", " -3"} {
		out := ""
		if filter == "" || filter == " -3" || filter == " -d" && disabled {
			out = "package:" + name
		}
		replies["shell pm list packages --user "+user+filter] = packageReply{Output: out}
	}
	replies["shell pm path --user "+user+" '"+name+"'"] = packageReply{Output: "package:/data/app/" + name + "/base.apk"}
}

func TestPackageInventoryScopesUIDsFlagsAndPathsByUser(t *testing.T) {
	replies := map[string]packageReply{
		"shell pm list users": {Output: "Users:\n UserInfo{0:Owner:4c13} running\n UserInfo{10:Secondary:410} running\n"},
		// This is actual AVD output. No correct path should ever query it.
		"shell pm list packages -U -u -i": {Output: "package:org.shared installer=null uid:10235,1010235"},
	}
	addUserReplies(replies, 0, 10235, "org.shared", false)
	addUserReplies(replies, 10, 1010235, "org.shared", true)
	client, calls := packageFixture(t, replies)
	packages, report, err := client.GetPackagesWithUsers(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 2 || packages[0].UserID != 0 || packages[1].UserID != 10 || packages[0].UID != 10235 || packages[1].UID != 1010235 {
		t.Fatalf("lost per-user identity: %+v", packages)
	}
	if packages[0].Disabled || !packages[1].Disabled || packages[0].Installed == nil || !*packages[0].Installed {
		t.Fatalf("wrong per-user package state: %+v", packages)
	}
	if len(report.Users) != 2 || report.Users[0].Status != "collected" || report.Users[1].Status != "collected" {
		t.Fatalf("report = %+v", report)
	}
	commands, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	for _, command := range strings.Split(strings.TrimSpace(string(commands)), "\n") {
		if strings.HasPrefix(command, "shell pm list packages") || strings.HasPrefix(command, "shell pm path") {
			if !strings.Contains(command, "--user ") {
				t.Fatalf("unscoped query: %s", command)
			}
		}
	}
}

func TestPackageInventoryKeepsAccessibleUserWhenSecondaryDenied(t *testing.T) {
	replies := map[string]packageReply{
		"shell pm list users":                       {Output: "Users:\nUserInfo{0:Owner:13} running\nUserInfo{10:Secondary:10}\n"},
		"shell pm list packages -U -u -i --user 10": {Output: "SecurityException: Shell does not have permission to access user 10", Fail: true},
		"shell pm list packages -U -u --user 10":    {Output: "SecurityException: Shell does not have permission to access user 10", Fail: true},
	}
	addUserReplies(replies, 0, 10123, "org.main", false)
	client, _ := packageFixture(t, replies)
	packages, report, err := client.GetPackagesWithUsers(true)
	if err == nil || len(packages) != 1 || packages[0].Name != "org.main" {
		t.Fatalf("packages = %+v, error = %v", packages, err)
	}
	if report.Users[0].Status != "collected" || report.Users[1].Status != "failed" || !strings.Contains(report.Users[1].Error, "SecurityException") {
		t.Fatalf("denied user not reported: %+v", report)
	}
}

func TestPackageInventoryEnumerationFallbackUsesKnownCurrentUser(t *testing.T) {
	replies := map[string]packageReply{
		"shell pm list users":       {Output: "Permission denied", Fail: true},
		"shell am get-current-user": {Output: "12"},
	}
	addUserReplies(replies, 12, 1210123, "org.current", false)
	client, _ := packageFixture(t, replies)
	packages, report, err := client.GetPackagesWithUsers(true)
	if err == nil || len(packages) != 1 || packages[0].UserID != 12 || report.EnumerationError == "" {
		t.Fatalf("packages = %+v, report = %+v, error = %v", packages, report, err)
	}
}

func TestPackageInventoryInstallerFallbackRemainsScoped(t *testing.T) {
	replies := map[string]packageReply{"shell pm list users": {Output: "Users:\nUserInfo{10:Secondary:10}"}}
	addUserReplies(replies, 10, 1010123, "org.secondary", false)
	replies["shell pm list packages -U -u -i --user 10"] = packageReply{Output: "unsupported -i", Fail: true}
	replies["shell pm list packages -U -u --user 10"] = packageReply{Output: "package:org.secondary uid:1010123"}
	client, _ := packageFixture(t, replies)
	packages, _, err := client.GetPackagesWithUsers(true)
	if err != nil || len(packages) != 1 || packages[0].Installer != "" || packages[0].UserID != 10 {
		t.Fatalf("packages = %+v, error = %v", packages, err)
	}
}

func TestPackageInventoryEmptyUserIsSuccessful(t *testing.T) {
	replies := map[string]packageReply{
		"shell pm list users":                       {Output: "Users:\nUserInfo{10:Secondary:10}"},
		"shell pm list packages -U -u -i --user 10": {},
	}
	client, _ := packageFixture(t, replies)
	packages, report, err := client.GetPackagesWithUsers(true)
	if err != nil || len(packages) != 0 || report.Users[0].Status != "collected" {
		t.Fatalf("packages = %+v, report = %+v, error = %v", packages, report, err)
	}
}

func TestPackageInventoryDoesNotDownloadUninstalledHistoricalRecord(t *testing.T) {
	replies := map[string]packageReply{"shell pm list users": {Output: "Users:\nUserInfo{10:Secondary:10}"}}
	addUserReplies(replies, 10, 1010123, "org.removed", false)
	replies["shell pm list packages --user 10"] = packageReply{}
	delete(replies, "shell pm path --user 10 'org.removed'")
	client, _ := packageFixture(t, replies)
	packages, _, err := client.GetPackagesWithUsers(true)
	if err != nil || len(packages) != 1 || packages[0].Installed == nil || *packages[0].Installed || len(packages[0].Files) != 0 {
		t.Fatalf("packages = %+v, error = %v", packages, err)
	}
}

func TestParsePackageUsers(t *testing.T) {
	users, err := parsePackageUsers("Users:\n UserInfo{10:Name:with:colons:410}\n UserInfo{0:Owner:4c13} running\n")
	if err != nil || len(users) != 2 || users[0].ID != 0 || !users[0].Running || users[1].Name != "Name:with:colons" || users[1].Running {
		t.Fatalf("users = %+v, error = %v", users, err)
	}
	for _, out := range []string{"", "Users:", "Error: permission denied", "UserInfo{0:Owner:13}\nUserInfo{0:Duplicate:13}", "UserInfo{-1:Invalid:13}"} {
		if _, err := parsePackageUsers(out); err == nil {
			t.Errorf("accepted invalid user list %q", out)
		}
	}
}

func TestPackageInventoryRetainsFileAccessFailure(t *testing.T) {
	replies := map[string]packageReply{"shell pm list users": {Output: "Users:\nUserInfo{10:Secondary:10}"}}
	addUserReplies(replies, 10, 1010123, "org.secondary", false)
	replies["shell pm path --user 10 'org.secondary'"] = packageReply{Output: "Permission denied", Fail: true}
	client, _ := packageFixture(t, replies)
	packages, report, err := client.GetPackagesWithUsers(true)
	if err == nil || len(packages) != 1 || packages[0].FilesError == "" || len(packages[0].Files) != 0 || report.Users[0].Status != "partial" {
		t.Fatalf("packages = %+v, report = %+v, error = %v", packages, report, err)
	}
}

func TestPackageInventoryLeavesInstallationStateUnknownOnFailure(t *testing.T) {
	replies := map[string]packageReply{"shell pm list users": {Output: "Users:\nUserInfo{10:Secondary:10}"}}
	addUserReplies(replies, 10, 1010123, "org.secondary", false)
	replies["shell pm list packages --user 10"] = packageReply{Output: "Permission denied", Fail: true}
	client, _ := packageFixture(t, replies)
	packages, report, err := client.GetPackagesWithUsers(true)
	if err == nil || len(packages) != 1 || packages[0].Installed != nil || report.Users[0].Status != "partial" {
		t.Fatalf("packages = %+v, report = %+v, error = %v", packages, report, err)
	}
	data, err := json.Marshal(packages[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"installed"`) {
		t.Fatalf("unknown installation state was serialized as fact: %s", data)
	}
}

func TestPackageInventoryRejectsEmptyAndDiagnosticAPKPaths(t *testing.T) {
	for _, out := range []string{"", "Error: permission denied", "not-a-package-path"} {
		t.Run(out, func(t *testing.T) {
			replies := map[string]packageReply{"shell pm list users": {Output: "Users:\nUserInfo{10:Secondary:10}"}}
			addUserReplies(replies, 10, 1010123, "org.secondary", false)
			replies["shell pm path --user 10 'org.secondary'"] = packageReply{Output: out}
			client, _ := packageFixture(t, replies)
			packages, _, err := client.GetPackagesWithUsers(true)
			if err == nil || len(packages) != 1 || packages[0].FilesError == "" || len(packages[0].Files) != 0 {
				t.Fatalf("packages = %+v, error = %v", packages, err)
			}
		})
	}
}
