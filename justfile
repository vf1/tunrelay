build:
  go build -C ./cmd/relay -o ../../bin/tunrelay

build-all:
    env GOOS=linux GOARCH=amd64 go build -C ./cmd/relay -o ../../bin/tunrelay_linux
    env GOOS=darwin GOARCH=arm64 go build -C ./cmd/relay -o ../../bin/tunrelay_darwin
    env GOOS=windows GOARCH=amd64 go build -C ./cmd/relay -o ../../bin/tunrelay_windows.exe

nat: build
  cd bin && ./tunrelay --config=../config/nat.yaml

replace_ip: build
  cd bin && ./tunrelay --config=../config/replace_ip.yaml

replace_subnet: build
  cd bin && ./tunrelay --config=../config/replace_subnet.yaml

multi_client: build
  cd bin && ./tunrelay --config=../config/multi_client_linux.yaml

client: build
  cd bin && ./tunrelay --config=../config/client.yaml

test:
  # sudo requred for some tunep test
  go test ./internal/endpoint/tunep/
  go test ./internal/endpoint/udpep/
  go test ./internal/iptool/
  go test ./internal/relay/