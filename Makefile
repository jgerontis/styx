BINARY := styx
BUILD_DIR := bin
CMD := ./cmd/styx

.PHONY: build install test check clean

build:
	go build -o $(BUILD_DIR)/$(BINARY) $(CMD)

install:
	go install $(CMD)

test:
	go test ./...

check:
	gofmt -w .
	go vet ./...
	go test ./...

clean:
	rm -rf $(BUILD_DIR)