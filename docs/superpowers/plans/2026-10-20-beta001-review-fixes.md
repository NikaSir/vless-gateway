# Beta001 independent review fixes

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans. Steps use checkbox syntax for tracking.

**Goal:** Repair the five confirmed review defects before merging Gateway #5 and HA #26.

**Architecture:** Keep API schema 1 and the existing read-only HA design. Fix authoritative state at the gateway, recover pending rule transactions before startup, and prevent persisted selector state from bypassing cold-start DIRECT. Preserve confirmed selection when only gatewayd restarts.

**Tech Stack:** Go 1.27.2, FriendlyWrt 24.10.2, sing-box 1.12.22, HA Python 3.14.

**Spec:** `docs/ACCEPTANCE.md`, `docs/API.md`, and the five independent-review findings reproduced in `/tmp/gateway-independent-review` and `/tmp/ha-contract-review-gateway`.

## Global Constraints

- Candidate remains `1.0.0-beta001`, API schema `1`; no release tag or deployment.
- No NanoPi actions or live HA configuration changes.
- Work from Gateway `81f64541eb186d761adab7d5b4d48fd3481f1c1b` and HA `1701785bb659ce8104d50d7e1a64ff7404995250` in isolated clones; leave beta002 working copies untouched.
- Failover defaults remain two bootstrap successes, three failure cycles, eight recovery successes; configured thresholds must work too.
- Controller-only restart retains confirmed selection while qualifying observations; a dead selected route must not be retained indefinitely.
- Actual route and last route change require selector acknowledgement. Unknown observations remain unavailable.
- HTTP API remains GET-only; secrets stay outside HA browser data and diagnostics.

## Review Focus

- Active DE fails while NL alternates successes/failures: new sessions must fall back to DIRECT at the selected-path threshold.
- Interrupted rule publication or failed rollback: boot must recover previous disk state, and unresolved recovery must retain its journal.
- Restart sing-box with an old selection cached: startup must begin DIRECT before new probe qualification.
- Both VLESS paths fail or selection is unobservable: tunnel telemetry must not claim connectivity from API success alone.
- Rejected mode changes and later polls: confirmed last-change metadata and history must remain factual.

### Task 1: Repair gateway recovery and authoritative telemetry, with paired HA contract checks

**Files:**
- Modify: `internal/controller/service.go`, its tests, `internal/failover/machine.go` only if needed for configured readiness.
- Modify: `internal/rules/updater.go`, its tests, `internal/app/gatewayctl.go`, `openwrt/etc/init.d/nikas-sing-box`, startup lifecycle tests.
- Modify: `internal/singbox/render.go` and its tests to prevent selector persistence from overriding cold-start DIRECT.
- Modify: paired HA fixture/tests under `/workspace/scratch/8a9a8c47e100/fix/ha/tests` as needed to test corrected gateway observations through existing roles and panel.
- Update: `docs/IMPLEMENTATION_REVIEW.md` and relevant operational docs to describe final fixes and validation without claiming device acceptance.

**Interfaces:**
- Consumes existing `PollOnce`, `SetMode`, `Snapshot`, root recovery commands, `rules.Updater`, API schema 1 and HA `role_value`.
- Produces safe lifecycle behavior and factual observations without changing public schema or adding HTTP mutation routes.

- [x] Add regression tests for the five findings and observe expected failures before product edits. Convert retained reproductions to assertions of desired behavior; cover both DE/NL selection and non-default failure thresholds where relevant.
- [x] Limit bootstrap preservation to the appropriate initial phase, honoring the configured selected-path failure threshold. Derive tunnel status from confirmed VLESS selection and measured route health. Keep confirmed LastChange separate from desired transitions during polling, mode change, and restore.
- [x] Add an explicit disk-only rule recovery operation used by normal sing-box startup before config validation. Live in-transaction restart must keep the candidate until acknowledgement. Failed rollback switching/reload must retain its journal and report failure rather than claim restored success.
- [x] Disable unnecessary persistent selector cache or otherwise guarantee cold-start DIRECT while preserving gatewayd-only restart behavior. Add a regression tied to generated configuration and pinned selector semantics.
- [x] Add paired HA regression checks using factual corrected snapshots for DIRECT/tunnel failure and confirmed last-change behavior; synchronize shared fixtures if changed.
- [x] Run focused tests after each fix, then `make verify`, JSON contracts, shell lifecycle/preflight/backup/installer, pinned sing-box validation, HA unittest/pytest and deterministic frontend checks. Root-only socket/network-namespace checks may rely on required GitHub CI if locally denied; report explicit skips.
- [x] Commit coherent gateway and HA changes locally, update review evidence, and write a report with red/green commands, test counts, limitations and commits. Do not push, merge, post comments or create releases; controller handles publication after review.
