#!/bin/sh
# RPM %pre — runs with $1 = 1 on install, $1 = 2 on upgrade.
# Create system user/group on fresh install; no-op on upgrade.
set -e

if [ "$1" -eq 1 ]; then
    if ! getent group goat-client >/dev/null 2>&1; then
        groupadd --system goat-client
    fi
    if ! getent passwd goat-client >/dev/null 2>&1; then
        useradd --system \
            --gid goat-client \
            --home-dir /var/lib/goat-client \
            --no-create-home \
            --shell /sbin/nologin \
            --comment "goat-client tunnel daemon" \
            goat-client
    fi
fi

exit 0
