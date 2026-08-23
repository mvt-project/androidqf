# androidqf

[![Go Report Card](https://goreportcard.com/badge/github.com/mvt-project/androidqf)](https://goreportcard.com/report/github.com/mvt-project/androidqf)

androidqf (Android Quick Forensics) is a portable tool to simplify the acquisition of relevant forensic data from Android devices.

androidqf is intended to provide a simple and portable cross-platform utility to quickly acquire data from Android devices. It is similar in functionality to [mvt-android](https://github.com/mvt-project/mvt). However, contrary to MVT, androidqf is designed to be easily run by non-tech savvy users as well. Data extracted by androidqf can be analyzed with [MVT](https://github.com/mvt-project/mvt).

> This repo is a fork of [androidqf](https://github.com/botherder/androidqf) maintained by [Amnesty International's Security Lab](https://securitylab.amnesty.org/). The androidqf tool was originally developed by [Claudio Guarnieri](https://github.com/botherder/).

[Download androidqf](https://github.com/mvt-project/androidqf/releases/latest)

![](androidqf.png)

## Build

Executable binaries for Linux, Windows and Mac should be available in the [latest release](https://github.com/mvt-project/androidqf/releases/latest). In case you have issues running the binary you might want to build it by yourself.

### Building with GoReleaser (Recommended)

This project uses [GoReleaser](https://goreleaser.com/) for automated builds and releases. To build locally:

1. Install GoReleaser:
   ```bash
   go install github.com/goreleaser/goreleaser@v2.17.1
   ```

2. Run a snapshot build (no publishing):
   ```bash
   ./build_locally.sh
   ```

This will create binaries for all platforms in the `dist/` directory, including a universal binary for macOS that works on both Intel and Apple Silicon.

### Building with Make (Legacy)

You can still use the traditional Makefile approach. You will need Go 1.26.5+ installed, along with `make`, `git`, `unzip` and `curl`. AndroidQF includes a cross-compiled `collector` which runs on the target device to more reliably extract forensically relevant information.

First build the `collector` module:

    make collector

Then compile AndroidQF for your platform of choice:

    make linux
    make darwin
    make windows

These commands will generate binaries in a *build/* folder.

### Building for distribution packages without bundled assets

Distribution packages can opt out of embedding the bundled ADB and collector
binaries by building with the `unbundle` build tag:

```bash
go build -tags unbundle -o build/
```

When this tag is enabled, androidqf expects:

- `adb` to be available from the system `PATH`.
- collector binaries to be installed under
  `/usr/lib/androidqf/android-collector/` using the names expected by
  androidqf, such as `collector_arm` and `collector_arm64`.

Packagers may remove the bundled binary assets from `assets/` before building,
but the `assets/` package directory and its Go source files must remain present.
The `unbundle` build still imports the `assets` package, and the build will fail
if the whole `assets/` directory is deleted.

## Container image

The release container image is published to GitHub Container Registry:

```bash
docker pull ghcr.io/mvt-project/androidqf:latest
```

To collect from a USB-connected Android device on Linux, pass through the USB
bus and mount an output directory:

```bash
docker run --rm -it --privileged \
  -v /dev/bus/usb:/dev/bus/usb \
  -v "$(pwd)/output:/output" \
  ghcr.io/mvt-project/androidqf:latest -fast -output /output
```

You can also build the image locally for a released version:

```bash
docker build --build-arg VERSION=1.8.3 -t androidqf .
```

## How to use

> [!TIP]
> See [Acquisition archives](docs/acquisition-archives.md) for the archive format, integrity hashes, encryption and large-file handling. For a dictionary of collected files, see the third-party [SocialTIC AndroidQF output file dictionary](https://forensics.socialtic.org/en/references/01-reference-androidqf-dictionary/01-reference-androidqf-dictionary.html).

Before launching androidqf you need to have the target Android device connected to your computer via USB, and you will need to have enabled USB debugging. Please refer to the [official documentation](https://developer.android.com/studio/debug/dev-options#enable) on how to do this, but also be mindful that Android phones from different manufacturers might require different navigation steps than the defaults.

Once USB debugging is enabled, you can proceed launching androidqf. It will first attempt to connect to the device over the USB bridge, which should result in the Android phone to prompt you to manually authorize the host keys. Make sure to authorize them, ideally permanently so that the prompt wouldn't appear again.

Now androidqf should be executing and creating an acquisition zip archive in your current working directory, or in the directory provided with `-output`. At some point in the execution, androidqf will prompt you some choices: these prompts will pause the acquisition until you provide a selection, so pay attention. Every prompt can also be answered ahead of time with a command-line flag, see [Unattended acquisitions](#unattended-acquisitions).

The following data can be extracted:

| Data | Optional? | Output path(s) |
|------|-----------|----------------|
| A full backup or backup of SMS and MMS messages. | :white_check_mark: | `backup.ab` |
| The output of the getprop shell command, providing build information and configuration parameters. | |  `getprop.txt` |
| All system settings | | `settings_*.txt` |
| The output of the ps shell command, providing a list of all running processes. | | `processes.json` when the collector is available, otherwise `processes.txt` |
| The list of system's services. | | `services.txt` |
| A copy of all the logs from the system. | | `logs/`, `logcat.txt` |
| The output of the dumpsys shell command, providing diagnostic information about the device. | | `dumpsys.txt` |
| A list of all packages installed and related distribution files. | |  `packages.json` |
| Copy of all installed APKs or of only those not marked as system apps. | ✅ | `apks/*` |
| Intrusion Logging logs. Contains private data such as navigation history. | ✅ | `intrusion_logs/*` |
| Installed Magisk module metadata and state markers, when existing root access is available. | ✅ | `magisk_modules/*` |
| A list of files on the system, optionally including on-device hashes. | :white_check_mark: | `files.json` |
| A copy of the files available in temp folders. | | `tmp/*` |
| A bug report containing system and app-specific logs, with no private data included. | | `bugreport.zip` |

Every acquisition also contains `acquisition.json`, `command.log` when log output
was produced, and `hashes.csv`. The hash list records the SHA-256 digest of each
preceding plaintext archive entry and does not include itself. Failed device
transfers are not committed as archive entries. `acquisition.json` records the
status (`completed`, `partial`, or `failed`), timing, and error (if any) for
every module that ran. A finalized
partial acquisition exits unsuccessfully instead of printing the normal
completion message. See [Acquisition
archives](docs/acquisition-archives.md) for details.

### About optional data collection

#### Backup

The following options are presented when running an androidqf collection:

```
Would you like to take a backup of the device?
...
? Backup:
  ▸ Only SMS
    Everything
    No backup
```

These options refers to data collected from the device by running the `adb backup` command in the background. If `No backup` is selected, the `adb backup` command is not run.

| Option | Explanation |
|--------|-------------|
| Only SMS | `adb backup com.android.providers.telephony` is run. Only data from `com.android.providers.telephony` is collected. This includes the SMS database. |
| Everything | `adb backup -all` is run. This requests backups of only apps that have explicitly allowed backups of their data via this method. Since Android 12+, this method doesn’t extract anything for almost all apps.|
| No backup | `adb backup` is not run |

### Downloading copies of apps

```
Would you like to download copies of all apps or only non-system ones?

? Download:
  ▸ All
    Only non-system packages
    Do not download any
```

| Option | Explanation |
|--------|-------------|
| All | All installed packages will be retrieved from the phone |
| Only non-system packages | Don't download any packages listed in `adb pm list packages -s` |
| Do not download any | Don't download any packages |

### Removing apps signed with a trusted certificate

Unless you chose `Do not download any`, androidqf asks whether downloaded APKs signed with a trusted certificate (for example by Google) should be omitted to limit the size of the output archive:

```
Would you like to remove copies of apps signed with a trusted certificate to limit the size of the output archive?

? Remove:
  ▸ Yes
    No
```

| Option | Explanation |
|--------|-------------|
| Yes | Downloaded APKs with a trusted signing certificate are omitted from the output archive |
| No | All downloaded APKs are kept |

### Intrusion Logs

```
Would you like to take the Intrusion Logs of the device?

? Intrusion Logs:
  ▸ Yes
    No
```

| Option | Explanation |
|--------|-------------|
| Yes | Intrusion Logs will be retrieved from the phone. |
| No | Intrusion Logs acquisition is skipped. |

### Hashing files on the device

```
Would you like to hash files on the device? This is resource-intensive and may cause the collector to stop on some devices.

? Hash files:
  ▸ No
    Yes
```

Selecting `Yes` adds MD5, SHA-1, SHA-256 and SHA-512 hashes to the entries in
`files.json` where hashing is supported. This performs the hashing on the
device and can take a long time or cause the collector to stop on devices with
limited resources. The default `No` option only collects file metadata.

### Browser history

AndroidQF can optionally collect Chromium `History` databases from supported
browsers when the device already provides working root access through `su`.
AndroidQF does not root the device, stop browser processes, or copy the files
through shared storage. The default `No` option skips this collection.

When enabled, AndroidQF streams each database and any present `-wal` and `-shm`
sidecars directly to host-side staging before adding them under
`browser_history/` in the acquisition. A `browser_history/manifest.json` file
records the browser, package, profile, original device path, and archive path.
The currently supported packages are Chrome, Brave, Microsoft Edge, and
Samsung Internet.

### Magisk modules

AndroidQF can optionally collect metadata for modules installed under
`/data/adb/modules` when the device already provides working root access
through `su`. AndroidQF does not attempt to root the device. The default `No`
option skips this collection.

When enabled, AndroidQF records each module directory and the presence of the
Magisk `disable`, `remove`, and `update` state files. It also streams each
available `module.prop` directly to host-side staging before adding it under
`magisk_modules/` in the acquisition. The accompanying manifest records
whether property and state collection completed, so an unavailable artifact is
not silently treated as an enabled module.

### Unattended acquisitions

Every prompt can be answered ahead of time with a command-line flag. A flag that is not passed keeps prompting interactively as before.

| Flag | Values | Prompt it answers |
|------|--------|-------------------|
| `-backup` / `-b` | `sms`, `all`, `none` | [Backup](#backup) |
| `-download` / `-d` | `all`, `non-system`, `none` | [Downloading copies of apps](#downloading-copies-of-apps) |
| `-remove-trusted` / `-r` | `yes`, `no` | [Removing apps signed with a trusted certificate](#removing-apps-signed-with-a-trusted-certificate), ignored with `-download none` |
| `-intrusion-logs` / `-i` | `yes`, `no` | [Intrusion Logs](#intrusion-logs) |
| `-hash-files` / `-H` | `yes`, `no` | [Hashing files on the device](#hashing-files-on-the-device) |
| `-browser-history` | `yes`, `no` | [Browser history](#browser-history) |
| `-magisk-modules` | `yes`, `no` | [Magisk modules](#magisk-modules) |

With `-non-interactive` (`-n`), androidqf never prompts: it fails before the acquisition starts if one of the flags above is missing, fails if multiple devices are attached and no `-serial` is given, and skips the final "Press Enter to finish". A fully unattended run looks like this:

```bash
androidqf -serial <serial> -backup none -download all -remove-trusted no -intrusion-logs no -hash-files no -browser-history no -magisk-modules no -non-interactive
```

> [!NOTE]
> `adb backup` requires manually authorizing the backup on the device, which androidqf cannot bypass. With `-backup sms` or `-backup all` someone still needs to confirm the backup on the phone; only `-backup none` makes the backup step fully unattended.
>
> Downloading new Intrusion Logs also requires interacting with the device: with `-intrusion-logs yes`, someone still needs to tap "Access Logs" and "Download and Decrypt" on the phone. Use `-intrusion-logs no` for a fully unattended run.

## Encryption & Potential Threats

Carrying the androidqf acquisitions on an unencrypted drive might expose yourself, and even more so those you acquired data from, to significant risk. For example, you might be stopped at a problematic border and your androidqf drive could be seized. The raw data might not only expose the purpose of your trip, but it will also likely contain very sensitive data (for example list of applications installed, or even SMS messages).

Ideally you should have the drive fully encrypted, but that might not always be possible. You could also consider placing androidqf inside a [VeraCrypt](https://www.veracrypt.fr/) container and carry with it a copy of VeraCrypt to mount it. However, VeraCrypt containers are typically protected only by a password, which you might be forced to provide.

Alternatively, androidqf allows to encrypt each acquisition with a provided [age](https://age-encryption.org) public key. Preferably, this public key belongs to a keypair for which the end-user does not possess, or at least carry, the private key. In this way, the end-user would not be able to decrypt the acquired data even under duress.

androidqf streams each acquisition into a zip archive. If you place a file called `key.txt` in the current working directory, androidqf will encrypt the zip stream with age and write `<UUID>.zip.age`; otherwise, it writes an unencrypted `<UUID>.zip`. Put one age recipient per line in `key.txt`; each recipient can decrypt the resulting acquisition. Empty lines and lines beginning with `#` are ignored. androidqf also checks for `key.txt` in the same folder as the executable; if both files exist, the current working directory takes precedence.

Encrypted acquisitions do not create a plaintext acquisition archive. Device
files that must be validated before they are added to an encrypted archive are
temporarily staged with authenticated ChaCha20-Poly1305 encryption. See
[Acquisition archives](docs/acquisition-archives.md#encrypted-acquisitions) for
the complete data flow and large-APK behavior.

Once you have retrieved an encrypted acquisition file, you can decrypt it with age like so:

```
$ age --decrypt -i ~/path/to/privatekey.txt -o <UUID>.zip <UUID>.zip.age
```

Bear in mind, it is always possible that at least some portion of the unencrypted data could be recovered through advanced forensics techniques - although we're working to mitigate that.

## License

The purpose of androidqf is to facilitate the ***consensual forensic analysis*** of devices of those who might be targets of sophisticated mobile spyware attacks, especially members of civil society and marginalized communities. We do not want androidqf to enable privacy violations of non-consenting individuals. Therefore, the goal of this license is to prohibit the use of androidqf (and any other software licensed the same) for the purpose of *adversarial forensics*.

In order to achieve this androidqf is released under [MVT License 1.1](https://license.mvt.re/1.1/), an adaptation of [Mozilla Public License v2.0](https://www.mozilla.org/MPL). This modified license includes a new clause 3.0, "Consensual Use Restriction" which permits the use of the licensed software (and any *"Larger Work"* derived from it) exclusively with the explicit consent of the person/s whose data is being extracted and/or analysed (*"Data Owner"*).
