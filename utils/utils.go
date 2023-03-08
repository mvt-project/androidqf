// pcqf - PC Quick Forensics
// Copyright (c) 2021 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package utils

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"time"
)

// GetBinFolder returns the folder containing the binary.
func GetBinFolder() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}

	return path.Dir(exe)
}

func AskForConfirmation(s string) bool {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("%s [y/n]: ", s)

		response, err := reader.ReadString('\n')
		if err != nil {
			log.Fatal(err)
		}

		response = strings.ToLower(strings.TrimSpace(response))

		if response == "y" || response == "yes" {
			return true
		} else if response == "n" || response == "no" {
			return false
		}
	}
}

func FmtDuration(d time.Duration) string {
	seconds := int(d.Seconds())
	s := seconds - int(d.Minutes())*60
	return fmt.Sprintf("%02dm%02ds", int(d.Minutes()), s)
}
