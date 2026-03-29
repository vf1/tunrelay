build:
  go build -C ./cmd/relay -o ../../bin/tunrelay

run: build
  cd bin && ./tunrelay --config=../cmd/relay/debug.yaml

build-all:
    env GOOS=linux GOARCH=amd64 go build -C ./cmd/relay -o ../../bin/tunrelay_linux
    env GOOS=darwin GOARCH=arm64 go build -C ./cmd/relay -o ../../bin/tunrelay_darwin