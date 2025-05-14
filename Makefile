# ──────────────────────────────────────────────────────────────────────────────
# Makefile — build all Go sub‑modules when their code changes
# ──────────────────────────────────────────────────────────────────────────────

# where we’ll drop the binaries
BINDIR := bin

# cross‑compile settings (exported for every go build)
GOOS        := linux
GOARCH      := amd64
CGO_ENABLED := 0
export GOOS GOARCH CGO_ENABLED

# collect .go files in each package
APIS_SRCS        := $(wildcard apis/*.go)
DB_SRCS          := $(wildcard db/*.go)
UPDATER_SRCS     := $(wildcard updater/*.go)
PROVISIONER_SRCS := $(wildcard provisioner/*.go)

# ─── Phony targets ────────────────────────────────────────────────────────────
.PHONY: all clean apis dbtool updater provision-sensor

all: $(BINDIR) \
     $(BINDIR)/apis \
     $(BINDIR)/dbtool \
     $(BINDIR)/updater \
     $(BINDIR)/provision-sensor

# ensure bin/ exists
$(BINDIR):
	mkdir -p $@

# ─── Build rules ──────────────────────────────────────────────────────────────

# apis
$(BINDIR)/apis: $(APIS_SRCS) | $(BINDIR)
	@echo "Building apis → $@"
	cd apis && go build -o ../$(BINDIR)/apis

# dbtool
$(BINDIR)/dbtool: $(DB_SRCS) | $(BINDIR)
	@echo "Building dbtool → $@"
	cd db && go build -o ../$(BINDIR)/dbtool

# updater
$(BINDIR)/updater: $(UPDATER_SRCS) | $(BINDIR)
	@echo "Building updater → $@"
	cd updater && go build -o ../$(BINDIR)/updater

# provision‑sensor
$(BINDIR)/provision-sensor: $(PROVISIONER_SRCS) | $(BINDIR)
	@echo "Building provision‑sensor → $@"
	cd provisioner && go build -o ../$(BINDIR)/provision-sensor

# ─── Cleanup ─────────────────────────────────────────────────────────────────
clean:
	rm -rf $(BINDIR)
