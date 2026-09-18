BUILD_FOLDER  = "$(shell pwd)/build"
DIST_FOLDER = "$(shell pwd)/dist"
ASSETS_FOLDER = "$(shell pwd)/assets"

.PHONY: all
default: all ;

VERSION := $(shell git describe --always --long --dirty)
PACKAGE_PATH = github.com/mvt-project/androidqf

FLAGS_LINUX   = GOOS=linux
FLAGS_DARWIN  = GOOS=darwin
FLAGS_WINDOWS = GOOS=windows GOARCH=amd64 CC=i686-w64-mingw32-gcc CGO_ENABLED=1
LD_FLAGS = -s -w -X ${PACKAGE_PATH}/utils.Version=${VERSION}

# Set if binaries should be compressed with UPX. Zero disables UPX
UPX_COMPRESS ?= "0"

PLATFORMTOOLS_WINDOWS = platform-tools-latest-windows.zip
PLATFORMTOOLS_DARWIN  = platform-tools-latest-darwin.zip
PLATFORMTOOLS_LINUX   = platform-tools-latest-linux.zip
PLATFORMTOOLS_FOLDER  = /tmp/platform-tools
PLATFORMTOOLS_DOWNLOAD_FOLDER = /tmp/platform-tools-downloads

check:
	@echo "[lint] Running go vet"
	go vet ./...
	cd android-collector && go vet ./...
	@echo "[lint] Running staticcheck on codebase"
	@staticcheck ./...
	cd android-collector && staticcheck ./...

vuln:
	@echo "Running go vuln check"
	@govulncheck ./...
	cd android-collector && govulncheck ./...

test:
	go test -race ./...
	cd android-collector && go test -race ./...

fmt:
	gofumpt -l -w .

deps:
	@echo "[deps] Installing dependencies..."
	go mod download
	go mod tidy
	@echo "[deps] Dependencies installed."

collector:
	@mkdir -p $(BUILD_FOLDER)
	@echo "Building Android collector..."
	cd android-collector && UPX_COMPRESS=$(UPX_COMPRESS) $(MAKE)
	@echo "Finished building collector."
	@echo "Copying collector binaries to assets folder."
	cp android-collector/build/collector_* $(ASSETS_FOLDER)

windows:
	@mkdir -p $(BUILD_FOLDER)

	@./scripts/download_platform_tools.sh windows

	@rm -rf $(PLATFORMTOOLS_FOLDER)
	@cd /tmp && unzip -u $(PLATFORMTOOLS_DOWNLOAD_FOLDER)/$(PLATFORMTOOLS_WINDOWS)
	@cp $(PLATFORMTOOLS_FOLDER)/AdbWinApi.dll $(ASSETS_FOLDER)
	@cp $(PLATFORMTOOLS_FOLDER)/AdbWinUsbApi.dll $(ASSETS_FOLDER)
	@cp $(PLATFORMTOOLS_FOLDER)/adb.exe $(ASSETS_FOLDER)

	@echo "[builder] Building Windows binary for amd64"

	$(FLAGS_WINDOWS) go build --ldflags '$(LD_FLAGS) -extldflags "-static"' -o $(BUILD_FOLDER)/androidqf_windows_amd64.exe .

	@echo "[builder] Done!"

darwin:
	@mkdir -p $(BUILD_FOLDER)

	@./scripts/download_platform_tools.sh darwin

	@rm -rf $(PLATFORMTOOLS_FOLDER)
	@cd /tmp && unzip -u $(PLATFORMTOOLS_DOWNLOAD_FOLDER)/$(PLATFORMTOOLS_DARWIN)
	@cp $(PLATFORMTOOLS_FOLDER)/adb $(ASSETS_FOLDER)/adb_darwin

	@echo "[builder] Building Darwin binary for amd64"

	$(FLAGS_DARWIN) GOARCH=amd64 go build --ldflags '$(LD_FLAGS)' -o $(BUILD_FOLDER)/androidqf_darwin_amd64 .
	$(FLAGS_DARWIN) GOARCH=arm64 go build --ldflags '$(LD_FLAGS)' -o $(BUILD_FOLDER)/androidqf_darwin_arm64 .

	@echo "[builder] Done!"

linux:
	@mkdir -p $(BUILD_FOLDER)

	@./scripts/download_platform_tools.sh linux

	@rm -rf $(PLATFORMTOOLS_FOLDER)
	@cd /tmp && unzip -u $(PLATFORMTOOLS_DOWNLOAD_FOLDER)/$(PLATFORMTOOLS_LINUX)
	@cp $(PLATFORMTOOLS_FOLDER)/adb $(ASSETS_FOLDER)/adb_linux

	@echo "[builder] Building Linux binary for amd64"

	@$(FLAGS_LINUX) GOARCH=amd64 go build --ldflags '$(LD_FLAGS)' -o $(BUILD_FOLDER)/androidqf_linux_amd64 .
	@$(FLAGS_LINUX) GOARCH=arm64 go build --ldflags '$(LD_FLAGS)' -o $(BUILD_FOLDER)/androidqf_linux_arm64 .

	@echo "[builder] Done!"

download:
	@./scripts/download_platform_tools.sh windows

	@rm -rf $(PLATFORMTOOLS_FOLDER)
	@cd /tmp && unzip -u $(PLATFORMTOOLS_DOWNLOAD_FOLDER)/$(PLATFORMTOOLS_WINDOWS)
	@cp $(PLATFORMTOOLS_FOLDER)/AdbWinApi.dll $(ASSETS_FOLDER)
	@cp $(PLATFORMTOOLS_FOLDER)/AdbWinUsbApi.dll $(ASSETS_FOLDER)
	@cp $(PLATFORMTOOLS_FOLDER)/adb.exe $(ASSETS_FOLDER)

	@./scripts/download_platform_tools.sh darwin

	@rm -rf $(PLATFORMTOOLS_FOLDER)
	@cd /tmp && unzip -u $(PLATFORMTOOLS_DOWNLOAD_FOLDER)/$(PLATFORMTOOLS_DARWIN)
	@cp $(PLATFORMTOOLS_FOLDER)/adb $(ASSETS_FOLDER)/adb_darwin

	@./scripts/download_platform_tools.sh linux

	@rm -rf $(PLATFORMTOOLS_FOLDER)
	@cd /tmp && unzip -u $(PLATFORMTOOLS_DOWNLOAD_FOLDER)/$(PLATFORMTOOLS_LINUX)
	@cp $(PLATFORMTOOLS_FOLDER)/adb $(ASSETS_FOLDER)/adb_linux

all: collector windows darwin linux

clean:
	rm -rf $(BUILD_FOLDER) $(DIST_FOLDER)
	rm -f $(ASSETS_FOLDER)/adb $(ASSETS_FOLDER)/adb_darwin $(ASSETS_FOLDER)/adb_linux $(ASSETS_FOLDER)/adb.exe $(ASSETS_FOLDER)/AdbWinApi.dll $(ASSETS_FOLDER)/AdbWinUsbApi.dll rm -f $(ASSETS_FOLDER)/collector_*
	cd android-collector && $(MAKE) clean
