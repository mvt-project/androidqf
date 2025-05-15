RUSTFLAGS="-Clink-arg=-z -Clink-arg=nostart-stop-gc" cargo ndk -t aarch64-linux-android build --release
