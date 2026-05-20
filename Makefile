BINARY := opvar
DIST_DIR := dist
OUT := $(DIST_DIR)/$(BINARY)
BINDIR ?= /usr/local/bin
VERSION_FILE := VERSION
VERSION ?= $(shell cat $(VERSION_FILE) 2>/dev/null || echo dev)
LDFLAGS := -X github.com/mrcat71/opvar/internal/cli.Version=$(VERSION)

.PHONY: build test vet fmt install clean

build:
	mkdir -p $(DIST_DIR)
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(OUT) ./cmd/opvar

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

install: build
	install -d "$(BINDIR)"
	install -m 0755 "$(OUT)" "$(BINDIR)/$(BINARY)"
	@echo "Installed: $(BINDIR)/$(BINARY)"

clean:
	rm -rf $(DIST_DIR)
