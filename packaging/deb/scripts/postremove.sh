#!/bin/sh
# postremove — final cleanup. On `purge`, remove the system user and
# any operator state that wasn't a conffile. On `remove`, leave state
# in place so re-install picks up where the operator left off.
set -e

action="$1"

if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload || true
fi

if command -v update-desktop-database >/dev/null 2>&1; then
    update-desktop-database -q /usr/share/applications || true
fi

case "$action" in
    purge)
        if getent passwd goat-client >/dev/null 2>&1; then
            deluser --quiet --system goat-client || true
        fi
        if getent group goat-client >/dev/null 2>&1; then
            delgroup --quiet --system goat-client || true
        fi
        rm -rf /var/lib/goat-client /var/log/goat-client /var/cache/goat-client
        ;;
esac

exit 0
