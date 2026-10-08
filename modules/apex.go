// androidqf - Android Quick Forensics
// Use of this source code is governed by the MVT License 1.1
// which can be found in the LICENSE file.

package modules

import (
	"encoding/xml"
	"errors"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/mvt-project/androidqf/acquisition"
	"github.com/mvt-project/androidqf/adb"
	"github.com/mvt-project/androidqf/log"
)

const apexInventoryPath = "/apex/apex-info-list.xml"

var apexFactoryRoots = []string{
	"/system/apex", "/system_ext/apex", "/product/apex",
	"/vendor/apex", "/odm/apex", "/oem/apex",
}

type Apex struct{}

func NewApex() *Apex         { return &Apex{} }
func (a *Apex) Name() string { return "apex" }

// Keep inventory relationships separately from files: a factory file can also
// be active, and multiple inventory records can refer to the same container.
type apexModuleInfo struct {
	Name             string `xml:"moduleName,attr" json:"module_name"`
	Path             string `xml:"modulePath,attr" json:"module_path"`
	PreinstalledPath string `xml:"preinstalledModulePath,attr" json:"preinstalled_module_path,omitempty"`
	VersionCode      string `xml:"versionCode,attr" json:"version_code,omitempty"`
	VersionName      string `xml:"versionName,attr" json:"version_name,omitempty"`
	IsFactory        *bool  `xml:"isFactory,attr" json:"is_factory,omitempty"`
	IsActive         *bool  `xml:"isActive,attr" json:"is_active,omitempty"`
}

type apexInventory struct {
	XMLName xml.Name         `xml:"apex-info-list"`
	Modules []apexModuleInfo `xml:"apex-info"`
}

type apexFile struct {
	DevicePath  string `json:"device_path"`
	ArchivePath string `json:"archive_path,omitempty"`
	Status      string `json:"status"`
	Method      string `json:"acquisition_method,omitempty"`
	Error       string `json:"error,omitempty"`
}

type apexManifest struct {
	SchemaVersion int              `json:"schema_version"`
	Status        string           `json:"status"`
	Inventory     string           `json:"inventory_source,omitempty"`
	Modules       []apexModuleInfo `json:"modules"`
	Files         []apexFile       `json:"files"`
	Errors        []string         `json:"errors,omitempty"`
}

func (a *Apex) Run(acq *acquisition.Acquisition, opts *Options) error {
	if adb.Client == nil {
		return fmt.Errorf("ADB client is unavailable")
	}
	manifest := apexManifest{SchemaVersion: 1, Status: "completed", Modules: []apexModuleInfo{}, Files: []apexFile{}}
	var collectionErr error
	recordError := func(err error) {
		manifest.Errors = append(manifest.Errors, err.Error())
		collectionErr = errors.Join(collectionErr, err)
	}
	finish := func() error {
		var interrupted error
		if err := opts.ContextOrBackground().Err(); err != nil {
			if !errors.Is(collectionErr, err) {
				recordError(err)
			}
			interrupted = fmt.Errorf("%w: %v", ErrAcquisitionInterrupted, err)
		}
		if collectionErr != nil {
			manifest.Status = "partial"
		}
		return errors.Join(interrupted, partialCollectionError(collectionErr), saveDataToAcquisition(acq, "apex/manifest.json", &manifest))
	}
	if sdk, err := adb.Client.Shell("getprop", "ro.build.version.sdk"); err == nil {
		if version, err := strconv.Atoi(sdk); err == nil && version > 0 && version < 29 {
			manifest.Status = "not_supported"
			return finish()
		}
	}
	log.Info("Collecting APEX containers for offline analysis. This can increase acquisition size and duration...")

	// Preserve the original inventory, including attributes not understood here.
	raw, inventoryErr := adb.Client.Exec("exec-out", "cat", apexInventoryPath)
	if len(raw) > 0 {
		if err := acq.ZipWriter.CreateFileFromBytes("apex/apex-info-list.xml", raw); err != nil {
			return err
		}
	}
	var inventory apexInventory
	if inventoryErr == nil {
		inventoryErr = xml.Unmarshal(raw, &inventory)
	}
	paths := make(map[string]struct{})
	if inventoryErr == nil {
		manifest.Inventory = apexInventoryPath
		manifest.Modules = append(manifest.Modules, inventory.Modules...)
		for _, module := range inventory.Modules {
			if module.Name == "" || module.Path == "" {
				recordError(fmt.Errorf("APEX inventory entry is missing its module name or path"))
			}
			for _, devicePath := range []string{module.Path, module.PreinstalledPath} {
				if devicePath != "" {
					paths[devicePath] = struct{}{}
				}
			}
		}
	} else {
		// A fallback can recover containers, but cannot reproduce all of the
		// factory/active relationships from an unavailable or invalid inventory.
		recordError(fmt.Errorf("read APEX inventory: %w", inventoryErr))
		manifest.Inventory = "package_manager_and_factory_directories"
		out, err := adb.Client.Shell("pm", "list", "packages", "--apex-only", "-f")
		if saveErr := saveStringToAcquisition(acq, "apex/packages.txt", out); saveErr != nil {
			return saveErr
		}
		if err != nil {
			recordError(fmt.Errorf("list APEX packages: %w", err))
		}
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "package:") {
				continue
			}
			devicePath, name, ok := strings.Cut(strings.TrimPrefix(line, "package:"), "=")
			if !ok || name == "" || devicePath == "" {
				recordError(fmt.Errorf("malformed APEX package record %q", line))
				continue
			}
			manifest.Modules = append(manifest.Modules, apexModuleInfo{Name: name, Path: devicePath})
			paths[devicePath] = struct{}{}
		}
		// Fixed roots only; never walk the mounted payloads under /apex.
		command := "for d in " + strings.Join(apexFactoryRoots, " ") + "; do " +
			"if [ -d \"$d\" ]; then find \"$d\" -mindepth 1 -maxdepth 1 -print0 || exit; fi; done"
		listing, err := adb.Client.Exec("shell", command)
		if err != nil {
			recordError(fmt.Errorf("list factory APEX directories: %w", err))
		}
		for _, devicePath := range strings.Split(string(listing), "\x00") {
			if devicePath != "" {
				paths[devicePath] = struct{}{}
			}
		}
	}

	ordered := make([]string, 0, len(paths))
	for devicePath := range paths {
		ordered = append(ordered, devicePath)
	}
	sort.Strings(ordered)
	rootChecked, hasRoot := false, false
	for _, devicePath := range ordered {
		if err := opts.ContextOrBackground().Err(); err != nil {
			return finish()
		}
		entry := apexFile{DevicePath: devicePath, Status: "failed"}
		archivePath, err := apexArchivePath(devicePath)
		if err == nil {
			// Probe before staging so a failed archive write is never retried as
			// root, which could otherwise create duplicate or incomplete entries.
			quoted := adb.QuoteRemoteShellArg(devicePath)
			probe := "if [ -d " + quoted + " ]; then printf directory; elif [ -f " + quoted + " ] && [ -r " + quoted + " ]; then printf file; else printf inaccessible; fi"
			kind, probeErr := adb.Client.Shell(probe)
			useRoot := false
			if probeErr == nil && kind == "inaccessible" {
				if !rootChecked {
					hasRoot, rootChecked = adb.Client.HasRoot(), true
				}
				if hasRoot {
					kind, probeErr = adb.Client.RootShell(probe)
					useRoot = true
				}
			}
			switch {
			case probeErr != nil:
				err = fmt.Errorf("inspect APEX path: %w", probeErr)
			case kind == "directory":
				entry.Status = "directory"
				// Flattened APEX directories do not contain signed containers.
				// Keep their inventory metadata without recursively copying them.
			case kind != "file":
				err = fmt.Errorf("APEX container is missing or unreadable")
			case path.Ext(devicePath) != ".apex" && path.Ext(devicePath) != ".capex":
				err = fmt.Errorf("unsupported APEX container extension")
			default:
				entry.Method = "adb"
				if useRoot {
					entry.Method = "su"
					err = acq.PullRootToZipStaged(devicePath, archivePath)
				} else {
					err = acq.PullToZipStaged(devicePath, archivePath)
				}
				if err == nil {
					entry.Status, entry.ArchivePath = "collected", archivePath
				}
			}
		}
		if err != nil {
			entry.Error = err.Error()
			recordError(fmt.Errorf("%s: %w", devicePath, err))
			log.Warningf("Unable to collect APEX %s: %v", devicePath, err)
		}
		manifest.Files = append(manifest.Files, entry)
	}
	return finish()
}

func apexArchivePath(devicePath string) (string, error) {
	if path.Clean(devicePath) != devicePath || strings.ContainsAny(devicePath, "\\\x00") {
		return "", fmt.Errorf("unsafe APEX path %q", devicePath)
	}
	roots := append([]string{"/data/apex"}, apexFactoryRoots...)
	for _, root := range roots {
		if _, err := relativeDeviceChild(root, devicePath); err == nil {
			return "apex/files/" + strings.TrimPrefix(devicePath, "/"), nil
		}
	}
	return "", fmt.Errorf("APEX path is outside supported container directories: %q", devicePath)
}
