# in the current directory
go mod init dbutil
go mod tidy
go build .

dsetia@instance-20250401-161229-deepinder:~/securite/db$ ./dbutil -help
Usage of ./dbutil:
  -api-key string
        API Key (UUID)
  -device-id string
        Device ID (UUID)
  -name string
        Name (for tenant or device)
  -operation string
        Operation to perform: add-tenant, add-device, add-api-key, query-tenant, query-device, validate, list-tenants, list-devices, list-api-keys
  -tenant-id int
        Tenant ID (optional if tenant-name is provided)
  -tenant-name string
        Tenant Name (alternative to tenant-id)


# updated 4/10/2025
go build -o dbutil dbutil.go

dsetia@instance-20250401-161229-deepinder:~/securite/db$ ./dbutil -help
Usage of ./dbutil:
  -api-key string
        API key
  -device-id string
        Device ID
  -device-name string
        Device name
  -image-version string
        Image version
  -op string
        Operation: insert, validate, list
  -rules-version string
        Rules version
  -table string
        Table: tenants, devices, api_keys, global_manifest, device_manifest
  -tenant-id int
        Tenant ID
  -tenant-name string
        Tenant name
  -threatfeed-version string
        Threatfeed version

# insert
# -db "postgres://pguser:pgpass@localhost:5432/pgdb?sslmode=disable"
./dbutil -op insert -table tenants -tenant-name "TenantA"
./dbutil -op insert -table devices -device-id "dev1" -tenant-id 1 -device-name "Device1"
./dbutil -op insert -table api_keys -api-key "key1" -tenant-id 1 -device-id "dev1"
./dbutil -op insert -table global_manifest -image-version "v1.2.3" -threatfeed-version "2025.04.01.001"
./dbtool -op insert-status -device-id "dev1" -tenant-id 1 -status-image "failure" -status-rules "failure" -status-malware "failure"
# validate
./dbutil -op validate -table tenants -tenant-id 1
./dbutil -op validate -table devices -device-id "dev1"
./dbutil -op validate -table api_keys -api-key "key1"
./dbutil -op validate -table global_manifest -tenant-id 1  # tenant-id repurposed as global_manifest_id
# list
./dbutil -op list -table tenants
./dbutil -op list -table devices
./dbutil -op list -table api_keys
./dbutil -op list -table global_manifest
./dbutil -op list -table device_manifest
# update
~/securite/db/dbutil -op update -table global_manifest -image-version "v1.2.3"


# v2
./dbtool -db /home/dsetia/securite/apis/updater.db -op insert-tenant -tenant-name tenant2
./dbtool -db /home/dsetia/securite/apis/updater.db -op list-tenants
./dbtool -op insert-device -tenant-id 2 -device-id dev1 -device-name "Device 1" -hndr-sw-version v1.2.3
./dbtool -op validate-api-key -api-key a04cfb37-b7f5-4039-8538-a1cd961b298a
./dbtool -op insert-hndr-sw -sw-version v1.2.3 -sw-size 1024 -sw-sha256 abc123
sudo docker cp dbutil apis-container:/app/dbutil
sudo docker exec apis-container ./dbutil -db /app/updater.db -op insert-tenant -tenant-name tenant2

# postgres
sudo apt install postgresql-client
psql -h localhost -U pguser -d pgdb
 > pgpass
# init PG db
./init_pg.sh pgdb schema_pg.sql
# drop the DB
psql -h localhost -U pguser -d postgres
 postgres=# DROP DATABASE pgdb;
 DROP DATABASE
 postgres=# CREATE DATABASE pgdb;
 CREATE DATABASE
 postgres=# \c testdb
 postgres=# \dt

# go code
Just execute a query	db.Exec(...)
Get a single row result	db.QueryRow(...)
Insert & get auto ID	db.QueryRow(...).Scan(...) with RETURNING
Insert multiple rows, no return	db.Exec(...)
