#!/bin/sh
# preremove — stop the unit before files are removed.
set -e

if command -v systemctl >/dev/null 2>&1; then
    systemctl stop    goat-clientd.service || true
    systemctl disable goat-clientd.service || true
fi

exit 0
