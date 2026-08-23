# Acquisition archives

androidqf writes each acquisition directly to a ZIP archive instead of first
building an acquisition directory. This keeps storage use bounded and allows
encryption to protect evidence from the moment it reaches the host.

## Output files

The `-output` option selects the output directory. If it is omitted, androidqf
uses the current working directory. The output filename is based on the
acquisition UUID:

| Configuration | Output |
|---|---|
| No `key.txt` found | `<UUID>.zip` |
| One or more valid age recipients in `key.txt` | `<UUID>.zip.age` |

androidqf looks for `key.txt` in the current working directory and then beside
the executable. A key in the current working directory takes precedence. The
file accepts one age recipient per line; empty lines and lines beginning with
`#` are ignored. Every listed recipient can decrypt the resulting acquisition.

The archive contains the collected module outputs documented in the main
[README](../README.md#how-to-use), plus these acquisition-level entries:

| Entry | Purpose |
|---|---|
| `acquisition.json` | Acquisition UUID, timestamps, androidqf version, device information, and per-module outcomes. |
| `adb_host_key.pub` | Public half of the ADB host key available to androidqf during the acquisition. |
| `command.log` | Debug-level command and acquisition log, when log output was produced. |
| `hashes.csv` | SHA-256 integrity records for preceding plaintext archive entries. |

The ADB public key is also recorded in `acquisition.json`. It can be compared
with ADB authorization records without exposing the private host key. It
identifies the ADB host identity, not the program that initiated a connection:
androidqf, the `adb` command-line client and other tools normally share the
same per-user ADB key.

`hashes.csv` has two CSV fields per row: the ZIP entry name and its lowercase
SHA-256 digest. The digest covers the plaintext, uncompressed entry content.
The file does not contain a hash record for itself.

## Streaming and failed pulls

Module output is written to the archive as it is collected. Streams produced
by external ADB commands, including backups, bug reports, and device-file
transfers, are staged and validated before androidqf creates their ZIP entries.
Consequently, a command that fails after producing some output does not leave
an empty or partial entry that appears valid in `hashes.csv`.

Archive completion writes `acquisition.json`, `command.log` and `hashes.csv`,
then closes the ZIP and, when enabled, the age encryption stream. A failure in
any of these operations is reported as a failed acquisition; androidqf does not
print the acquisition-success message for an archive that could not be
finalized.

`acquisition.json` includes a `module_results` list with the status, start and
completion timestamps, and any error for each selected module. Status is
`completed`, `partial` when some requested evidence could not be collected, or
`failed`. If a module is partial or fails, androidqf still finalizes the archive
so successfully collected evidence remains available, but exits unsuccessfully
and clearly reports that the acquisition contains incomplete modules.

## Encrypted acquisitions

When a valid `key.txt` is present, the ZIP stream is encrypted to every
recipient in the file and passed directly through age into `<UUID>.zip.age`:

```text
device data -> ZIP writer -> age encryption -> <UUID>.zip.age
```

No plaintext ZIP archive or acquisition directory is created. To decrypt an
acquisition:

```bash
age --decrypt -i /path/to/privatekey.txt \
  -o <UUID>.zip <UUID>.zip.age
```

The decrypted ZIP can then be opened with standard ZIP tooling.

### Validated encrypted staging

A ZIP entry cannot be removed after it has been created. For device files that
must be completely pulled or inspected before entry creation, androidqf uses a
temporary encrypted staging file during encrypted acquisitions.

Temporary staging uses MinIO SIO's DARE format with authenticated
ChaCha20-Poly1305 encryption. A new random 256-bit key is generated for each
staged file and retained only in process memory. DARE provides authenticated
random access, allowing APK signature verification without writing a plaintext
APK to disk or loading the entire APK into memory.

```text
ADB pull
   -> ChaCha20-Poly1305 DARE temporary file
   -> authenticated seekable plaintext view in memory
      -> APK certificate verification
      -> final age-encrypted ZIP stream
```

The staging file is deleted and its in-memory key is cleared after use. DARE is
an internal temporary format only; the delivered acquisition remains a
standard age-encrypted ZIP stream.

During an unencrypted acquisition, files requiring validation are staged in an
ordinary temporary file before being committed to the ZIP. A process or host
crash may leave such a plaintext temporary file behind. Use age encryption
when plaintext-at-rest protection is required.

## APK handling

androidqf uses an in-memory buffer of up to 500 MB for APK processing. APKs
within that limit are certificate-checked from memory for both encrypted and
unencrypted acquisitions.

Larger APKs use seekable staging so certificate verification still occurs:

| Acquisition | APK larger than 500 MB |
|---|---|
| Encrypted | Authenticated ChaCha20-Poly1305 DARE staging; no plaintext temporary APK. |
| Unencrypted | Plaintext temporary staging. |

The certificate metadata is recorded in `packages.json` in both cases. If the
operator selects removal of APKs signed by a trusted certificate, that choice
also applies to APKs larger than 500 MB. A trusted APK selected for removal is
not added to the archive.

## Operational considerations

- Keep sufficient free space for the output archive and one additional staged
  copy of the largest device file, backup, or bug report.
- Treat a finalization error as a failed acquisition. The output may be
  incomplete or unreadable and should not be treated as finalized evidence.
- Preserve the age identity separately from the acquisition host when the
  operator should not be able to decrypt collected evidence under duress.
