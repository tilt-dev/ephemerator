.PHONY: install

lint:
	golangci-lint run -v --timeout 180s

install:
	go get ./...
	go install ./cmd/ephdash

build-static:
	go get ./...
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/ephdash ./cmd/ephdash
