#!/bin/sh
# RPM %post — $1 = 1 on install, 2 on upgrade.
set -e

if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload || true
    if [ "$1" -eq 1 ]; then
        systemctl enable goat-clientd.service || true
        systemctl start  goat-clientd.service || true
    else
        systemctl restart goat-clientd.service || true
    fi
fi

if command -v update-desktop-database >/dev/null 2>&1; then
    update-desktop-database -q /usr/share/applications || true
fi
if command -v gtk-update-icon-cache >/dev/null 2>&1; then
    gtk-update-icon-cache -q -t /usr/share/icons/hicolor || true
fi

exit 0
