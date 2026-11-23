#!/bin/sh

set -eu

download_dir=${ANDROIDQF_PLATFORM_TOOLS_DOWNLOAD_DIR:-/tmp/platform-tools-downloads}
platform=${1:-all}
base_url=https://dl.google.com/android/repository
platform_tools_version=37.0.3

download() {
    platform_name=$1
    archive_platform_name=$2
    expected_hash=$3
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
        "$base_url/platform-tools_r$platform_tools_version-$archive_platform_name.zip" \
        --output "$temporary"
    printf '%s  %s\n' "$expected_hash" "$temporary" | sha256sum -c -
    mv "$temporary" "$archive"
    trap - EXIT HUP INT TERM
    printf '%s\n' "$archive"
}

case "$platform" in
    windows)
        download windows win 89d108360a3bf2924d3bea837239edaccc22fd6ff5befd3787c4348a9fe4f09d
        ;;
    darwin)
        download darwin darwin df21339b9371fef9b19264404338dc686f59a88a1dce634b963da344909001d6
        ;;
    linux)
        download linux linux 214548146cadb2ca3508143d8a1d4be4c59de5bb837253e3ede89ed74cf450d1
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
