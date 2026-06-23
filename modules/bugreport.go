// androidqf - Android Quick Forensics
// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package modules

import (
	"fmt"

	"github.com/mvt-project/androidqf/acquisition"
	"github.com/mvt-project/androidqf/log"
)

type Bugreport struct{}

func NewBugreport() *Bugreport {
	return &Bugreport{}
}

func (b *Bugreport) Name() string {
	return "bugreport"
}

func (b *Bugreport) Run(acq *acquisition.Acquisition, fast bool) error {
	log.Info(
		"Generating a bugreport for the device...",
	)

	err := acq.StreamBugreportToZip("bugreport.zip")
	if err != nil {
		return fmt.Errorf("failed to stream bugreport to archive: %v", err)
	}

	log.Debug("Bugreport completed!")

	return nil
}
