#!/bin/sh
set -eu
# Railway mounts the persistent volume at runtime, after image ownership is set.
# Drop privileges before starting the only long-running process.
if [ "$(id -u)" = "0" ]; then
  mkdir -p /data/uploads
  chown app:app /data /data/uploads
  exec gosu app "$@"
fi
exec "$@"
