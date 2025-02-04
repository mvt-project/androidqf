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
ifneq ($(UPX_COMPRESS),0)
ifneq ($(UPX_COMPRESS),"0")
	upx -$(UPX_COMPRESS) $(BUILD_FOLDER)/collector_arm # UPX_COMPRESS
endif
endif

build_arm64:
	env GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o $(BUILD_FOLDER)/collector_arm64
ifneq ($(UPX_COMPRESS),0)
ifneq ($(UPX_COMPRESS),"0")
	upx -$(UPX_COMPRESS) $(BUILD_FOLDER)/collector_arm64 # UPX_COMPRESS
endif
endif

build_amd64:
	env GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_FOLDER)/collector_amd64
ifneq ($(UPX_COMPRESS),0)
ifneq ($(UPX_COMPRESS),"0")
	upx -$(UPX_COMPRESS) $(BUILD_FOLDER)/collector_amd64 # UPX_COMPRESS
endif
endif

build: build_arm build_arm64 build_amd64

all: build

clean:
	rm -rf $(BUILD_FOLDER)

test: build
	adb shell "rm -rf /data/local/tmp/collector"
	adb push $(BUILD_FOLDER)/collector_arm /data/local/tmp/collector
	adb shell "chmod +x /data/local/tmp/collector"
	adb shell "/data/local/tmp/collector ps"
