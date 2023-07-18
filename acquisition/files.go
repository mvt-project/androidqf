// androidqf - Android Quick Forensics
// Copyright (c) 2021 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package acquisition

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mvt/androidqf/adb"
)

func (a *Acquisition) FindFullCommand(path string) ([]adb.FileInfo, error) {
	var results []adb.FileInfo
	out, err := adb.Client.Shell("find", fmt.Sprintf("'%s'", path), "-type", "f", "-printf", "'%T@ %m %s %u %g %p\n'", "2>", "/dev/null")

	if err == nil {
		return results, err
	}

	for _, line := range strings.Split(out, "\n") {
		var new_file adb.FileInfo
		s := strings.Fields(line)
		if len(s) == 0 {
			continue
		}
		time, err := strconv.ParseFloat(s[0], 64)
		if err == nil {
			new_file.ModifiedTime = int64(time)
		}
		new_file.Mode = s[1]
		size, err := strconv.ParseInt(s[2], 10, 64)
		if err == nil {
			new_file.Size = size
		}
		new_file.UserName = s[3]
		new_file.GroupName = s[4]
		new_file.Path = strings.Join(s[5:], "/")

		results = append(results, new_file)
	}

	return results, nil
}

func (a *Acquisition) FindLimitedCommand(path string) ([]adb.FileInfo, error) {
	var results []adb.FileInfo
	out, err := adb.Client.Shell("find", fmt.Sprintf("'%s'", path), "-type", "f", "2>", "/dev/null")
	if err != nil {
		return results, err
	}

	for _, line := range strings.Split(out, "\n") {
		var new_file adb.FileInfo
		new_file.Path = line
		results = append(results, new_file)
	}

	return results, nil
}
