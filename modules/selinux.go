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

type SELinux struct {
	StoragePath string
}

func NewSELinux() *SELinux {
	return &SELinux{}
}

func (s *SELinux) Name() string {
	return "selinux"
}

func (s *SELinux) InitStorage(storagePath string) error {
	s.StoragePath = storagePath
	return nil
}

func (s *SELinux) Run(acq *acquisition.Acquisition, fast bool) error {
	log.Info("Collecting SELinux status...")

	out, err := adb.Client.Shell("getenforce")
	if err != nil {
		return fmt.Errorf("failed to run `adb shell getenforce`: %v", err)
	}

	return saveStringToAcquisition(acq, "selinux.txt", out)
}
