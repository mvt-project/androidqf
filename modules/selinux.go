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

type SELinux struct{}

type selinuxPolicyFile struct {
	remotePath  string
	archivePath string
	optional    bool
}

var selinuxPolicyFiles = []selinuxPolicyFile{
	{
		remotePath:  "/odm/etc/selinux/precompiled_sepolicy",
		archivePath: "selinux/odm/etc/selinux/precompiled_sepolicy",
		optional:    true,
	},
	{
		remotePath:  "/vendor/etc/selinux/precompiled_sepolicy",
		archivePath: "selinux/vendor/etc/selinux/precompiled_sepolicy",
		optional:    true,
	},
	{
		remotePath:  "/sys/fs/selinux/policy",
		archivePath: "selinux/sys/fs/selinux/policy",
	},
}

func NewSELinux() *SELinux {
	return &SELinux{}
}

func (s *SELinux) Name() string {
	return "selinux"
}

func (s *SELinux) Run(acq *acquisition.Acquisition, opts *Options) error {
	return s.run(acq, adb.Client.Shell, adb.Client.FileExists, acq.SyncPullToZipStaged)
}

func (s *SELinux) run(
	acq *acquisition.Acquisition,
	shell func(...string) (string, error),
	fileExists func(string) (bool, error),
	pull func(string, string) error,
) error {
	log.Info("Collecting SELinux status and policies...")

	var errs []error
	out, err := shell("getenforce")
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to run `adb shell getenforce`: %w", err))
	} else if err := saveStringToAcquisition(acq, "selinux.txt", out); err != nil {
		errs = append(errs, fmt.Errorf("failed to save SELinux status: %w", err))
	}

	for _, policy := range selinuxPolicyFiles {
		if policy.optional {
			exists, err := fileExists(policy.remotePath)
			if err == nil && !exists {
				log.Debugf("SELinux policy not present at %s", policy.remotePath)
				continue
			}
			if err != nil {
				log.Debugf("Unable to check for SELinux policy at %s, attempting collection: %v", policy.remotePath, err)
			}
		}

		log.Debugf("Collecting SELinux policy from %s", policy.remotePath)
		if err := pull(policy.remotePath, policy.archivePath); err != nil {
			log.Warningf("Failed to collect SELinux policy from %s: %v", policy.remotePath, err)
			errs = append(errs, fmt.Errorf("failed to collect SELinux policy from %s: %w", policy.remotePath, err))
		}
	}

	return errors.Join(errs...)
}
