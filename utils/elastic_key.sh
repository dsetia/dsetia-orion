#!/bin/bash

# This is used to fetch API key from Elastic container for testing
# It also generates a sample filebeat.yml in the current directory

# Ensure elastic.pem exists in the current directory
if [ ! -f "elastic.pem" ]; then
  echo "Error: elastic.pem not found in $(pwd)"
  exit 1
fi

# Reset elastic password and capture it, stripping newlines and whitespace
NEW_PASSWORD=$(docker exec es01 /usr/share/elasticsearch/bin/elasticsearch-reset-password -u elastic -b | grep "New value" | awk '{print $NF}' | tr -d '\r\n' | sed 's/[[:space:]]*$//')
if [ -z "$NEW_PASSWORD" ]; then
  echo "Error: Failed to capture new password"
  exit 1
fi
echo "Elastic password: $NEW_PASSWORD"

# Wait for Elasticsearch to apply the password reset
sleep 5

# Create API key
API_KEY_RESPONSE=$(curl -s -X POST "https://localhost:9200/_security/api_key" -u "elastic:$NEW_PASSWORD" --cacert elastic.pem -H "Content-Type: application/json" -d '{"name": "filebeat_api_key", "role_descriptors": {"filebeat_writer": {"cluster": ["monitor", "read_ilm", "read_pipeline", "manage_ilm", "manage_index_templates", "manage_ingest_pipelines", "cluster:monitor/main"], "index": [{"names": ["filebeat-*", ".monitoring-beats-*", ".ds-filebeat-*", ".ds-hndr.eve-*"], "privileges": ["all"]}]}}}')
if [ $? -ne 0 ]; then
  echo "Error: curl command failed"
  echo "Response: $API_KEY_RESPONSE"
  exit 1
fi

# Extract id and api_key for Filebeat
ID=$(echo "$API_KEY_RESPONSE" | jq -r .id)
API_KEY=$(echo "$API_KEY_RESPONSE" | jq -r .api_key)
API_KEY_ENCODED=$(echo "$API_KEY_RESPONSE" | jq -r .encoded)
if [ -z "$ID" ] || [ -z "$API_KEY" ]; then
  echo "Error: Failed to parse API key ID or value"
  echo "Response: $API_KEY_RESPONSE"
  exit 1
fi
ID_KEY="$ID:$API_KEY"
echo "API Key (id:key): $ID_KEY"
echo "API Key Encoded : $API_KEY_ENCODED"

# Generate filebeat.yml with id:key format
cat <<EOF > filebeat.yml
filebeat.inputs:
  - type: filestream
    id: suricata-eve
    enabled: true
    paths:
      - /var/log/suricata/eve.json
    parsers:
      - ndjson:
          overwrite_keys: true
    fields_under_root: true
    fields:
      dataset: hndr.eve
      type: log
      namespace: default

output.elasticsearch:
  hosts: ["https://localhost:9200"]
  api_key: "$ID_KEY"
  ssl:
    verification_mode: full
    certificate_authorities: ["/etc/filebeat/certs/elastic.pem"]

setup.ilm.enabled: true
setup.template.enabled: true
setup.template.type: "data_stream"

logging.level: info
logging.to_files: true
logging.files:
  path: /var/log/filebeat
  name: filebeat
  keepfiles: 7
  permissions: 0644
EOF
echo "Generated filebeat.yml with API key"

# set permissions so only owner has write access
chmod go-w filebeat.yml
