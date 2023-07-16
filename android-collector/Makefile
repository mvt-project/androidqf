BUILD_FOLDER  = $(shell pwd)/build

.PHONY: build
default: build ;

# Set if binaries should be compressed with UPX. Zero disables UPX
UPX_COMPRESS ?= "0"

check:
	@echo "[lint] Running go vet"
	go vet ./...
	@echo "[lint] Running staticheck on codebase"
	@staticcheck ./...

fmt:
	gofmt -w -s ./

build_arm:
	env GOOS=linux GOARCH=arm GOARM=6 go build -ldflags="-s -w" -o $(BUILD_FOLDER)/collector_arm
	if [[ $(UPX_COMPRESS) -gt 0 ]]; then upx -$(UPX_COMPRESS) $(BUILD_FOLDER)/collector_arm; fi

build_arm64:
	env GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o $(BUILD_FOLDER)/collector_arm64
	if [[ $(UPX_COMPRESS) -gt 0 ]]; then upx -$(UPX_COMPRESS) $(BUILD_FOLDER)/collector_arm64; fi

build_amd64:
	env GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_FOLDER)/collector_amd64
	if [[ $(UPX_COMPRESS) -gt 0 ]]; then upx -$(UPX_COMPRESS) $(BUILD_FOLDER)/collector_amd64; fi

build: build_arm build_arm64 build_amd64

clean:
	rm -rf $(BUILD_FOLDER)

test: build
	adb shell "rm -rf /data/local/tmp/collector"
	adb push $(BUILD_FOLDER)/collector_arm7 /data/local/tmp/collector
	adb shell "chmod +x /data/local/tmp/collector"
	adb shell "/data/local/tmp/collector ps"
