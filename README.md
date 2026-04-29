## SecurITe License

```
// Copyright (c) 2025 SecurITe
// All rights reserved.
//
// This source code is the property of SecurITe.
// Unauthorized copying, modification, or distribution of this file,
// via any medium is strictly prohibited unless explicitly authorized
// by SecurITe.
//
// This software is proprietary and confidential.
//
// File Owner:       deepinder@securite.world
// Created On:       04/28/2026
```

# Orion Management Services

This repository uses `docker-compose.yml` to run the basic Orion management
services locally. The Docker services are managed through the `orion.sh` shell
script, which wraps the common launch, stop, restart, status, and test commands.

The compose stack includes:

- `nginx`: reverse proxy on ports `80` and `443`
- `minio`: object store on ports `9000` and `9001`
- `apis-container`: Orion API service on port `8080`
- `postgres`: PostgreSQL database on port `5432`
- `redpanda`: Kafka-compatible broker on ports `9092` and `9644`

## Prerequisites

Install the following tools on the host:

- Docker Engine
- Docker Compose
- Go toolchain
- `jq`
- PostgreSQL client tools, including `psql` and `createdb`
- MinIO client, `mc`

On Ubuntu, the common packages are:

```sh
sudo apt-get update
sudo apt-get install -y docker.io docker-compose jq postgresql-client
```

Install the MinIO client if it is not already available:

```sh
curl https://dl.min.io/client/mc/release/linux-amd64/mc -o mc
chmod +x mc
sudo mv mc /usr/local/bin/mc
```

## Build And Install

Build the Orion binaries and install the runtime configuration used by the
Docker services:

```sh
git pull
make all
make install
make install-utils
make config
```

`make config` installs configuration under `/opt/config`. The compose file
mounts these files into the containers, including:

- `/opt/config/nginx/nginx.conf`
- `/opt/config/db.json`
- `/opt/config/minio.json`
- `/opt/config/schema_pg_v2.sql`

## Manage Services

Use `orion.sh` for normal service lifecycle operations:

```sh
./orion.sh launch
./orion.sh status
./orion.sh restart
./orion.sh stop
```

`./orion.sh launch` calls `launch.sh`, which starts `docker-compose.yml` with
`up -d --build` and verifies that the expected containers are running.

For direct Docker Compose access, use:

```sh
docker-compose -f docker-compose.yml ps
docker-compose -f docker-compose.yml logs -f
docker-compose -f docker-compose.yml down
```

## Initialize Services

After the containers are running, initialize PostgreSQL and MinIO:

```sh
init_db.sh /opt/config/db.json /opt/config/schema_pg_v2.sql
init_minio.sh /opt/config/minio.json
```

The database initialization script drops and recreates the configured database
after confirmation. Verify the printed configuration before approving the
prompt.

To populate the database with development data:

```sh
populate_db.sh /opt/config/db.json
```

To populate MinIO with the sample data in `minio/`:

```sh
populate_minio.sh /opt/config/db.json /opt/config/minio.json minio/
```

## Validate The Stack

Run the sanity test after launch and initialization:

```sh
./orion.sh test
```

Useful local endpoints:

- API service: `http://localhost:8080`
- MinIO API: `http://localhost:9000`
- MinIO console: `http://localhost:9001`
- Nginx: `http://localhost`
- PostgreSQL: `localhost:5432`
- Redpanda Kafka: `localhost:9092`

## Notes

- `docker-compose.yml` uses named and local volumes for service data. Stopping
  the stack with `./orion.sh stop` removes containers and the compose network,
  but it does not delete the named PostgreSQL volume.
- MinIO data is stored under `./minio/data`.
- The default development credentials are defined in `docker-compose.yml` and
  the matching files under `config/`.
