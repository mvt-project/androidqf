BUILD_FOLDER  = $(shell pwd)/build

.PHONY: build
default: build ;

check:
	@echo "[lint] Running go vet"
	go vet ./...
	@echo "[lint] Running staticheck on codebase"
	@staticcheck ./...

fmt:
	gofmt -w -s ./

build:
	env GOOS=linux GOARCH=arm GOARM=7 go build -o $(BUILD_FOLDER)/collector_arm7
	env GOOS=linux GOARCH=arm GOARM=6 go build -o $(BUILD_FOLDER)/collector_arm6
	env GOOS=linux GOARCH=amd64 go build -o $(BUILD_FOLDER)/collector_amd64
	env GOOS=linux GOARCH=arm64 go build -o $(BUILD_FOLDER)/collector_arm64

clean:
	rm -rf $(BUILD_FOLDER)

test: build
	adb shell "rm -rf /data/local/tmp/collector"
	adb push $(BUILD_FOLDER)/collector_arm7 /data/local/tmp/collector
	adb shell "chmod +x /data/local/tmp/collector"
	adb shell "/data/local/tmp/collector ps"
