#!/bin/sh
# postinstall — runs after files are unpacked. Reload systemd, enable
# + start the unit on fresh installs, restart on upgrades.
#
# Patterned after netbird's release_files/post_install.sh, adapted for
# our systemd-only deploy (no init/upstart fallback — Debian 11+ /
# Ubuntu 20.04+ ship systemd).
set -e

action="$1"
case "$action" in
    configure)
        if [ -z "$2" ]; then
            action="install"
        else
            action="upgrade"
        fi
        ;;
esac

if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload || true
    case "$action" in
        install)
            systemctl enable goat-clientd.service || true
            systemctl start  goat-clientd.service || true
            ;;
        upgrade)
            systemctl restart goat-clientd.service || true
            ;;
    esac
else
    echo "goat-client: systemctl not found — skipping service enable. Start the daemon manually." >&2
fi

if command -v update-desktop-database >/dev/null 2>&1; then
    update-desktop-database -q /usr/share/applications || true
fi
if command -v gtk-update-icon-cache >/dev/null 2>&1; then
    gtk-update-icon-cache -q -t /usr/share/icons/hicolor || true
fi

exit 0
