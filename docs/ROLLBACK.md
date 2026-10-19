# Rollback and physical bypass

## Software DIRECT

Verify management/console access and inspect gatewayctl status before stopping interception:

```sh
/etc/init.d/nikas-sing-box stop
```

Expected: reserved nft/rules/routes removed, kernel IPv4 forwarding active, dnsmasq available through Keenetic fallback. Verify a new TCP connection and DNS. Existing proxy sessions may reconnect. If DIRECT fails, check Keenetic's transit return route and use physical bypass. Before restarting, validate configuration; restore if it fails.

## Restore

Use the exact timestamped archive printed by the installer. Verify root ownership, 0600 mode and adjacent SHA-256 record; retain another local copy. Backups contain credentials and must never be uploaded unencrypted.

```sh
gatewayctl restore --backup /etc/nikas-vless/backups/SELECT_VERIFIED_ARCHIVE.tar.gz --yes
```

Expected: checksum/metadata/path validation before archive writes; previous files restored and policy returned to DIRECT. Verify SSH, transit addressing, DNS and new internet sessions. Re-enable policy only after renewed validation/acceptance. If restore cannot finish, bypass and use the console.

Restore uses individually atomic file replacements, not a filesystem-wide transaction. Power loss during restore requires repeating it from the console; keep bypass available until verification.

## NanoPi power loss

Software cannot bridge an unpowered NanoPi. Identify labeled cables and the original UXG-Max WAN profile before disconnecting anything. Restore that profile's pre-NanoPi addressing/DNS, then connect Keenetic directly to the selected UXG-Max WAN using the bypass cable.

Expected: UXG-Max uses its original Keenetic-side WAN settings, and new internet/DNS sessions work on TverLine and LTE. If bypass fails, verify the original profile, cable labels, selected WAN port and provider status locally. Do not add temporary NanoPi masquerade to hide a missing return route. Reinsert NanoPi only after bench acceptance.
