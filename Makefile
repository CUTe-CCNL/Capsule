.PHONY: build run test clean package install uninstall

# 變數
BINARY_NAME=incus-plugin
CMD_DIR=./cmd/incus-plugin
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.version=${VERSION} -X main.buildTime=${BUILD_TIME} -s -w"
BIN_DIR=bin
BUILD_FLAGS=-buildvcs=false

build:
	@mkdir -p $(BIN_DIR)
	go build $(BUILD_FLAGS) ${LDFLAGS} -o $(BIN_DIR)/$(BINARY_NAME) $(CMD_DIR)

run: build
	$(BIN_DIR)/$(BINARY_NAME)

test:
	go test ./...

clean:
	go clean
	rm -rf $(BIN_DIR)

package:
	bash scripts/package.sh

install:
	bash scripts/install.sh

uninstall:
	bash scripts/uninstall.sh
