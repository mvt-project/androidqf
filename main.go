// androidqf - Android Quick Forensics
// Copyright (c) 2021-2022 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/i582/cfmt/cmd/cfmt"
	"github.com/mvt-project/androidqf/acquisition"
	"github.com/mvt-project/androidqf/adb"
	"github.com/mvt-project/androidqf/log"
	"github.com/mvt-project/androidqf/modules"
	"github.com/mvt-project/androidqf/utils"
)

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

func main() {
	var err error
	var verbose bool
	var version_flag bool
	var list_modules bool
	var fast bool
	var module string
	var output_folder string
	var serial string

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
	adb.Client, err = adb.New(serial)
	if err != nil {
		log.Fatal("Impossible to initialize adb: ", err)
	}

	// Initialization
	for {
		_, err = adb.Client.GetState()
		if err == nil {
			break
		}
		log.Debug(err)
		log.Error("Unable to get device state. Please make sure it is connected and authorized. Trying again in 5 seconds...")
		time.Sleep(5 * time.Second)
	}

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

	err = acq.HashFiles()
	if err != nil {
		log.ErrorExc("Failed to generate list of file hashes", err)
		return
	}

	acq.Complete()
	acq.StoreInfo()

	err = acq.StoreSecurely()
	if err != nil {
		log.ErrorExc("Something failed while encrypting the acquisition", err)
		log.Warning("WARNING: The secure storage of the acquisition folder failed! The data is unencrypted!")
	}

	log.Info("Acquisition completed.")

	systemPause()
}
