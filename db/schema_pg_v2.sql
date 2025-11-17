-- ===================================================================
-- SCHEMA VERSION 2: Multi-Environment Tenant Management
-- ===================================================================
-- This schema supports tenant ID allocation across multiple cloud
-- environments with distinct ID ranges
-- ===================================================================

-- Tenant ID Blocks Table (STATIC - Identical across all environments)
-- This table defines ID ranges for each environment
CREATE TABLE IF NOT EXISTS tenant_id_blocks (
    environment TEXT PRIMARY KEY,
    start_id BIGINT NOT NULL,
    end_id BIGINT NOT NULL,
    description TEXT,
    CHECK (end_id >= start_id),
    CHECK (start_id > 0)
);

-- Insert predefined ID blocks (same across ALL environments)
INSERT INTO tenant_id_blocks (environment, start_id, end_id, description) VALUES
    ('private-staging',  1,     1000,   'Private staging tenants (1-1000)'),
    ('private-prod',     1001,  10000,  'Private production tenants (1001-10000)'),
    ('aws-prod',         11000, 20000,  'AWS production tenants (11000-20000)'),
    ('gcloud-prod',      21000, 30000,  'GCloud production tenants (21000-30000)'),
    ('azure-prod',       31000, 40000,  'Azure production tenants (31000-40000)')
ON CONFLICT (environment) DO NOTHING;

-- Create sequences for each environment
CREATE SEQUENCE IF NOT EXISTS seq_private_staging_tenant_id
    START WITH 1 INCREMENT BY 1 MINVALUE 1 MAXVALUE 1000 NO CYCLE;

CREATE SEQUENCE IF NOT EXISTS seq_private_prod_tenant_id
    START WITH 1001 INCREMENT BY 1 MINVALUE 1001 MAXVALUE 10000 NO CYCLE;

CREATE SEQUENCE IF NOT EXISTS seq_aws_prod_tenant_id
    START WITH 11000 INCREMENT BY 1 MINVALUE 11000 MAXVALUE 20000 NO CYCLE;

CREATE SEQUENCE IF NOT EXISTS seq_gcloud_prod_tenant_id
    START WITH 21000 INCREMENT BY 1 MINVALUE 21000 MAXVALUE 30000 NO CYCLE;

CREATE SEQUENCE IF NOT EXISTS seq_azure_prod_tenant_id
    START WITH 31000 INCREMENT BY 1 MINVALUE 31000 MAXVALUE 40000 NO CYCLE;

-- Tenants Table (DYNAMIC - Different per environment)
CREATE TABLE tenants (
    tenant_id BIGINT PRIMARY KEY,
    tenant_name TEXT NOT NULL UNIQUE,
    environment TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (environment) REFERENCES tenant_id_blocks(environment)
);

-- Devices table
CREATE TABLE devices (
    device_id TEXT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    device_name TEXT,
    hndr_sw_version TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id) ON DELETE CASCADE
);

-- API Keys table
CREATE TABLE api_keys (
    api_key TEXT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    device_id TEXT NOT NULL,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    FOREIGN KEY (device_id) REFERENCES devices(device_id) ON DELETE CASCADE
);

-- Handler Software table
CREATE TABLE hndr_sw (
    id SERIAL PRIMARY KEY,
    version TEXT NOT NULL,
    size INTEGER NOT NULL,
    sha256 TEXT NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Handler Rules table
CREATE TABLE hndr_rules (
    id SERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    version TEXT NOT NULL,
    size INTEGER NOT NULL,
    sha256 TEXT NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id) ON DELETE CASCADE
);

-- Threat Intelligence table
CREATE TABLE threatintel (
    id SERIAL PRIMARY KEY,
    version TEXT NOT NULL,
    size INTEGER NOT NULL,
    sha256 TEXT NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Status table
CREATE TABLE status (
    device_id TEXT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    software TEXT NOT NULL,
    rules TEXT NOT NULL,
    threatintel TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id) ON DELETE CASCADE
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_device_tenant ON devices(tenant_id);
CREATE INDEX IF NOT EXISTS idx_status_tenant ON status(tenant_id);
CREATE INDEX IF NOT EXISTS idx_api_key_tenant_device ON api_keys(tenant_id, device_id);
CREATE INDEX IF NOT EXISTS idx_hndr_rules_tenant ON hndr_rules(tenant_id);
CREATE INDEX IF NOT EXISTS idx_hndr_sw_updated ON hndr_sw(updated_at);
CREATE INDEX IF NOT EXISTS idx_hndr_rules_updated ON hndr_rules(updated_at);
CREATE INDEX IF NOT EXISTS idx_threatintel_updated ON threatintel(updated_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_hndr_sw_version ON hndr_sw(version);
CREATE UNIQUE INDEX IF NOT EXISTS idx_hndr_rules_version ON hndr_rules(version, tenant_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_threatintel_version ON threatintel(version);
CREATE INDEX IF NOT EXISTS idx_tenant_environment ON tenants(environment);

-- Function to get next tenant ID for a given environment
CREATE OR REPLACE FUNCTION get_next_tenant_id(env TEXT)
RETURNS BIGINT AS $$
DECLARE
    next_id BIGINT;
    seq_name TEXT;
BEGIN
    -- Construct sequence name from environment (replace hyphen with underscore)
    seq_name := 'seq_' || replace(env, '-', '_') || '_tenant_id';
    
    -- Get next value from the appropriate sequence
    EXECUTE format('SELECT nextval(%L)', seq_name) INTO next_id;
    
    RETURN next_id;
END;
$$ LANGUAGE plpgsql;

-- Function to validate that tenant ID is within the correct range
CREATE OR REPLACE FUNCTION validate_tenant_id_range()
RETURNS TRIGGER AS $$
DECLARE
    block_start BIGINT;
    block_end BIGINT;
BEGIN
    -- Get the ID range for this environment
    SELECT start_id, end_id INTO block_start, block_end
    FROM tenant_id_blocks
    WHERE environment = NEW.environment;
    
    -- Validate that the tenant_id is within the allowed range
    IF NEW.tenant_id < block_start OR NEW.tenant_id > block_end THEN
        RAISE EXCEPTION 'Tenant ID % is outside the valid range [%--%] for environment=%',
            NEW.tenant_id, block_start, block_end, NEW.environment;
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to validate tenant ID range on insert or update
CREATE TRIGGER trg_validate_tenant_id_range
    BEFORE INSERT OR UPDATE ON tenants
    FOR EACH ROW
    EXECUTE FUNCTION validate_tenant_id_range();

-- View to show current tenant allocation status per environment
CREATE OR REPLACE VIEW tenant_allocation_status AS
SELECT 
    b.environment,
    b.start_id,
    b.end_id,
    b.end_id - b.start_id + 1 AS total_capacity,
    COUNT(t.tenant_id) AS allocated_count,
    b.end_id - b.start_id + 1 - COUNT(t.tenant_id) AS remaining_capacity,
    ROUND(100.0 * COUNT(t.tenant_id) / (b.end_id - b.start_id + 1), 2) AS utilization_percent
FROM tenant_id_blocks b
LEFT JOIN tenants t ON b.environment = t.environment
GROUP BY b.environment, b.start_id, b.end_id
ORDER BY b.start_id;
