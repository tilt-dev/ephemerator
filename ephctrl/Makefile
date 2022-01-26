.PHONY: install

install:
	go get ./...
	go install ./cmd/ephctrl

build-static:
	go get ./...
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/ephctrl ./cmd/ephctrl
