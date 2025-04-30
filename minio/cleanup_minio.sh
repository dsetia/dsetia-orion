#!/bin/bash

mc alias set myminio http://localhost:9000 minioadmin minioadmin

read -p "❓ Proceed with this configuration? [y/N] " confirm
confirm=${confirm,,}  # to lowercase

if [[ "$confirm" != "y" && "$confirm" != "yes" ]]; then
    echo "❌ Aborting."
    exit 1
fi

# delete bucket and objects
mc rb --force myminio/software
mc rb --force myminio/rules
mc rb --force myminio/threatintel

