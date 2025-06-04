# Android Rust Collector

## Deps:

export ANDROID_NDK_HOME="/path"
rustup target add aarch64-linux-android
cargo install cargo-ndk

## Compile for aarch64
RUSTFLAGS="-Clink-arg=-z -Clink-arg=nostart-stop-gc" cargo ndk -t aarch64-linux-android build --release
