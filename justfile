build:
  go build -C ./cmd/relay -o ../../bin/relay

run: build
  cd bin && ./relay --config=../cmd/relay/debug.yaml

build-all:
    env GOOS=linux GOARCH=amd64 go build -C ./cmd/relay -o ../../bin/relay_linux
    env GOOS=darwin GOARCH=arm64 go build -C ./cmd/relay -o ../../bin/relay_darwin