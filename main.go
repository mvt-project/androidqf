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
	"github.com/mvt/androidqf/acquisition"
	"github.com/mvt/androidqf/adb"
	"github.com/mvt/androidqf/log"
	"github.com/mvt/androidqf/modules"
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

	// Configure vrebose mode
	verbose := flag.Bool("verbose", false, "Verbose mode")
	flag.Parse()
	if *verbose {
		log.SetLogLevel(log.DEBUG)
	}

	// TODO: add version information
	log.Debug("Starting androidqf")
	adb.Client, err = adb.New()
	if err != nil {
		log.Fatal("Impossible to initialize adb")
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

	acq, err := acquisition.New()
	if err != nil {
		log.Debug(err)
		log.FatalExc("Impossible to initialise the acquisition", err)
	}

	// Start acquisitions
	log.Info(fmt.Sprintf("Started new acquisition %s", acq.UUID))

	mods := modules.List()
	for _, mod := range mods {
		err = mod.InitStorage(acq.StoragePath)
		if err != nil {
			log.Infof(
				"ERROR: failed to initialize storage for module %s: %v",
				mod.Name(),
				err,
			)
			continue
		}

		err = mod.Run(acq)
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
