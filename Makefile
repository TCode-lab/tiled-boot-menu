BIN := bin/boot-menu
GO_FILES := $(shell find . -name '*.go' -not -path './bin/*')

.PHONY: all build run fmt test vet race tidy clean

all: build

build:
	go build -o $(BIN) ./cmd/boot-menu

run: build
	./$(BIN)

fmt:
	gofmt -w $(GO_FILES)

test:
	go test ./...

vet:
	go vet ./...

race:
	go test -race ./...

tidy:
	go mod tidy

clean:
	rm -rf bin
