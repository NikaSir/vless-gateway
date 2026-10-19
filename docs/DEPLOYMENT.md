# NanoPi R5S deployment

Supported: NanoPi R5S LTS 4+64 GB, FriendlyWrt 24.10.2, sing-box 1.12.22 with Clash API. Build 1.0.0-beta001 is a bench candidate, not an OS image. Keep local console access and the labeled physical bypass cable available.

## Preparation and MAC inventory

Record the archive SHA-256, binary versions, FriendlyWrt backup, package inventory and current Keenetic/UXG-Max settings. Keep production inputs in a root-owned 0700 preparation directory; secrets are 0600. Never paste credential values into chat, issues or CLI value arguments.

Label both ends of the 2.5 GbE cables UPSTREAM/Keenetic and DOWNSTREAM/UXG-Max WAN. Label the 1 GbE cable MANAGEMENT/UniFi VLAN. Identify each port by link test and MAC; example interface names/MACs are not physical assignments.

Choose a transit subnet distinct from Keenetic, UniFi/VLAN, VPN and management networks. Initial proposal: 192.168.2.0/24, NanoPi .1, UXG-Max WAN .2 with gateway/DNS .1. Reserve NanoPi's upstream DHCP address on Keenetic and add a static return route for transit through it. Management has no default gateway or forwarding path.

Verify administrator SSH key login before applying topology. The generated network/firewall/dhcp files replace this dedicated appliance's topology; inspect every file and retain the current backup.

## Prepare and inspect

Verify the archive checksum against its trusted build record and companion file, then extract into a new private directory. In that directory:

```sh
bin/gatewayctl --version
sing-box version
bin/gatewayctl --config-root /root/nikas-prepared preflight
```

Expected: 1.0.0-beta001, sing-box 1.12.22 with with_clash_api, and preflight validated. Hardware, overlap or reserved-resource errors stop deployment. No live configuration changed; rollback here is to leave the current system in service.

Prepare local files:

```sh
sh scripts/install.sh --config-dir /root/nikas-prepared --secrets-file /root/nikas-prepared/secrets.json --stage direct
```

Expected: a timestamped backup and exact restore command, validated config, and /etc/nikas-vless/network-plan/. Save the printed rollback command locally and inspect the plan. Existing secrets are preserved; independent API/Clash tokens are generated locally.

## Apply inline DIRECT

Compare the plan with MAC inventory, reservation, return route and administrator access. Verify the bypass cable and approved maintenance window before applying:

```sh
sh scripts/install.sh --config-dir /root/nikas-prepared --secrets-file /root/nikas-prepared/secrets.json --stage direct --apply-network
```

Expected: network/firewall/DNS restart, management SSH remains accessible, policy_enabled=0 and no sing-box interception. On failure, run the printed restore command from the console or use ROLLBACK.md physical bypass. Perform the DIRECT tests in ACCEPTANCE.md, including TverLine/LTE and DNS fallback. Retain accepted local evidence before policy.

## Policy and 24-hour soak

After DIRECT evidence is accepted and the policy window approved, verify config and the current rollback command:

```sh
gatewayctl validate
sh scripts/install.sh --config-dir /root/nikas-prepared --secrets-file /root/nikas-prepared/secrets.json --stage policy --apply-network
gatewayctl status
```

Expected: startup DIRECT, then Germany after two successful cycles. Follow DE → NL → DIRECT, recovery and DNS acceptance. For immediate software DIRECT stop /etc/init.d/nikas-sing-box; dnsmasq stays running. Restore or physically bypass if DIRECT fails.

Begin the 24-hour soak only after failover tests pass. Record route changes, unavailable periods, temperature, storage and new-session success. Connect separate ha-vless-gateway read-only after the soak and credential review.
