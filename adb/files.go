// androidqf - Android Quick Forensics
// Copyright (c) 2021 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package adb

import (
	"fmt"
	"strconv"
	"strings"
)

func (a *ADB) FindFullCommand(path string) ([]FileInfo, error) {
	out, err := a.Shell(
		"find",
		QuoteRemoteShellArg(path),
		"-type", "f",
		"-printf", `'%T@\t%m\t%s\t%u\t%g\t%p\0'`,
		"2>", "/dev/null",
	)
	if err != nil {
		return nil, err
	}
	return parseFullFindOutput(out)
}

func parseFullFindOutput(out string) ([]FileInfo, error) {
	var results []FileInfo
	for _, record := range strings.Split(out, "\x00") {
		if record == "" {
			continue
		}
		fields := strings.SplitN(record, "\t", 6)
		if len(fields) != 6 {
			return nil, fmt.Errorf("malformed find output record %q", record)
		}

		modified, err := strconv.ParseFloat(fields[0], 64)
		if err != nil {
			return nil, fmt.Errorf("invalid modification time in find output %q: %w", record, err)
		}
		size, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid size in find output %q: %w", record, err)
		}
		results = append(results, FileInfo{
			ModifiedTime: int64(modified),
			Mode:         fields[1],
			Size:         size,
			UserName:     fields[3],
			GroupName:    fields[4],
			Path:         fields[5],
		})
	}
	return results, nil
}

func (a *ADB) FindLimitedCommand(path string) ([]FileInfo, error) {
	var results []FileInfo
	out, err := a.Shell("find", QuoteRemoteShellArg(path), "-type", "f", "2>", "/dev/null")
	if err != nil {
		return results, err
	}

	for _, filePath := range strings.Split(out, "\n") {
		if filePath != "" {
			results = append(results, FileInfo{Path: filePath})
		}
	}

	return results, nil
}

// QuoteRemoteShellArg quotes a value for use as one argument in an adb shell
// command.
func QuoteRemoteShellArg(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}
