// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this source code is governed by the MVT License 1.1
// which can be found in the LICENSE file.

package modules

import (
	"fmt"
	"path/filepath"

	"github.com/mvt-project/androidqf/acquisition"
	"github.com/mvt-project/androidqf/adb"
	"github.com/mvt-project/androidqf/log"
)

type GetProp struct {
	StoragePath string
}

func NewGetProp() *GetProp {
	return &GetProp{}
}

func (g *GetProp) Name() string {
	return "getprop"
}

func (g *GetProp) InitStorage(storagePath string) error {
	g.StoragePath = storagePath
	return nil
}

func (g *GetProp) Run(acq *acquisition.Acquisition, fast bool) error {
	log.Info("Collecting device properties...")

	out, err := adb.Client.Shell("getprop")
	if err != nil {
		return fmt.Errorf("failed to run `adb shell getprop`: %v", err)
	}

	return saveCommandOutput(acq.Fs, filepath.Join(g.StoragePath, "getprop.txt"), out)
}
