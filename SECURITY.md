# Security policy

goat-client is the cross-platform daemon + GUI for the **wg-cp0 silent
control plane** in the goat data substrate. It handles cryptographic
material at the device boundary — bundle parsing, Ed25519 signature
verification, WireGuard private-key custody at install scope, IPC
auth, and OS-level VPN integration on five platforms. Security reports
are taken seriously and prioritized over feature work.

## How to report a vulnerability

**Do not open a public GitHub issue.** Public reports tip off
adversaries before a fix is available.

Use one of the following channels, in order of preference:

1. **GitHub Private Vulnerability Reporting** (preferred). On this
   repo: *Security* tab → *Report a vulnerability*. This creates a
   private advisory visible only to maintainers.
   ([GitHub docs](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability))
2. **Project mailbox** *(queued)*. A dedicated security mailbox
   (`security@dlf-dds.io` or equivalent under the project's eventual
   org domain) will be published here once provisioned. Until then,
   GitHub Private Vulnerability Reporting is the stable channel.

Please include:

- A description of the issue and its impact.
- Reproduction steps or a proof-of-concept.
- Affected platforms (Linux / macOS / Windows / iOS / Android), git
  commit or release tag, and any relevant build flags.
- Your suggested mitigation, if any.
- Whether you wish to be credited in the eventual advisory.

## What to expect

| Stage | Target |
|---|---|
| Acknowledgement of receipt | within 3 business days |
| Initial triage + severity assessment | within 7 business days |
| Patch development + advisory drafting | depends on severity; we will keep you informed |
| Coordinated disclosure | by mutual agreement |

We follow a coordinated-disclosure model: a fix is prepared and
released, then a public advisory is published. We will credit reporters
by name unless they prefer anonymity.

## Scope

In scope:

- This repository's source code, build configuration, and CI workflows.
- The `goat-clientd` daemon and `goat-client` GUI binaries we build and
  publish from this repository (Linux, macOS, Windows; iOS and Android
  shells once Tracks C/D land — see [`HANDOFF.md`](HANDOFF.md)).
- The CBOR bundle parser in `internal/bundle/` (Ed25519 signature
  verification against the pinned offline-CA root).
- The IPC surface in `internal/ipc/` (JSON-RPC over Unix socket / named
  pipe; local-uid auth on writes).
- The single-peer wg-cp0 tunnel manager in `internal/tunnel/`.
- Default configurations shipped by per-platform packaging
  (`packaging/{deb,rpm,dmg,msi}/`).

Out of scope:

- The upstream netbird codebase we forked from
  (`netbirdio/netbird` at `3fc5a8d4a1fe308ff1068764a09b90b0859ab8fe`).
  Report netbird issues to that project. We will accept reports of
  *integration* mistakes on our side that misuse a sound upstream
  primitive — including bugs in our local netbird fork
  (`dfarrel1/netbird` commit `32d04da19`, the embed-CA / ServerName
  patch).
- The wg-cp0 silent control plane *server* side, the offline CA, the
  bundle issuance pipeline, and the management-plane components — those
  live in goat-trunk
  ([`dlf-dds/DesertBreadBird`](https://github.com/dlf-dds/DesertBreadBird))
  and have their own [`SECURITY.md`](https://github.com/dlf-dds/DesertBreadBird/blob/main/SECURITY.md).
- Application-layer systems that ride on the mesh.
- Deployments operated by other organizations using this code. Report
  those to that operator.
- Issues in the underlying OS VPN frameworks (Linux WireGuard kernel
  module, macOS NetworkExtension, Windows Wintun, Apple
  NEPacketTunnelProvider, Android VpnService). Report those to the
  upstream maintainers.

## Hardening posture

This repo's posture inherits from goat-trunk's substrate posture. For
context on accepted risks, the FIPS-PQC boundary, and the four standing
waivers:

- [`docs/compliance/comparative-posture.md`](https://github.com/dlf-dds/DesertBreadBird/blob/main/docs/compliance/comparative-posture.md)
  — substrate-wide posture comparison.
- [`docs/adr/0855-accepted-risks-register.md`](https://github.com/dlf-dds/DesertBreadBird/blob/main/docs/adr/0855-accepted-risks-register.md)
  — explicitly accepted risks with rationale.
- [`docs/adr/0840-goat-client-cross-platform-daemon-gui.md`](https://github.com/dlf-dds/DesertBreadBird/blob/main/docs/adr/0840-goat-client-cross-platform-daemon-gui.md)
  — this client's design + threat model.

Releases are cosign-signed (Track E). To verify:

```bash
cosign verify-blob \
  --certificate goat-clientd-linux-amd64.cert \
  --signature   goat-clientd-linux-amd64.sig \
  --certificate-identity-regexp 'https://github\.com/dlf-dds/goat-client/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  goat-clientd-linux-amd64
```

## Safe-harbor

We will not pursue or support legal action against researchers who:

- Make a good-faith effort to comply with this policy.
- Avoid privacy violations, destruction of data, and degradation of
  service.
- Limit testing to systems they own or have explicit permission to
  test (do **not** test against any deployment operated by a third
  party — including any government deployment that uses this code).
- Provide us a reasonable window to address the issue before disclosing
  publicly.

---

*See [`CONTRIBUTING.md`](CONTRIBUTING.md) for the contributor workflow
and goat-trunk's
[`docs/adr/0109-git-collaboration-trunk-based.md`](https://github.com/dlf-dds/DesertBreadBird/blob/main/docs/adr/0109-git-collaboration-trunk-based.md)
for the change-control policy this project enforces on its own code.*
