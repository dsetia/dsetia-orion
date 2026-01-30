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

make all

# binaries and utils
cp $BINDIR/* $DEPLOY_DIR/opt/bin/
cp $SRCDIR/utils/* $DEPLOY_DIR/opt/bin/
cp $SRCDIR/db/migrate_v1_to_v2.sh $DEPLOY_DIR/opt/bin/
cp $SRCDIR/db/move-tenant.sh $DEPLOY_DIR/opt/bin/
cp $SRCDIR/db/tenant-info.sh $DEPLOY_DIR/opt/bin/

# schema
cp $SRCDIR/db/schema_pg.sql $DEPLOY_DIR/opt/db
cp $SRCDIR/db/schema_pg_v2.sql $DEPLOY_DIR/opt/db

tar cvzf deployment-code.tar.gz -C $DEPLOY_DIR .
