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
UPDATER_SRCS     := $(shell find updater -name '*.go')
OBJUPDATER_SRCS  := $(wildcard objupdater/*.go)
PROVISIONER_SRCS := $(wildcard provisioner/*.go)

# ─── Phony targets ────────────────────────────────────────────────────────────
.PHONY: all clean apis dbtool updater objupdater provisioner config

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

# ─── Config ─────────────────────────────────────────────────────────────────
config:
	sudo mkdir -p /opt/config/nginx /opt/config/scripts /opt/config/supervisor /opt/config/provisioner /opt/config/logrotate.d
	sudo cp config/db.json config/db_dev.json /opt/config/
	sudo cp config/minio.json /opt/config/
	sudo cp config/filebeat.yml /opt/config/
	sudo cp db/schema_pg_v3.sql /opt/config/
	sudo cp nginx/nginx.conf /opt/config/nginx/
	sudo cp config/provisioner/* /opt/config/provisioner/
	sudo cp config/scripts/* /opt/config/scripts/
	sudo cp config/supervisor/* /opt/config/supervisor/
	sudo cp config/logrotate.d/securite /opt/config/logrotate.d/

# ─── Install Utils ─────────────────────────────────────────────────────────────────
install-utils:
	sudo cp utils/* /usr/local/bin

# ─── Cleanup ─────────────────────────────────────────────────────────────────
clean:
	rm -rf $(BINDIR)
