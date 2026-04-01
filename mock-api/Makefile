BINARY=mock-api
GOOS?=linux
GOARCH?=arm64

.PHONY: build run clean

build:
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o $(BINARY) .

run:
	go run .

clean:
	rm -f $(BINARY)
