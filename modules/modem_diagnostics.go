// androidqf - Android Quick Forensics
// Use of this source code is governed by the MVT License 1.1
// which can be found in the LICENSE file.

package modules

import (
	"context"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
	"github.com/mvt-project/androidqf/acquisition"
	"github.com/mvt-project/androidqf/adb"
	"github.com/mvt-project/androidqf/log"
)

const (
	acquireModemDiagnostics = "Yes"
	skipModemDiagnostics    = "No"
)

// Probe actual access rather than inferring permissions from the manufacturer
// or chipset. Firmware, SELinux policy and prior exports affect availability.
// See docs/modem-diagnostics.md for the sources and scope of this catalogue.
var modemDiagnosticRoots = []string{
	"/sdcard/diag_logs",
	"/sdcard/log",
	"/sdcard/Download",
	"/sdcard/Android/data/com.android.pixellogger/files/logs",
	"/data/diag_logs",
	"/data/vendor/radio/diag_logs",
	"/data/vendor/radio/logs",
	"/data/vendor/slog",
	"/data/log",
}

type ModemDiagnostics struct{}

func NewModemDiagnostics() *ModemDiagnostics { return &ModemDiagnostics{} }
func (m *ModemDiagnostics) Name() string     { return "modem_diagnostics" }

type modemDiagnosticSource struct {
	DevicePath string `json:"device_path"`
	Status     string `json:"status"`
	Method     string `json:"acquisition_method"`
	Error      string `json:"error,omitempty"`
}

type modemDiagnosticFile struct {
	DevicePath  string `json:"device_path"`
	ArchivePath string `json:"archive_path,omitempty"`
	Format      string `json:"format"`
	Status      string `json:"status"`
	Method      string `json:"acquisition_method,omitempty"`
	Error       string `json:"error,omitempty"`
}

type modemDiagnosticsManifest struct {
	SchemaVersion int                     `json:"schema_version"`
	GeneratedAt   time.Time               `json:"generated_at"`
	Status        string                  `json:"status"`
	RootAccess    string                  `json:"su_access"`
	Properties    map[string]string       `json:"device_properties"`
	Sources       []modemDiagnosticSource `json:"sources"`
	Files         []modemDiagnosticFile   `json:"files"`
	Errors        []string                `json:"errors,omitempty"`
}

func ParseModemDiagnosticsOption(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "yes":
		return acquireModemDiagnostics, nil
	case "no":
		return skipModemDiagnostics, nil
	}
	return "", fmt.Errorf("invalid -modem-diagnostics value %q (valid values: yes, no)", value)
}

func (m *ModemDiagnostics) Run(acq *acquisition.Acquisition, opts *Options) error {
	selection, err := resolveOption(opts, opts.ModemDiagnostics, "-modem-diagnostics (yes, no)", func() (string, error) {
		log.Info("Collect existing modem diagnostic logs? They may contain sensitive communications and location data and can be large. Existing root access is used only if needed; logging is never enabled.")
		prompt := promptui.Select{Label: "Modem diagnostics", Items: []string{skipModemDiagnostics, acquireModemDiagnostics}}
		_, selection, err := prompt.Run()
		return selection, err
	})
	if err != nil {
		return fmt.Errorf("failed to make selection for modem diagnostics: %w", err)
	}
	if selection == skipModemDiagnostics {
		log.Info("Skipping modem diagnostic logs...")
		return nil
	}
	if selection != acquireModemDiagnostics {
		return fmt.Errorf("invalid modem diagnostics selection %q", selection)
	}
	if adb.Client == nil {
		return fmt.Errorf("ADB client is unavailable")
	}
	return m.collect(acq, opts, modemDiagnosticRoots)
}

func (m *ModemDiagnostics) collect(acq *acquisition.Acquisition, opts *Options, roots []string) error {
	ctx := opts.ContextOrBackground()
	client := &adb.ADB{ExePath: adb.Client.ExePath, Serial: adb.Client.Serial}
	client.SetContext(ctx)
	manifest := modemDiagnosticsManifest{
		SchemaVersion: 1, GeneratedAt: time.Now().UTC(), Status: "no_logs", RootAccess: "not_checked",
		Properties: map[string]string{}, Sources: []modemDiagnosticSource{}, Files: []modemDiagnosticFile{},
	}
	var collectionErr error
	accessLimited, collected := false, false
	recordError := func(err error) {
		manifest.Errors = append(manifest.Errors, err.Error())
		collectionErr = errors.Join(collectionErr, err)
	}
	finish := func() error {
		var interrupted error
		switch {
		case ctx.Err() != nil:
			manifest.Status = "interrupted"
			interrupted = fmt.Errorf("%w: %v", ErrAcquisitionInterrupted, ctx.Err())
		case collectionErr != nil:
			manifest.Status = "partial"
		case accessLimited:
			manifest.Status = "access_limited"
			log.Warning("Some modem log locations are inaccessible. See modem_diagnostics/manifest.json; absence of collected logs does not establish absence of diagnostic data.")
		case collected:
			manifest.Status = "collected"
		}
		return errors.Join(interrupted, partialCollectionError(collectionErr), saveDataToAcquisition(acq, "modem_diagnostics/manifest.json", &manifest))
	}
	// Do not restart adbd, install su, or wait indefinitely on a root manager's
	// authorization dialog. An unattended acquisition requires pre-authorized su.
	hasRoot := func() bool {
		if manifest.RootAccess == "not_checked" && ctx.Err() == nil {
			rootCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			probe := &adb.ADB{ExePath: client.ExePath, Serial: client.Serial}
			probe.SetContext(rootCtx)
			manifest.RootAccess = "unavailable"
			if probe.HasRoot() {
				manifest.RootAccess = "available"
			}
		}
		return manifest.RootAccess == "available"
	}
	if ctx.Err() != nil {
		return finish()
	}
	log.Info("Collecting existing modem diagnostic logs without changing device logging settings...")
	properties, err := client.Shell("getprop")
	if err != nil {
		recordError(fmt.Errorf("read modem device properties: %w", err))
	}
	for _, key := range []string{"ro.product.manufacturer", "ro.product.model", "ro.board.platform", "ro.hardware", "ro.soc.manufacturer", "ro.soc.model", "ro.build.fingerprint", "gsm.version.baseband"} {
		for _, line := range strings.Split(properties, "\n") {
			if value, ok := strings.CutPrefix(line, "["+key+"]: ["); ok {
				manifest.Properties[key] = strings.TrimSuffix(strings.TrimSuffix(value, "\r"), "]")
			}
		}
	}

	seen := make(map[string]bool)
	for _, root := range roots {
		if ctx.Err() != nil {
			return finish()
		}
		source := modemDiagnosticSource{DevicePath: root, Method: "adb"}
		state, probeErr := probeModemPath(client, root, true, false)
		useRoot := false
		if probeErr == nil && state == "inaccessible" && hasRoot() {
			useRoot, source.Method = true, "su"
			state, probeErr = probeModemPath(client, root, true, true)
		}
		source.Status = state
		var listing string
		if probeErr == nil && state == "readable" {
			listing, probeErr = listModemLogs(client, root, useRoot)
			// A readable directory can contain protected subdirectories. Retain
			// partial shell output even if the root retry also fails.
			if probeErr != nil && !useRoot && hasRoot() {
				var rootListing string
				rootListing, probeErr = listModemLogs(client, root, true)
				listing += rootListing
				source.Method = "su"
			}
			source.Status = "listed"
		}
		if state == "inaccessible" {
			accessLimited = true
		}
		if probeErr != nil {
			source.Status, source.Error = "partial", probeErr.Error()
			recordError(fmt.Errorf("list modem logs in %s: %w", root, probeErr))
		}
		manifest.Sources = append(manifest.Sources, source)
		names := strings.Split(listing, "\x00")
		sort.Strings(names)
		for _, name := range names {
			if ctx.Err() != nil {
				return finish()
			}
			if name == "" {
				continue
			}
			format := modemDiagnosticFormat(name)
			if format == "" {
				continue
			}
			devicePath, archivePath, err := modemDiagnosticPaths(root, name)
			if err != nil {
				recordError(err)
				continue
			}
			if seen[devicePath] {
				continue
			}
			seen[devicePath] = true
			entry := modemDiagnosticFile{DevicePath: devicePath, Format: format, Status: "failed", Method: "adb"}
			// Always try ordinary ADB for the file, even after a root listing.
			// Decide access before staging: never retry a failed archive write.
			state, err := probeModemPath(client, devicePath, false, false)
			useRoot := false
			if err == nil && state == "inaccessible" && hasRoot() {
				useRoot, entry.Method = true, "su"
				state, err = probeModemPath(client, devicePath, false, true)
			}
			if err == nil && state != "readable" {
				err = fmt.Errorf("file is %s", state)
			}
			if err == nil {
				if useRoot {
					err = acq.PullRootToZipStaged(devicePath, archivePath)
				} else {
					err = acq.PullToZipStaged(devicePath, archivePath)
				}
			}
			if err != nil {
				entry.Error = err.Error()
				recordError(fmt.Errorf("collect modem log %s: %w", devicePath, err))
			} else {
				entry.Status, entry.ArchivePath = "collected", archivePath
				collected = true
			}
			manifest.Files = append(manifest.Files, entry)
		}
	}
	return finish()
}

func probeModemPath(client *adb.ADB, devicePath string, directory, root bool) (string, error) {
	q := adb.QuoteRemoteShellArg(devicePath)
	parent := adb.QuoteRemoteShellArg(path.Dir(devicePath))
	kind, wrongKind, readable, prefix := "-f", "not_regular", "[ -r "+q+" ]", ""
	if directory {
		kind, wrongKind, readable = "-d", "not_directory", readable+" && [ -x "+q+" ]"
	} else {
		prefix = "if [ -L " + q + " ]; then printf symlink; exit; fi; "
	}
	command := prefix + "if [ " + kind + " " + q + " ]; then if " + readable + "; then printf readable; else printf inaccessible; fi; " +
		"elif [ -e " + q + " ] || [ -L " + q + " ]; then printf " + wrongKind + "; " +
		"elif [ -r " + parent + " ] && [ -x " + parent + " ]; then printf not_found; else printf inaccessible; fi"
	out, err := modemShell(client, command, root)
	if err != nil {
		return "failed", err
	}
	state := strings.TrimSpace(out)
	switch state {
	case "readable", "inaccessible", "not_found", "not_directory", "not_regular", "symlink":
		return state, nil
	default:
		return "failed", fmt.Errorf("unexpected modem path probe response %q", state)
	}
}

func listModemLogs(client *adb.ADB, root string, asRoot bool) (string, error) {
	// Follow the fixed root's symlink (notably /sdcard), but no symlinks within
	// it. NUL delimiters and raw Exec preserve whitespace in device filenames.
	out, err := modemShell(client, "cd "+adb.QuoteRemoteShellArg(root)+" && find . -type f -print0", asRoot)
	if out != "" && !strings.HasSuffix(out, "\x00") {
		out = out[:strings.LastIndex(out, "\x00")+1]
		err = errors.Join(err, fmt.Errorf("incomplete modem log directory listing"))
	}
	return out, err
}

func modemShell(client *adb.ADB, command string, root bool) (string, error) {
	args := []string{"shell", command}
	if root {
		args = []string{"shell", "su", "-c", adb.QuoteRemoteShellArg(command)}
	}
	out, err := client.Exec(args...)
	return string(out), err
}

func modemDiagnosticPaths(root, name string) (string, string, error) {
	rel, ok := strings.CutPrefix(name, "./")
	if !ok || rel == "" || strings.ContainsAny(rel, "\\:\x00") {
		return "", "", fmt.Errorf("unsafe modem log path %q", name)
	}
	for _, component := range strings.Split(rel, "/") {
		if component == "" || component == "." || component == ".." {
			return "", "", fmt.Errorf("unsafe modem log path %q", name)
		}
	}
	devicePath := root + "/" + rel
	if _, err := relativeDeviceChild(root, devicePath); err != nil {
		return "", "", err
	}
	return devicePath, "modem_diagnostics/files/" + strings.TrimPrefix(devicePath, "/"), nil
}

func modemDiagnosticFormat(devicePath string) string {
	name := strings.ToLower(path.Base(devicePath))
	// Keep compressed logs intact. A suffix is a discovery hint, not a claim
	// that the content is valid or can be decoded by SCAT/QCSuper.
	switch path.Ext(name) {
	case ".gz", ".zip", ".lz4", ".zst":
		name = strings.TrimSuffix(name, path.Ext(name))
	}
	switch ext := path.Ext(name); ext {
	case ".qmdl", ".qmdl2", ".dlf", ".sdm", ".lpd":
		return strings.TrimPrefix(ext, ".")
	}
	return ""
}
