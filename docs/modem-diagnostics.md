# Existing modem diagnostic logs

The optional `modem_diagnostics` module addresses
[issue #142](https://github.com/mvt-project/androidqf/issues/142) by preserving
**existing files only**, not starting a live radio capture. AndroidQF does not
enable modem logging, change USB configuration or SELinux policy, invoke service
menus, install applications, restart adbd, or attempt to root the device.

Use `-modem-diagnostics yes` or `-modem-diagnostics no`. With neither flag,
interactive acquisitions ask, defaulting to **No**. `-non-interactive` requires
an explicit answer if this module is selected (including a full acquisition).
`-fast` does not override the answer. For example:

```sh
androidqf -serial SERIAL -module modem_diagnostics -modem-diagnostics yes -non-interactive
```

## Phones and root requirements

There is no reliable manufacturer-wide root/no-root rule. Access to a **live
diagnostic interface**, the ability to **generate/export logs**, and permission
to **read existing files** are different capabilities. AndroidQF implements only
the last one. Model, regional firmware, carrier restrictions, Android version,
SELinux policy and whether logs were previously generated all matter.

| Device/modem family | Existing evidence this module can collect | Root implications |
| --- | --- | --- |
| Qualcomm-based Android phones, including some Xiaomi, Samsung, LG, Sony and Nexus models | QMDL, QMDL2 and DLF files in the searched locations | Shared-storage exports can be read without phone root if accessible to ADB. Protected vendor files need existing privileged access. Qualcomm hardware alone does not guarantee saved logs. |
| Samsung phones with Exynos/Shannon modems | SDM files, including compressed copies | Already-exported files may be rootless; private files depend on permissions. SilentLogger availability/authentication is firmware-dependent. |
| Tensor Pixels using Samsung modems | Existing SDM files in vendor log directories or already-exported copies | Stock/private access may be denied; AndroidQF probes permissions and uses existing root when necessary. A readable export does not require root. |
| Huawei/HiSilicon phones | Existing LPD exports in the searched locations | Accessible exports do not require root. This is not general support for every private Huawei log directory. |
| MediaTek, Unisoc and other formats | No dedicated format/location support | Root alone does not make their proprietary logs supported by this module. |

SCAT documents rootless offline QMDL generation on **some older Xiaomi
firmware**, not a promise for current devices. It describes Samsung SDM logging
with OTP restrictions, and HiSilicon CP logging through a vendor menu or a
root-only command. AndroidQF invokes none of these mechanisms. See
[SCAT's baseband dump notes](https://github.com/fgsect/scat/wiki/Baseband-Dumps).

For comparison, the upstream **live-interface** test matrix reports:

| Upstream tested examples | Phone root for that upstream interface |
| --- | --- |
| LG G Flex 2 (LG-H955); Samsung Galaxy S4 LTE (GT-I9505), S23 (SM-S911B), S6 (SM-G920F), S8 (SM-G950F), S21 (SM-G991N), S22 (SM-S901B) | Not required on the tested firmware |
| Nexus 5/5X; Sony Xperia X; Xiaomi POCO F1, Mi Mix 3 5G, Mi 10T 5G | Required in the reported setups |
| Pixel 2 | Root plus system-partition changes in the reported setup |
| Pixel 6, 7 and 9 | Required in the reported setups |

These examples are from [SCAT's device matrix](https://github.com/fgsect/scat/wiki/Devices),
not hardware validation by AndroidQF and not an allowlist. The same matrix
warns that some Samsung firmware blocks diagnostic menus. Do not infer that
every Samsung is rootless or that a device with root must have useful log files.

[QCSuper's documentation](https://github.com/P1sec/QCSuper#how-to-manually-enable-the-diagnostic-ports-on-my-phone)
also distinguishes rooted ADB access from an exposed Qualcomm USB DIAG port.
It notes that newer Qualcomm kernels often lack `/dev/diag` and may instead
require USB mode switching. AndroidQF neither reads that character device nor
switches USB modes; QCSuper/SCAT are not runtime dependencies.

## Searched locations and formats

The module checks these fixed roots on every opted-in device, rather than
guessing access from its manufacturer or application processor:

| Directory | Purpose/source |
| --- | --- |
| `/sdcard/diag_logs` | Qualcomm exported logs; the path is used by [Qualcomm's diag_mdlog implementation](https://github.com/bcyj/android_tools_leeco_msm8996/blob/master/diag/mdlog/diag_mdlog.c). |
| `/sdcard/log` | Vendor exports; Samsung documents shared-storage log exports in its [SysDump guide](https://docs.samsungknox.com/admin/fundamentals/kbas/kba-115013353688-mobile-device-logs/). An ordinary dumpstate `.log` is not an SDM trace and is not selected. |
| `/sdcard/Download` | Recognized modem logs previously placed in the standard download folder. This is an export discovery location, not a claim that every vendor saves logs there. |
| `/sdcard/Android/data/com.android.pixellogger/files/logs` | Existing PixelLogger exports, including its `diag_logs` subdirectory, documented in [AOSP's PixelLogger test utilities](https://android.googlesource.com/platform/tools/test/connectivity/+/bba16076ee9814d2f142536bf5ec04f230835b3b%5E2..bba16076ee9814d2f142536bf5ec04f230835b3b/). Android's storage policy may restrict ADB access. |
| `/data/diag_logs` | Legacy Qualcomm logs; see [Nexus 5's diagnostic configuration](https://android.googlesource.com/device/lge/hammerhead/+/4227127ec66719397100d9700283d40e93560881/init.hammerhead.diag.rc.userdebug). Debug-build configuration does not imply stock-build access. |
| `/data/vendor/radio/diag_logs` | Qualcomm diagnostic logs, as used by [AOSP ACTS](https://android.googlesource.com/platform/tools/test/connectivity/+/refs/heads/main/acts/framework/acts/controllers/android_device.py). |
| `/data/vendor/radio/logs` and `/data/vendor/slog` | Pixel/Shannon logs, including `logs/always-on`; see [Google's GS101 init configuration](https://android.googlesource.com/device/google/gs101/+/refs/heads/main/conf/init.gs101.rc) and AOSP ACTS above. |
| `/data/log` | Private vendor logs, also searched by AndroidQF's existing `logs` module; only recognized modem filenames are selected here. |

Only regular files ending in `.qmdl`, `.qmdl2`, `.dlf`, `.sdm` or `.lpd` are
selected, case-insensitively. One additional `.gz`, `.zip`, `.lz4` or `.zst`
suffix is accepted; the compressed bytes are copied unchanged, not unpacked.
For example, `trace.sdm.gz` is selected but `report.zip`, `dumpstate.log` and
`trace.qdss` are not. A filename is only a discovery hint, not format validation.
SCAT's [format notes](https://github.com/fgsect/scat/wiki/Baseband-Dumps) caution
that QMDL2 parsing varies, QDSS is unsupported, and HiSilicon decoding is
experimental. Preserving a file does not guarantee that it can be decoded.

Only these directories are searched recursively, not all of shared storage or
the filesystem. Symlinks at the fixed roots are followed (notably `/sdcard`),
but symlinks within them and special files are excluded. Secondary users,
removable storage and arbitrary vendor paths are not enumerated. Logs with
unrecognized names, generic archive wrappers, or different storage locations
will not be collected.

## Access, output and limitations

Ordinary ADB is attempted first, including when adbd already runs as root.
Only an inaccessible path or incomplete listing triggers a check for an
already-functional `su` running as UID 0. That check is cached for the module
and limited to ten seconds. For unattended use, any root-manager authorization
must already be granted; AndroidQF cannot approve it. SELinux can still deny
access even with UID 0. No files are copied through shared storage to bypass
permissions.

Raw files are saved to
`modem_diagnostics/files/<device-path-without-leading-slash>`. Transfers use the
standard host-side staging, encrypted temporary storage for encrypted
acquisitions, and `hashes.csv` integrity mechanism. Failed transfers are not
committed as truncated archive entries. Distinct original paths stay distinct;
repeated discoveries of the same path are collected once.

`modem_diagnostics/manifest.json` contains schema version 1, collection time,
selected device properties, `su_access` (`not_checked`, `available`, or
`unavailable`), every searched source, file provenance/access method (`adb` or
`su`), and errors. Source states distinguish `listed`, `not_found`,
`inaccessible`, `not_directory`, and `partial`. File states are `collected` or
`failed`; only successful files have an `archive_path`. The module-level states
are:

- `collected`: at least one file copied, without detected discovery/transfer
  failures. This does not establish complete modem history.
- `no_logs`: no recognized files found in the accessible catalogue.
- `access_limited`: one or more locations inaccessible with available
  privileges; readable files may still have been collected. This expected
  device limitation is recorded and warned about, not treated as a fatal
  acquisition error.
- `partial`: a command, listing, path validation or file transfer failed. The
  module returns a partial-collection error while preserving other results.
- `interrupted`: cancellation; the manifest and completed files are retained.

No logs collected does **not** establish that logging never occurred, that the
phone is unsupported, or that it is uncompromised. Logs must already exist;
stock phones often have none. This is not an atomic snapshot: an existing
logger may rotate or modify files during collection. The module runs before
bugreport generation to preserve files before vendor bugreport hooks run.

These files may contain subscriber identifiers, location, signaling and
communications data. Collection can substantially increase archive size and
duration. Opting out prevents this module's probes and reads, but does not
filter overlapping evidence already collected by the separate `logs`,
`logcat`, or `bugreport` modules.
