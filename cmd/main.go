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
	"github.com/mvt/androidqf/log"
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
	var acq *acquisition.Acquisition
	var err error

	verbose := flag.Bool("verbose", false, "Verbose mode")
	flag.Parse()

	if *verbose {
		log.SetLogLevel(log.DEBUG)
	}

	// TODO: add version information
	log.Debug("Starting androidqf")

	// Initialization
	for {
		acq, err = acquisition.New()
		if err == nil {
			break
		}
		log.Debug(err)
		log.Error("Unable to get device state. Please make sure it is connected and authorized. Trying again in 5 seconds...")
		time.Sleep(5 * time.Second)
	}

	err = acq.Initialize()
	if err != nil {
		log.Debug(err)
		log.ErrorExc("Impossible to initialise the acquisition", err)
		return
	}

	// Start acquisitions
	log.Info(fmt.Sprintf("Started new acquisition %s", acq.UUID))

	// Start with acquisitions that require user interaction
	err = acq.Backup()
	if err != nil {
		log.ErrorExc("Failed to create backup", err)
	}
	err = acq.DownloadAPKs()
	if err != nil {
		log.ErrorExc("Failed to download APKs", err)
	}
	err = acq.GetProp()
	if err != nil {
		log.ErrorExc("Failed to get device properties", err)
	}
	err = acq.Settings()
	if err != nil {
		log.ErrorExc("Failed to get device settings", err)
	}
	err = acq.Processes()
	if err != nil {
		log.ErrorExc("Failed to get list of running processes", err)
	}
	err = acq.GetEnv()
	if err != nil {
		log.ErrorExc("Failed to get list of environment variables", err)
	}
	err = acq.Services()
	if err != nil {
		log.ErrorExc("Failed to get list of running services", err)
	}
	err = acq.Logcat()
	if err != nil {
		log.ErrorExc("Failed to get logcat from device", err)
	}
	err = acq.Logs()
	if err != nil {
		log.ErrorExc("Failed to download logs from device", err)
	}
	err = acq.DumpSys()
	if err != nil {
		log.ErrorExc("Failed to get output of dumpsys", err)
	}
	err = acq.GetFiles()
	if err != nil {
		log.ErrorExc("Failed to get a list of files", err)
	}
	err = acq.GetTmpFolder()
	if err != nil {
		log.ErrorExc("Failed to get files in tmp folder", err)
	}
	err = acq.HashFiles()
	if err != nil {
		log.ErrorExc("Failed to generate list of file hashes", err)
		return
	}

	acq.Complete()

	err = acq.StoreSecurely()
	if err != nil {
		log.ErrorExc("Something failed while encrypting the acquisition", err)
		log.Warning("WARNING: The secure storage of the acquisition folder failed! The data is unencrypted!")
	}

	log.Info("Acquisition completed.")

	systemPause()
}
