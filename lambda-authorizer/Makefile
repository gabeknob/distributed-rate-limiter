BINARY=bootstrap
GOOS=linux
GOARCH=arm64

.PHONY: build zip clean

build:
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o $(BINARY) .

zip: build
	zip function.zip $(BINARY)

clean:
	rm -f $(BINARY) function.zip
