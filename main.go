// androidqf - Android Quick Forensics
// Copyright (c) 2021-2022 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/i582/cfmt/cmd/cfmt"
	"github.com/manifoldco/promptui"
	"github.com/mvt-project/androidqf/acquisition"
	"github.com/mvt-project/androidqf/adb"
	"github.com/mvt-project/androidqf/log"
	"github.com/mvt-project/androidqf/modules"
	"github.com/mvt-project/androidqf/utils"
)

type deviceMenuItem struct {
	Serial string
	Title  string
	Status string
}

func init() {
	cfmt.Print(`
	{{                    __           _     __      ____ }}::green
	{{   ____  ____  ____/ /________  (_)___/ /___  / __/ }}::yellow
	{{  / __ '/ __ \/ __  / ___/ __ \/ / __  / __ '/ /_   }}::red
	{{ / /_/ / / / / /_/ / /  / /_/ / / /_/ / /_/ / __/   }}::magenta
	{{ \__,_/_/ /_/\__,_/_/   \____/_/\__,_/\__, /_/      }}::blue
	{{                                        /_/         }}::cyan
	`)
	cfmt.Println("\tandroidqf - Android Quick Forensics")
	cfmt.Println()
}

func systemPause() {
	cfmt.Println("Press {{Enter}}::bold|green to finish ...")
	os.Stdin.Read(make([]byte, 1))
}

func buildDeviceMenuItems(devices []adb.DeviceInfo, running map[string]runningExtraction) []deviceMenuItem {
	items := make([]deviceMenuItem, 0, len(devices))
	for _, device := range devices {
		item := deviceMenuItem{
			Serial: device.Serial,
			Title:  deviceMenuTitle(device),
			Status: deviceMenuStatus(device),
		}
		if state, ok := running[device.Serial]; ok {
			if item.Status != "" {
				item.Status += " "
			}
			item.Status += fmt.Sprintf("(extraction running, pid %d, started %s)", state.PID, state.Started.Local().Format("2006-01-02 15:04:05"))
		}
		items = append(items, item)
	}
	return items
}

func deviceMenuTitle(device adb.DeviceInfo) string {
	name := device.Model
	if name == "" {
		name = device.Device
	}
	if name == "" {
		name = device.Product
	}
	name = strings.ReplaceAll(name, "_", " ")
	if name == "" {
		return device.Serial
	}
	return fmt.Sprintf("%s (%s)", name, device.Serial)
}

func deviceMenuStatus(device adb.DeviceInfo) string {
	if device.State == "" || device.State == "device" {
		return ""
	}
	return fmt.Sprintf("(%s)", device.State)
}

func selectADBDeviceFromMenu(items []deviceMenuItem) (string, error) {
	promptDevice := promptui.Select{
		Label: "Multiple Android devices detected. Select the device to acquire",
		Items: items,
		Templates: &promptui.SelectTemplates{
			Active:   "> {{ .Title | cyan }} {{ .Status | yellow }}",
			Inactive: "  {{ .Title }} {{ .Status }}",
			Selected: "{{ .Title }}",
		},
	}

	index, _, err := promptDevice.Run()
	if err != nil {
		return "", fmt.Errorf("failed to select ADB device: %v", err)
	}

	return items[index].Serial, nil
}

func resolveADBSerial(serial string, devices []adb.DeviceInfo, selectDevice func([]deviceMenuItem) (string, error), running map[string]runningExtraction) (string, bool, error) {
	serial = strings.TrimSpace(serial)
	if serial != "" || len(devices) == 0 {
		return serial, false, nil
	}
	if len(devices) == 1 {
		return devices[0].Serial, false, nil
	}

	selectedSerial, err := selectDevice(buildDeviceMenuItems(devices, running))
	return selectedSerial, true, err
}

func errorOnDeviceSelection([]deviceMenuItem) (string, error) {
	return "", fmt.Errorf("multiple devices detected, use -serial to select one")
}

func buildOptions(fast, nonInteractive bool, backup, download, removeTrusted, intrusionLogs, hashFiles, moduleFilter string) (*modules.Options, error) {
	opts := &modules.Options{Fast: fast, NonInteractive: nonInteractive}
	var err error
	if backup != "" {
		if opts.Backup, err = modules.ParseBackupOption(backup); err != nil {
			return nil, err
		}
	}
	if download != "" {
		if opts.Download, err = modules.ParseDownloadOption(download); err != nil {
			return nil, err
		}
	}
	if removeTrusted != "" {
		if opts.RemoveTrusted, err = modules.ParseRemoveTrustedOption(removeTrusted); err != nil {
			return nil, err
		}
	}
	if intrusionLogs != "" {
		if opts.IntrusionLogs, err = modules.ParseIntrusionLogsOption(intrusionLogs); err != nil {
			return nil, err
		}
	}
	if hashFiles != "" {
		if opts.HashFiles, err = modules.ParseHashFilesOption(hashFiles); err != nil {
			return nil, err
		}
	}
	if err = modules.ValidateNonInteractive(opts, moduleFilter); err != nil {
		return nil, err
	}
	return opts, nil
}

func main() {
	var err error
	var verbose bool
	var version_flag bool
	var list_modules bool
	var fast bool
	var module string
	var output_folder string
	var serial string
	var tcpAddr string
	var backupFlag string
	var downloadFlag string
	var removeTrustedFlag string
	var intrusionLogsFlag string
	var hashFilesFlag string
	var nonInteractive bool

	// Command line options
	flag.BoolVar(&verbose, "verbose", false, "Verbose mode")
	flag.BoolVar(&verbose, "v", false, "Verbose mode")
	flag.BoolVar(&fast, "fast", false, "Fast mode")
	flag.BoolVar(&fast, "f", false, "Fast mode")
	flag.BoolVar(&list_modules, "list", false, "List modules and exit")
	flag.BoolVar(&list_modules, "l", false, "List modules and exit")
	flag.StringVar(&module, "module", "", "Only execute a specific module")
	flag.StringVar(&module, "m", "", "Only execute a specific module")
	flag.StringVar(&output_folder, "output", "", "Output folder")
	flag.StringVar(&output_folder, "o", "", "Output folder")
	flag.StringVar(&serial, "serial", "", "Phone serial number")
	flag.StringVar(&serial, "s", "", "Phone serial number")
	flag.StringVar(&tcpAddr, "connect", "", "Connect to device over network using ip:port")
	flag.StringVar(&tcpAddr, "c", "", "Connect to device over network using ip:port")
	flag.StringVar(&backupFlag, "backup", "", "Answer the backup prompt: sms, all or none (sms/all still require a tap on the device to authorize)")
	flag.StringVar(&backupFlag, "b", "", "Answer the backup prompt: sms, all or none (sms/all still require a tap on the device to authorize)")
	flag.StringVar(&downloadFlag, "download", "", "Answer the APK download prompt: all, non-system or none")
	flag.StringVar(&downloadFlag, "d", "", "Answer the APK download prompt: all, non-system or none")
	flag.StringVar(&removeTrustedFlag, "remove-trusted", "", "Answer the trusted-APK removal prompt: yes or no (ignored with -download none)")
	flag.StringVar(&removeTrustedFlag, "r", "", "Answer the trusted-APK removal prompt: yes or no (ignored with -download none)")
	flag.StringVar(&intrusionLogsFlag, "intrusion-logs", "", "Answer the Intrusion Logs prompt: yes or no (yes still requires taps on the device to download new logs)")
	flag.StringVar(&intrusionLogsFlag, "i", "", "Answer the Intrusion Logs prompt: yes or no (yes still requires taps on the device to download new logs)")
	flag.StringVar(&hashFilesFlag, "hash-files", "", "Answer the on-device file hashing prompt: yes or no (resource-intensive)")
	flag.StringVar(&hashFilesFlag, "H", "", "Answer the on-device file hashing prompt: yes or no (resource-intensive)")
	flag.BoolVar(&nonInteractive, "non-interactive", false, "Never prompt: fail if a prompt would be reached without its flag and skip the final 'Press Enter'")
	flag.BoolVar(&nonInteractive, "n", false, "Never prompt: fail if a prompt would be reached without its flag and skip the final 'Press Enter'")
	flag.BoolVar(&version_flag, "version", false, "Show version")

	flag.Parse()
	if verbose {
		log.SetLogLevel(log.DEBUG)
	}

	if version_flag {
		log.Infof("AndroidQF version: %s", utils.Version)
		os.Exit(0)
	}

	if list_modules {
		mods := modules.List()
		log.Info("List of modules:")
		for _, mod := range mods {
			log.Infof("- %s", mod.Name())
		}
		os.Exit(0)
	}

	opts, err := buildOptions(fast, nonInteractive, backupFlag, downloadFlag, removeTrustedFlag, intrusionLogsFlag, hashFilesFlag, module)
	if err != nil {
		log.Fatal(err)
	}

	log.Debug("Starting androidqf")
	adb.Client, err = adb.New()
	if err != nil {
		log.Fatal("Impossible to initialize ADB: ", err)
	}

	if tcpAddr != "" {
		log.Infof("Attempting to connect to %s over network...", tcpAddr)
		out, err := adb.Client.Exec("connect", tcpAddr)
		if err != nil {
			log.Error(fmt.Sprintf("Failed to connect to %s: %v", tcpAddr, err))
		} else {
			log.Infof("ADB connect output: %s", strings.TrimSpace(string(out)))
			// If no serial was explicitly provided, use the ip:port as the serial
			if serial == "" {
				serial = tcpAddr
			}
		}
	}
	specificDeviceRequested := serial != ""

	selectDevice := selectADBDeviceFromMenu
	if nonInteractive {
		selectDevice = errorOnDeviceSelection
	}

	// Initialization
	for {
		if serial == "" {
			devices, err := adb.Client.DeviceInfos()
			if err != nil {
				if nonInteractive {
					log.Fatal("Error listing ADB devices: ", err)
				}
				log.Error(fmt.Sprintf("Error listing ADB devices: %s", err))
			} else {
				serial, _, err = resolveADBSerial(serial, devices, selectDevice, activeRunningExtractionsBySerial())
				if err != nil {
					if nonInteractive {
						log.Fatal("Error selecting ADB device: ", err)
					}
					log.Error(fmt.Sprintf("Error selecting ADB device: %s", err))
					time.Sleep(5 * time.Second)
					continue
				}
			}
		}

		serial, err = adb.Client.SetSerial(serial)
		if err != nil {
			if nonInteractive {
				log.Fatal("Error trying to connect over ADB: ", err)
			}
			log.Error(fmt.Sprintf("Error trying to connect over ADB: %s", err))
			if !specificDeviceRequested {
				serial = ""
			}
		} else {
			_, err = adb.Client.GetState()
			if err == nil {
				break
			}
			log.Debug(err)
			if nonInteractive {
				log.Fatal("Unable to get device state: ", err)
			}
			log.Error("Unable to get device state. Please make sure it is connected and authorized. Trying again in 5 seconds...")
			if !specificDeviceRequested {
				serial = ""
			}
		}
		time.Sleep(5 * time.Second)
	}

	acq, err := acquisition.New(output_folder)
	if err != nil {
		log.Debug(err)
		log.FatalExc("Impossible to initialise the acquisition", err)
	}

	releaseRunning, err := registerRunningExtraction(adb.Client.Serial, acq.StoragePath)
	if err != nil {
		log.Warningf("Unable to record running extraction state: %v", err)
		releaseRunning = func() {}
	}
	runningReleased := false
	defer func() {
		if !runningReleased {
			releaseRunning()
		}
	}()

	// Start acquisitions
	log.Info(fmt.Sprintf("Started new acquisition archive in %s", acq.StoragePath))

	signals := make(chan os.Signal, 2)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	opts.Signals = signals

	mods := modules.List()
	failedModules := 0
	interrupted := false
	for _, mod := range mods {
		if !modules.ModuleEnabled(mod.Name(), module) {
			continue
		}

		moduleStarted := time.Now().UTC()
		err = mod.Run(acq, opts)
		result := acquisition.ModuleResult{
			Name:      mod.Name(),
			Status:    "completed",
			Started:   moduleStarted,
			Completed: time.Now().UTC(),
		}
		if err != nil {
			result.Status = "failed"
			result.Error = err.Error()
			failedModules++
			log.Infof("ERROR: failed to run module %s: %v", mod.Name(), err)
		}
		acq.ModuleResults = append(acq.ModuleResults, result)

		if errors.Is(err, modules.ErrAcquisitionInterrupted) {
			interrupted = true
		} else {
			select {
			case received := <-signals:
				log.Warningf("Received %s; stopping after module %s and finalizing the partial acquisition.", received, mod.Name())
				interrupted = true
			default:
			}
		}
		if interrupted {
			break
		}
	}

	log.Info("Finalizing acquisition archive...")
	if err := acq.Complete(); err != nil {
		releaseRunning()
		runningReleased = true
		log.FatalExc("Failed to finalize acquisition archive", err)
	}
	releaseRunning()
	runningReleased = true
	if interrupted {
		log.Fatal("Acquisition was interrupted and finalized as partial.")
	}
	if failedModules > 0 {
		log.Fatalf("Acquisition finalized with %d failed module(s). Review acquisition.json and command.log for details.", failedModules)
	}
	log.Info("Acquisition completed.")

	if !nonInteractive {
		systemPause()
	}
}
