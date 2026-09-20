# Build stage
FROM golang:1.27.0-alpine AS builder

ENV CGO_ENABLED=1

WORKDIR /app

# 安裝編譯依賴和 golangci-lint
RUN apk add --no-cache git curl build-base
RUN curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b /usr/local/bin v2.10.1


# 複製 go mod 檔案
COPY go.mod go.sum ./
RUN go mod download

# 複製程式碼
COPY . .

# 執行測試
RUN go test -v -race -coverprofile=coverage.out ./...

# 執行 lint
RUN GOFLAGS="-buildvcs=false" golangci-lint run --timeout=5m

# 編譯
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -a -o incus-plugin ./cmd/incus-plugin
