// androidqf - Android Quick Forensics
// Copyright (c) 2021-2022 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
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

	// Command line options
	flag.BoolVar(&verbose, "verbose", false, "Verbose mode")
	flag.BoolVar(&verbose, "v", false, "Verbose mode")
	flag.BoolVar(&fast, "fast", false, "Fast mode")
	flag.BoolVar(&verbose, "f", false, "Fast mode")
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

	// Initialization
	for {
		if serial == "" {
			devices, err := adb.Client.DeviceInfos()
			if err != nil {
				log.Error(fmt.Sprintf("Error listing ADB devices: %s", err))
			} else {
				serial, _, err = resolveADBSerial(serial, devices, selectADBDeviceFromMenu, activeRunningExtractionsBySerial())
				if err != nil {
					log.Error(fmt.Sprintf("Error selecting ADB device: %s", err))
					time.Sleep(5 * time.Second)
					continue
				}
			}
		}

		serial, err = adb.Client.SetSerial(serial)
		if err != nil {
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
			log.Error("Unable to get device state. Please make sure it is connected and authorized. Trying again in 5 seconds...")
			if !specificDeviceRequested {
				serial = ""
			}
		}
		time.Sleep(5 * time.Second)
	}

	releaseRunning, err := registerRunningExtraction(adb.Client.Serial, "")
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

	acq, err := acquisition.New(output_folder)
	if err != nil {
		log.Debug(err)
		log.FatalExc("Impossible to initialise the acquisition", err)
	}

	// Start acquisitions
	log.Info(fmt.Sprintf("Started new acquisition in %s", acq.StoragePath))

	mods := modules.List()
	for _, mod := range mods {
		if (module != "") && (module != mod.Name()) {
			continue
		}
		err = mod.InitStorage(acq.StoragePath)
		if err != nil {
			log.Infof(
				"ERROR: failed to initialize storage for module %s: %v",
				mod.Name(),
				err,
			)
			continue
		}

		err = mod.Run(acq, fast)
		if err != nil {
			log.Infof("ERROR: failed to run module %s: %v", mod.Name(), err)
		}
	}

	if acq.StreamingMode {
		// In streaming mode, all data is already encrypted in the zip stream
		log.Info("Finalizing encrypted acquisition...")
	} else {
		// Traditional mode: hash files, then encrypt if key exists
		err = acq.HashFiles()
		if err != nil {
			log.ErrorExc("Failed to generate list of file hashes", err)
			return
		}

		err = acq.StoreInfo()
		if err != nil {
			log.ErrorExc("Failed to store acquisition info", err)
			return
		}

		err = acq.StoreSecurely()
		if err != nil {
			log.ErrorExc("Something failed while encrypting the acquisition", err)
			log.Warning("WARNING: The secure storage of the acquisition folder failed! The data is unencrypted!")
		}
	}

	acq.Complete()
	releaseRunning()
	runningReleased = true
	log.Info("Acquisition completed.")

	systemPause()
}
