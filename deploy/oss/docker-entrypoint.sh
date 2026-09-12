#!/bin/sh
# Named volumes mount as root-owned empty dirs; nonroot cannot create llms.db
# (SQLite CANTOPEN / "out of memory (14)"). Fix ownership when we start as root,
# then drop to nonroot before exec.
set -eu

mkdir -p /data/secrets

if [ "$(id -u)" = "0" ]; then
	chown -R nonroot:nonroot /data
	exec su-exec nonroot:nonroot /llms-gateway "$@"
fi

exec /llms-gateway "$@"
