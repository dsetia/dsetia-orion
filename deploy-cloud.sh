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

DEPLOY_DIR="/tmp/deploy"

BINDIR=$HOME/go/bin
SRCDIR=$HOME/code/orion

# cleanup old stuff
rm -rf $DEPLOY_DIR

mkdir -p $DEPLOY_DIR/opt/db
mkdir -p $DEPLOY_DIR/opt/bin
mkdir -p $DEPLOY_DIR/opt/config
mkdir -p $DEPLOY_DIR/opt/docker
mkdir -p $DEPLOY_DIR/etc/supervisord.d/

# binaries and utils
cp $BINDIR/* $DEPLOY_DIR/opt/bin/
cp $SRCDIR/utils/* $DEPLOY_DIR/opt/bin/

# config
cp -r $SRCDIR/config/  $DEPLOY_DIR/opt/
cp $SRCDIR/db/schema_pg.sql $DEPLOY_DIR/opt/db

cp $SRCDIR/docker-compose.yml $DEPLOY_DIR/opt/docker/

tar cvzf deployment.tar.gz -C $DEPLOY_DIR .
