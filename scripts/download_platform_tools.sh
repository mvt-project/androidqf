#!/bin/sh

set -eu

download_dir=${ANDROIDQF_PLATFORM_TOOLS_DOWNLOAD_DIR:-/tmp/platform-tools-downloads}
platform=${1:-all}
base_url=https://dl.google.com/android/repository
platform_tools_version=37.0.1

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
        download windows win 45f4d63113e895ebde0c90f194099a4676b6ac653bd28d54314a9e022bbc1a99
        ;;
    darwin)
        download darwin darwin ee39ad5967e95c2a07f04dbcbde96b1a0c916ba376096db5d2f498b7727a5d1d
        ;;
    linux)
        download linux linux d230f13842f60f782a8645f9c813f8f845bf36089ea7289f28c48f17979313f1
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
