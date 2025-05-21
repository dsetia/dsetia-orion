# ──────────────────────────────────────────────────────────────────────────────
# Makefile — build all Go sub‑modules when their code changes
# ──────────────────────────────────────────────────────────────────────────────

# where we’ll drop the binaries
BINDIR := $(HOME)/go/bin

# cross‑compile settings (exported for every go build)
GOOS        := linux
GOARCH      := amd64
CGO_ENABLED := 0
export GOOS GOARCH CGO_ENABLED

# collect .go files in each package
APIS_SRCS        := $(wildcard apis/*.go)
DB_SRCS          := $(wildcard db/*.go)
UPDATER_SRCS     := $(wildcard updater/*.go)
OBJUPDATER_SRCS  := $(shell find updater -name '*.go')
PROVISIONER_SRCS := $(wildcard provisioner/*.go)

# ─── Phony targets ────────────────────────────────────────────────────────────
.PHONY: all clean apis dbtool updater objupdater provisioner

all: $(BINDIR) \
     $(BINDIR)/apis \
     $(BINDIR)/dbtool \
     $(BINDIR)/updater \
     $(BINDIR)/objupdater \
     $(BINDIR)/provisioner

# ensure bin/ exists
$(BINDIR):
	mkdir -p $@

# ─── Build rules ──────────────────────────────────────────────────────────────

# apis
$(BINDIR)/apis: $(APIS_SRCS) | $(BINDIR)
	@echo "Building apis → $@"
	cd apis && go build -o $(BINDIR)/apis

# dbtool
$(BINDIR)/dbtool: $(DB_SRCS) | $(BINDIR)
	@echo "Building dbtool → $@"
	cd db && go build -o $(BINDIR)/dbtool

# updater
$(BINDIR)/updater: $(UPDATER_SRCS) | $(BINDIR)
	@echo "Building updater → $@"
	cd updater && go build -o $(BINDIR)/updater
	#
# objupdater
$(BINDIR)/objupdater: $(OBJUPDATER_SRCS) | $(BINDIR)
	@echo "Building objupdater → $@"
	cd objupdater && go build -o $(BINDIR)/objupdater

# provision‑sensor
$(BINDIR)/provisioner: $(PROVISIONER_SRCS) | $(BINDIR)
	@echo "Building provisioner → $@"
	cd provisioner && go build -o $(BINDIR)/provisioner

# ─── Install ─────────────────────────────────────────────────────────────────
install:
	sudo cp $(BINDIR)/apis /usr/local/bin
	sudo cp $(BINDIR)/dbtool /usr/local/bin
	sudo cp $(BINDIR)/updater /usr/local/bin
	sudo cp $(BINDIR)/objupdater /usr/local/bin
	sudo cp $(BINDIR)/provisioner /usr/local/bin

# ─── Cleanup ─────────────────────────────────────────────────────────────────
clean:
	rm -rf $(BINDIR)
