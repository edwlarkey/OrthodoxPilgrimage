#!/bin/sh
set -e

# Restore the database if it does not already exist.
if [ -f /data/orthodox_pilgrimage.db ]; then
  echo "Database already exists, skipping restore"
else
  echo "No database found, restoring from replica if exists"
  /usr/local/bin/litestream restore -if-replica-exists /data/orthodox_pilgrimage.db
fi

# Run litestream with app as the subprocess.
exec /usr/local/bin/litestream replicate -exec "/orthodoxpilgrimage -db-path=/data/orthodox_pilgrimage.db"
