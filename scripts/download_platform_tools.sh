#!/bin/sh

set -eu

download_dir=${ANDROIDQF_PLATFORM_TOOLS_DOWNLOAD_DIR:-/tmp/platform-tools-downloads}
platform=${1:-all}
base_url=https://dl.google.com/android/repository

download() {
    platform_name=$1
    expected_hash=$2
    archive="$download_dir/platform-tools-latest-$platform_name.zip"

    umask 077
    mkdir -p "$download_dir"

    if [ -f "$archive" ] && printf '%s  %s\n' "$expected_hash" "$archive" | sha256sum -c - >/dev/null 2>&1; then
        printf '%s\n' "$archive"
        return
    fi

    temporary=$(mktemp "$archive.XXXXXX")
    trap 'rm -f "$temporary"' EXIT HUP INT TERM
    curl --fail --location --silent --show-error \
        "$base_url/platform-tools-latest-$platform_name.zip" \
        --output "$temporary"
    printf '%s  %s\n' "$expected_hash" "$temporary" | sha256sum -c -
    mv "$temporary" "$archive"
    trap - EXIT HUP INT TERM
    printf '%s\n' "$archive"
}

case "$platform" in
    windows)
        download windows 4fe305812db074cea32903a489d061eb4454cbc90a49e8fea677f4b7af764918
        ;;
    darwin)
        download darwin 094a1395683c509fd4d48667da0d8b5ef4d42b2abfcd29f2e8149e2f989357c7
        ;;
    linux)
        download linux 198ae156ab285fa555987219af237b31102fefe8b9d2bc274708a8d4f2865a07
        ;;
    all)
        "$0" windows
        "$0" darwin
        "$0" linux
        ;;
    *)
        echo "usage: $0 [windows|darwin|linux|all]" >&2
        exit 2
        ;;
esac
