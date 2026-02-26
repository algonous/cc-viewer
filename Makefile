BIN ?= $(HOME)/.local/bin
BINARY = cc-tree

.PHONY: all test build clean

all: test build

test:
	go test ./...

build:
	go build -o $(BIN)/$(BINARY) .

clean:
	rm -f $(BIN)/$(BINARY)
