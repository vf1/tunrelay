build:
  go build -C ./cmd/relay -o ../../bin/tunrelay

build-all:
    env GOOS=linux GOARCH=amd64 go build -C ./cmd/relay -o ../../bin/tunrelay_linux
    env GOOS=darwin GOARCH=arm64 go build -C ./cmd/relay -o ../../bin/tunrelay_darwin

nat: build
  cd bin && ./tunrelay --config=../config/nat.yaml

replace_ip: build
  cd bin && ./tunrelay --config=../config/replace_ip.yaml

replace_subnet: build
  cd bin && ./tunrelay --config=../config/replace_subnet.yaml