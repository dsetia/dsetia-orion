CREATE TABLE tenants (
    tenant_id SERIAL PRIMARY KEY,
    tenant_name TEXT NOT NULL UNIQUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE devices (
    device_id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    device_name TEXT,
    hndr_sw_version TEXT, -- Changed to nullable
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id) ON DELETE CASCADE
);

CREATE TABLE api_keys (
    api_key TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    device_id TEXT NOT NULL,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    FOREIGN KEY (device_id) REFERENCES devices(device_id) ON DELETE CASCADE
);
CREATE TABLE hndr_sw (
    id SERIAL PRIMARY KEY,
    version TEXT NOT NULL,
    size INTEGER NOT NULL,
    sha256 TEXT NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE hndr_rules (
    id SERIAL PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    version TEXT NOT NULL,
    size INTEGER NOT NULL,
    sha256 TEXT NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id) ON DELETE CASCADE
);
CREATE TABLE threatintel (
    id SERIAL PRIMARY KEY,
    version TEXT NOT NULL,
    size INTEGER NOT NULL,
    sha256 TEXT NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
-- Indexes
CREATE INDEX idx_device_tenant ON devices(tenant_id);
CREATE INDEX idx_api_key_tenant_device ON api_keys(tenant_id, device_id);
CREATE INDEX idx_hndr_rules_tenant ON hndr_rules(tenant_id);
CREATE INDEX idx_hndr_sw_updated ON hndr_sw(updated_at);
CREATE INDEX idx_hndr_rules_updated ON hndr_rules(updated_at);
CREATE INDEX idx_threatintel_updated ON threatintel(updated_at);
CREATE UNIQUE INDEX idx_hndr_sw_version ON hndr_sw(version);
CREATE UNIQUE INDEX idx_hndr_rules_version ON hndr_rules(version, tenant_id);
CREATE UNIQUE INDEX idx_threatintel_version ON threatintel(version);
