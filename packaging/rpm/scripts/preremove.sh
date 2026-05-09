#!/bin/sh
# RPM %preun — $1 = 0 on uninstall, 1 on upgrade.
# Stop+disable on uninstall; leave running for upgrade (postinstall
# restarts it after the new files land).
set -e

if [ "$1" -eq 0 ] && command -v systemctl >/dev/null 2>&1; then
    systemctl stop    goat-clientd.service || true
    systemctl disable goat-clientd.service || true
fi

exit 0
