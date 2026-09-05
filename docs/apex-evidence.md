# APEX evidence

The `apex` module collects evidence for offline analysis. It does not verify
signatures, maintain a test-key catalogue, or determine whether a device is
vulnerable or compromised. Those assessments belong in MVT.

## Collected data

| Archive entry | Contents |
| --- | --- |
| `apex/apex-info-list.xml` | Original bytes returned for `/apex/apex-info-list.xml`, including attributes AndroidQF does not interpret. Also retained when parsing fails or the read returns partial data. |
| `apex/manifest.json` | Schema version 1, discovery source, module metadata, file provenance and collection errors. |
| `apex/packages.txt` | Output of `pm list packages --apex-only -f`, when fallback discovery is needed. |
| `apex/files/<device-path-without-leading-slash>` | Complete, unmodified `.apex` and `.capex` containers. |

For example, `/system/apex/com.example.apex` is saved as
`apex/files/system/apex/com.example.apex`. Identical device paths are collected
once; files with the same name on different partitions remain separate.

Complete containers retain the APK signing material, `apex_pubkey`, payload and
its AVB metadata. Compressed APEX files retain their embedded `original_apex`.
Keeping the original bytes lets an offline analyzer extract keys and certificates
or verify signatures without trusting an AndroidQF-generated verdict. Containers
are not unpacked, executed or installed during acquisition.

## Discovery and access

The module uses the XML inventory to collect both `modulePath` and
`preinstalledModulePath`. Module names, version codes, version names and available
factory/active flags are also recorded in the JSON manifest. Missing boolean
attributes are omitted rather than inferred. All inventory records are preserved,
including inactive factory versions; collection is not restricted to active
modules.

If the inventory cannot be read or parsed, AndroidQF queries the package manager
and lists the immediate entries of `/system/apex`, `/system_ext/apex`,
`/product/apex`, `/vendor/apex`, `/odm/apex` and `/oem/apex`. This fallback may
recover containers but cannot recover the complete XML metadata, so the module
reports a partial collection. It does not recursively copy mounted payloads under
`/apex` or enumerate historical/staged updates under `/data/apex`.

Normal ADB access is used first. Only when a path is inaccessible does AndroidQF
check whether `su` can already execute as UID 0. If so, it uses that access to read
the container. It never enables or installs root. Missing or unreadable files are
recorded individually, and collection continues with other files. Devices reporting
an Android API level below 29 are recorded as `not_supported` and skipped.

Directory entries, such as flattened APEX modules, are recorded with status
`directory`; their contents are not copied. This records the absence of a collected
signed container and must not be interpreted as a successful signature check.

## Manifest and storage

The manifest's overall status is `completed`, `partial` or `not_supported`.
`completed` describes collection only. Each file record contains `device_path`,
`status` (`collected`, `directory` or `failed`), and, where applicable,
`archive_path`, `acquisition_method` (`adb` or `su`) and `error`. The separate
`modules` list associates inventory metadata with the original device paths.

APEX collection runs by default and with `-module apex`. The `-download`,
`-remove-trusted` and `-fast` options do not suppress APEX collection. Full
containers can substantially increase transfer time and archive size.

Each transfer is staged on the host before an archive entry is created, so failed
transfers do not leave truncated container entries. Encrypted acquisitions use
encrypted temporary storage. Temporary storage is cleaned up after each transfer,
and retained archive entries use the standard `hashes.csv` integrity mechanism.

See the [Android APEX format documentation](https://source.android.com/docs/core/ota/apex)
for the container formats and signing layers.
