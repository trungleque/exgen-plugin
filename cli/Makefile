BINARY  := exgen
PLUGIN  := ../plugins/exgen
BUNDLED := internal/bundled/exgen
VERSION ?= $(shell sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' $(PLUGIN)/.claude-plugin/plugin.json | head -1)
LDFLAGS := -X github.com/trungleque/exgen-plugin/cli/internal/cmd.Version=$(VERSION)

.PHONY: all sync build test check fmt vet install clean

all: check build

## sync: refresh the embedded copy of the plugin.
##
## go:embed cannot reach outside the cli/ module, so the plugin is mirrored
## here at build time. TestBundledMatchesSource fails if the two drift.
sync:
	rm -rf $(BUNDLED)
	cp -R $(PLUGIN) $(BUNDLED)
	find $(BUNDLED) -name .DS_Store -delete

build: sync
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/exgen

test:
	go test ./...

fmt:
	gofmt -l -w .

vet:
	go vet ./...

check: vet test

install: sync
	go install -ldflags "$(LDFLAGS)" ./cmd/exgen

clean:
	rm -f $(BINARY)
