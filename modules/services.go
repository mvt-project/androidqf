// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this source code is governed by the MVT License 1.1
// which can be found in the LICENSE file.

package modules

import (
	"fmt"

	"github.com/mvt-project/androidqf/acquisition"
	"github.com/mvt-project/androidqf/adb"
	"github.com/mvt-project/androidqf/log"
)

type Services struct {
	StoragePath string
}

func NewServices() *Services {
	return &Services{}
}

func (s *Services) Name() string {
	return "services"
}

func (s *Services) InitStorage(storagePath string) error {
	s.StoragePath = storagePath
	return nil
}

func (s *Services) Run(acq *acquisition.Acquisition, fast bool) error {
	log.Info("Collecting list of services...")

	out, err := adb.Client.Shell("service list")
	if err != nil {
		return fmt.Errorf("failed to run `adb shell service list`: %v", err)
	}

	return saveStringToAcquisition(acq, "services.txt", out)
}
