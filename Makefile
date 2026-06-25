.PHONY: build run test clean

# 變數
BINARY_NAME=incus-plugin
CMD_DIR=./cmd/incus-plugin
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.version=${VERSION} -X main.buildTime=${BUILD_TIME} -s -w"
BIN_DIR=bin

build:
	@mkdir -p $(BIN_DIR)
	go build ${LDFLAGS} -o $(BIN_DIR)/$(BINARY_NAME) $(CMD_DIR)

run: build
	$(BIN_DIR)/$(BINARY_NAME)

test:
	go test ./...

clean:
	go clean
	rm -rf $(BIN_DIR)
