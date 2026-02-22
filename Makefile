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

PLATFORMTOOLS_URL     = https://dl.google.com/android/repository/
PLATFORMTOOLS_WINDOWS = platform-tools-latest-windows.zip
PLATFORMTOOLS_DARWIN  = platform-tools-latest-darwin.zip
PLATFORMTOOLS_LINUX   = platform-tools-latest-linux.zip
PLATFORMTOOLS_FOLDER  = /tmp/platform-tools

check:
	@echo "[lint] Running go vet"
	go vet ./...
	@echo "[lint] Running staticheck on codebase"
	@staticcheck ./...

vuln:
	@echo "Running go vuln check"
	@govulncheck ./...

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

	@if [ ! -f /tmp/$(PLATFORMTOOLS_WINDOWS) ]; then \
		echo "Downloading Windows Android Platform Tools..."; \
		wget $(PLATFORMTOOLS_URL)$(PLATFORMTOOLS_WINDOWS) -O /tmp/$(PLATFORMTOOLS_WINDOWS); \
	fi

	@rm -rf $(PLATFORMTOOLS_FOLDER)
	@cd /tmp && unzip -u $(PLATFORMTOOLS_WINDOWS)
	@cp $(PLATFORMTOOLS_FOLDER)/AdbWinApi.dll $(ASSETS_FOLDER)
	@cp $(PLATFORMTOOLS_FOLDER)/AdbWinUsbApi.dll $(ASSETS_FOLDER)
	@cp $(PLATFORMTOOLS_FOLDER)/adb.exe $(ASSETS_FOLDER)

	@echo "[builder] Building Windows binary for amd64"

	$(FLAGS_WINDOWS) go build --ldflags '$(LD_FLAGS) -extldflags "-static"' -o $(BUILD_FOLDER)/androidqf_windows_amd64.exe .

	@echo "[builder] Done!"

darwin:
	@mkdir -p $(BUILD_FOLDER)

	@if [ ! -f /tmp/$(PLATFORMTOOLS_DARWIN) ]; then \
		echo "Downloading Darwin Android Platform Tools..."; \
		wget $(PLATFORMTOOLS_URL)$(PLATFORMTOOLS_DARWIN) -O /tmp/$(PLATFORMTOOLS_DARWIN); \
	fi

	@rm -rf $(PLATFORMTOOLS_FOLDER)
	@cd /tmp && unzip -u $(PLATFORMTOOLS_DARWIN)
	@cp $(PLATFORMTOOLS_FOLDER)/adb $(ASSETS_FOLDER)

	@echo "[builder] Building Darwin binary for amd64"

	$(FLAGS_DARWIN) GOARCH=amd64 go build --ldflags '$(LD_FLAGS)' -o $(BUILD_FOLDER)/androidqf_darwin_amd64 .
	$(FLAGS_DARWIN) GOARCH=arm64 go build --ldflags '$(LD_FLAGS)' -o $(BUILD_FOLDER)/androidqf_darwin_arm64 .

	@echo "[builder] Done!"

linux:
	@mkdir -p $(BUILD_FOLDER)

	@if [ ! -f /tmp/$(PLATFORMTOOLS_LINUX) ]; then \
		echo "Downloading Linux Android Platform Tools..."; \
		wget $(PLATFORMTOOLS_URL)$(PLATFORMTOOLS_LINUX) -O /tmp/$(PLATFORMTOOLS_LINUX); \
	fi

	@rm -rf $(PLATFORMTOOLS_FOLDER)
	@cd /tmp && unzip -u $(PLATFORMTOOLS_LINUX)
	@cp $(PLATFORMTOOLS_FOLDER)/adb $(ASSETS_FOLDER)

	@echo "[builder] Building Linux binary for amd64"

	@$(FLAGS_LINUX) GOARCH=amd64 go build --ldflags '$(LD_FLAGS)' -o $(BUILD_FOLDER)/androidqf_linux_amd64 .
	@$(FLAGS_LINUX) GOARCH=arm64 go build --ldflags '$(LD_FLAGS)' -o $(BUILD_FOLDER)/androidqf_linux_arm64 .

	@echo "[builder] Done!"

download:
	@if [ ! -f /tmp/$(PLATFORMTOOLS_WINDOWS) ]; then \
		echo "Downloading Windows Android Platform Tools..."; \
		wget $(PLATFORMTOOLS_URL)$(PLATFORMTOOLS_WINDOWS) -O /tmp/$(PLATFORMTOOLS_WINDOWS); \
	fi

	@rm -rf $(PLATFORMTOOLS_FOLDER)
	@cd /tmp && unzip -u $(PLATFORMTOOLS_WINDOWS)
	@cp $(PLATFORMTOOLS_FOLDER)/AdbWinApi.dll $(ASSETS_FOLDER)
	@cp $(PLATFORMTOOLS_FOLDER)/AdbWinUsbApi.dll $(ASSETS_FOLDER)
	@cp $(PLATFORMTOOLS_FOLDER)/adb.exe $(ASSETS_FOLDER)

	@if [ ! -f /tmp/$(PLATFORMTOOLS_DARWIN) ]; then \
		echo "Downloading Darwin Android Platform Tools..."; \
		wget $(PLATFORMTOOLS_URL)$(PLATFORMTOOLS_DARWIN) -O /tmp/$(PLATFORMTOOLS_DARWIN); \
	fi

	@rm -rf $(PLATFORMTOOLS_FOLDER)
	@cd /tmp && unzip -u $(PLATFORMTOOLS_DARWIN)
	@cp $(PLATFORMTOOLS_FOLDER)/adb $(ASSETS_FOLDER)

all: collector windows darwin linux

clean:
	rm -rf $(BUILD_FOLDER) $(DIST_FOLDER)
	rm -f $(ASSETS_FOLDER)/adb $(ASSETS_FOLDER)/adb_darwin $(ASSETS_FOLDER)/adb_linux $(ASSETS_FOLDER)/adb.exe $(ASSETS_FOLDER)/AdbWinApi.dll $(ASSETS_FOLDER)/AdbWinUsbApi.dll rm -f $(ASSETS_FOLDER)/collector_*
	cd android-collector && $(MAKE) clean
