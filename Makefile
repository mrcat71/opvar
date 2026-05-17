BINARY := opvar
DIST_DIR := dist
OUT := $(DIST_DIR)/$(BINARY)
BINDIR ?= /usr/local/bin
VERSION_FILE := VERSION
VERSION ?= $(shell cat $(VERSION_FILE) 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

.PHONY: build test install clean

build:
	mkdir -p $(DIST_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(OUT) .

test:
	go test ./...

install: build
	install -d "$(BINDIR)"
	install -m 0755 "$(OUT)" "$(BINDIR)/$(BINARY)"
	@echo "Installed: $(BINDIR)/$(BINARY)"

clean:
	rm -rf $(DIST_DIR)
