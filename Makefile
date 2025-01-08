BUILD_ENV := CGO_ENABLED=0
DIR := $(shell pwd)
LDFLAGS := -ldflags "-w -s"
TARGET_EXEC := helper

.PHONY: all clean setup build-linux build-osx build-win copy

all: clean setup build-linux build-linux-arm64 build-osx build-win copy

clean:
	rm -rf bin

setup:
	mkdir -p bin/linux
	mkdir -p bin/osx
	mkdir -p bin/windows

copy: clean setup
	cp config.yaml bin/config.yaml

build-linux: copy
	${BUILD_ENV} GOARCH=amd64 GOOS=linux go build ${LDFLAGS} -o bin/linux/${TARGET_EXEC} -trimpath cmd/command.go

build-linux-arm64: copy
	${BUILD_ENV} GOARCH=arm64 GOOS=linux go build ${LDFLAGS} -o bin/linux/${TARGET_EXEC}-arm64 -trimpath cmd/command.go

build-osx: copy
	${BUILD_ENV} GOARCH=amd64 GOOS=darwin go build ${LDFLAGS} -o bin/osx/${TARGET_EXEC} -trimpath cmd/command.go

build-win: copy
	${BUILD_ENV} GOARCH=amd64 GOOS=windows go build ${LDFLAGS} -o bin/windows/${TARGET_EXEC}.exe -trimpath cmd/command.go
