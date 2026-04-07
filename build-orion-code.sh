#!/bin/bash
#
# Copyright (c) 2025 SecurITe
# All rights reserved.
#
# This source code is the property of SecurITe.
# Unauthorized copying, modification, or distribution of this file,
# via any medium is strictly prohibited unless explicitly authorized
# by SecurITe.
#
# This software is proprietary and confidential.
#
# File Owner:       deepinder@securite.world
# Created On:       01/30/2026

set -e

# Print usage/help
usage() {
    echo "Usage: $0 environment"
    echo "  Build Orion code tarball"
    exit 1
}

DEPLOY_DIR="/tmp/deploy"
BINDIR=$HOME/go/bin
SRCDIR=$HOME/code/orion

# cleanup old stuff
rm -rf $DEPLOY_DIR

mkdir -p $DEPLOY_DIR/opt/db
mkdir -p $DEPLOY_DIR/opt/bin
mkdir -p $DEPLOY_DIR/opt/nginx
mkdir -p $DEPLOY_DIR/opt/docker

make all

# binaries and utils
cp $BINDIR/* $DEPLOY_DIR/opt/bin/
cp $SRCDIR/utils/* $DEPLOY_DIR/opt/bin/
cp $SRCDIR/db/migrate_v2_to_v3.sh $DEPLOY_DIR/opt/bin/
cp $SRCDIR/db/move-tenant.sh $DEPLOY_DIR/opt/bin/
cp $SRCDIR/db/tenant-info.sh $DEPLOY_DIR/opt/bin/

# schema
cp $SRCDIR/db/schema_pg.sql $DEPLOY_DIR/opt/db
cp $SRCDIR/db/schema_pg_v3.sql $DEPLOY_DIR/opt/db

# docker deployment
cp $SRCDIR/docker-compose.yml $DEPLOY_DIR/opt/docker/

tar cvzf deployment-code.tar.gz -C $DEPLOY_DIR .
