// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this source code is governed by the MVT License 1.1
// which can be found in the LICENSE file.

package modules

import (
	"errors"
	"fmt"

	"github.com/mvt-project/androidqf/acquisition"
	"github.com/mvt-project/androidqf/adb"
	"github.com/mvt-project/androidqf/log"
)

type Settings struct{}

func NewSettings() *Settings {
	return &Settings{}
}

func (s *Settings) Name() string {
	return "settings"
}

func (s *Settings) Run(acq *acquisition.Acquisition, opts *Options) error {
	log.Info("Collecting device settings...")
	var collectionErr error

	for _, namespace := range []string{"system", "secure", "global"} {
		out, err := adb.Client.Shell(fmt.Sprintf("cmd settings list %s", namespace))
		if err != nil {
			return fmt.Errorf("failed to run `cmd settings %s`: %v", namespace, err)
		}

		err = saveStringToAcquisition(acq, fmt.Sprintf("settings_%s.txt", namespace), out)
		if err != nil {
			log.Errorf("Impossible to save settings: %v", err)
			collectionErr = errors.Join(collectionErr, fmt.Errorf("%s: %w", namespace, err))
		}
	}

	return partialCollectionError(collectionErr)
}
