#!/bin/bash

mc alias set myminio http://localhost:9000 minioadmin minioadmin

# seed minio
mc mb myminio/images
mc mb myminio/rules/1
mc mb myminio/threatintel
mc cp hndr-sw-v1.2.3.tar.gz myminio/images/
mc cp hndr-rules-r1.2.3.tar.gz myminio/rules/1
mc cp threatintel-2025.04.10.1523.tar.gz myminio/threatintel/
