// androidqf - Android Quick Forensics
// Copyright (c) 2021-2022 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package assets

import (
	_ "embed"
)

//go:embed "adb"
var adbData []byte

//go:embed collector_arm6
var collectorArm6 []byte

//go:embed collector_arm7
var collectorArm7 []byte

func getAssets() []Asset {
	return []Asset{
		{Name: "adb", Data: adbData},
		{Name: "collector_arm6", Data: collectorArm6},
		{Name: "collector_arm7", Data: collectorArm7},
	}
}
