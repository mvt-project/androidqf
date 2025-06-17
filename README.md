# androidqf

[![Go Report Card](https://goreportcard.com/badge/github.com/mvt-project/androidqf)](https://goreportcard.com/report/github.com/mvt-project/androidqf)

androidqf (Android Quick Forensics) is a portable tool to simplify the acquisition of relevant forensic data from Android devices.

androidqf is intended to provide a simple and portable cross-platform utility to quickly acquire data from Android devices. It is similar in functionality to [mvt-android](https://github.com/mvt-project/mvt). However, contrary to MVT, androidqf is designed to be easily run by non-tech savvy users as well. Data extracted by androidqf can be analyzed with [MVT](https://github.com/mvt-project/mvt).

> This repo is a fork of [androidqf](https://github.com/botherder/androidqf) maintained by [Amnesty International's Security Lab](https://securitylab.amnesty.org/). The androidqf tool was originally developed by [Claudio Guarnieri](https://github.com/botherder/).

[Download androidqf](https://github.com/mvt-project/androidqf/releases/latest)

![](androidqf.png)

## Build

Executable binaries for Linux, Windows and Mac should be available in the [latest release](https://github.com/mvt-project/androidqf/releases/latest). In case you have issues running the binary you might want to build it by yourself.

In order to build androidqf you will need Go 1.15+ installed. You will also need to install `make`. AndroidQF includes a cross-compiled `collector` which runs on the target device to more reliably extract forensically relevant information. Android shell quirkes can make running shell commands to gather information too brittle.

When ready you can clone the repository and first build the `collector` module with:

    make collector

You can then compile AndroidQF for your platform of choice:

    make linux
    make darwin
    make windows

These commands will generate binaries in a _build/_ folder.

## How to use

Before launching androidqf you need to have the target Android device connected to your computer via USB, and you will need to have enabled USB debugging. Please refer to the [official documentation](https://developer.android.com/studio/debug/dev-options#enable) on how to do this, but also be mindful that Android phones from different manufacturers might require different navigation steps than the defaults.

Once USB debugging is enabled, you can proceed launching androidqf. It will first attempt to connect to the device over the USB bridge, which should result in the Android phone to prompt you to manually authorize the host keys. Make sure to authorize them, ideally permanently so that the prompt wouldn't appear again.

Now androidqf should be executing and creating an acquisition folder at the same path you have placed your androidqf binary. At some point in the execution, androidqf will prompt you some choices: these prompts will pause the acquisition until you provide a selection, so pay attention.

The following data can be extracted:

| Data                                                                                               | Optional?          | Output path(s)        |
| -------------------------------------------------------------------------------------------------- | ------------------ | --------------------- |
| A full backup or backup of SMS and MMS messages.                                                   | :white_check_mark: | `backup.ab`           |
| The output of the getprop shell command, providing build information and configuration parameters. |                    | `getprop.txt`         |
| All system settings                                                                                |                    | `settings_*.txt`      |
| The output of the ps shell command or the collector, providing a list of all running processes.    |                    | `processes.txt`       |
| The list of system's services.                                                                     |                    | `services.txt`        |
| A copy of all the logs from the system.                                                            |                    | `logs/`, `logcat.txt` |
| The output of the dumpsys shell command, providing diagnostic information about the device.        |                    | `dumpsys.txt`         |
| A list of all packages installed and related distribution files.                                   |                    | `packages.json`       |
| Copy of all installed APKs or of only those not marked as system apps.                             | :white_check_mark: | `apks/*`              |
| The output of the find shell command or the collector, providing a list of files on the system.    |                    | `files.json`          |
| A copy of the files available in temp folders.                                                     |                    | `tmp/*`               |
| A bug report containing system and app-specific logs, with no private data included.               |                    | `bugreport.zip`       |

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

| Option     | Explanation                                                                                                                                                                                                   |
| ---------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Only SMS   | `adb backup com.android.providers.telephony` is run. Only data from `com.android.providers.telephony` is collected. This includes the SMS database.                                                           |
| Everything | `adb backup -all` is run. This requests backups of only apps that have explicitly allowed backups of their data via this method. Since Android 12+, this method doesn’t extract anything for almost all apps. |
| No backup  | `adb backup` is not run                                                                                                                                                                                       |

### Downloading copies of apps

```
Would you like to download copies of all apps or only non-system ones?

? Download:
  ▸ All
    Only non-system packages
    Do not download any
```

| Option                   | Explanation                                                     |
| ------------------------ | --------------------------------------------------------------- |
| All                      | All installed packages will be retrieved from the phone         |
| Only non-system packages | Don't download any packages listed in `adb pm list packages -s` |
| Do not download any      | Don't download any packages                                     |

## Encryption & Potential Threats

Carrying the androidqf acquisitions on an unencrypted drive might expose yourself, and even more so those you acquired data from, to significant risk. For example, you might be stopped at a problematic border and your androidqf drive could be seized. The raw data might not only expose the purpose of your trip, but it will also likely contain very sensitive data (for example list of applications installed, or even SMS messages).

Ideally you should have the drive fully encrypted, but that might not always be possible. You could also consider placing androidqf inside a [VeraCrypt](https://www.veracrypt.fr/) container and carry with it a copy of VeraCrypt to mount it. However, VeraCrypt containers are typically protected only by a password, which you might be forced to provide.

Alternatively, androidqf allows to encrypt each acquisition with a provided [age](https://age-encryption.org) public key. Preferably, this public key belongs to a keypair for which the end-user does not possess, or at least carry, the private key. In this way, the end-user would not be able to decrypt the acquired data even under duress.

If you place a file called `key.txt` in the same folder as the androidqf executable, androidqf will automatically attempt to compress and encrypt each acquisition and delete the original unencrypted copies.

Once you have retrieved an encrypted acquisition file, you can decrypt it with age like so:

```
$ age --decrypt -i ~/path/to/privatekey.txt -o <UUID>.zip <UUID>.zip.age
```

Bear in mind, it is always possible that at least some portion of the unencrypted data could be recovered through advanced forensics techniques - although we're working to mitigate that.

## License

The purpose of androidqf is to facilitate the **_consensual forensic analysis_** of devices of those who might be targets of sophisticated mobile spyware attacks, especially members of civil society and marginalized communities. We do not want androidqf to enable privacy violations of non-consenting individuals. Therefore, the goal of this license is to prohibit the use of androidqf (and any other software licensed the same) for the purpose of _adversarial forensics_.

In order to achieve this androidqf is released under [MVT License 1.1](https://license.mvt.re/1.1/), an adaptation of [Mozilla Public License v2.0](https://www.mozilla.org/MPL). This modified license includes a new clause 3.0, "Consensual Use Restriction" which permits the use of the licensed software (and any _"Larger Work"_ derived from it) exclusively with the explicit consent of the person/s whose data is being extracted and/or analysed (_"Data Owner"_).
