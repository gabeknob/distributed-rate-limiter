BINARY=benchmark

.PHONY: build run clean

build:
	go build -o $(BINARY) .

run:
	go run . $(ARGS)

clean:
	rm -f $(BINARY)
