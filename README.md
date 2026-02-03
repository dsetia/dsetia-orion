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
// File Owner:       <email>@securite.world
// Created On:       MM/DD/YYYY
```

The 'orion.sh' script will pull all nginx, minio and postgres containers. Using docker compose
orchestrates the containers.

Dependencies
 - docker
   - Follow the link for details instruction on installing docker in Ubuntu
     - https://docs.docker.com/engine/install/ubuntu/
     - https://www.digitalocean.com/community/tutorials/how-to-install-and-use-docker-on-ubuntu-20-04
 - docker-compose
   - sudo apt install docker-compose # on Ubuntu
 - MinIO Client (mc)
   There are two options to use docker container or install locally
   - Docker Container
     - docker pull minio/mc
     - mc alias set myminio http://localhost:9000 minioadmin minioadmin
   - Install locally
     - curl https://dl.min.io/client/mc/release/linux-amd64/mc --create-dirs -o $HOME/minio-binaries/mc
     - chmod +x $HOME/minio-binaries/mc
     - export PATH=$PATH:$HOME/minio-binaries/
     - mc --help
  - psql
    - sudo apt install postgresql-client
    - init_db.sh config/db_dev_config.json db/schema_pg.sql
  - Minio client (mc)
    - curl https://dl.min.io/client/mc/release/linux-amd64/mc -o mc
    - chmod +x mc; sudo mv mc /usr/local/bin

# Deploy management services using docker containers
```
git pull
make all
make instal
make install-utils
make config

./orion.sh launch

init_minio.sh /opt/config/minio.json
init_db.sh /opt/config/db.json /opt/config/schema_pg_v2.sql
```

To populate minio with data for testing:
```
populate_minio.sh /opt/config/db.json /opt/config/minio.json minio/
```

Sanity test: 
```
./orion.sh test
```
