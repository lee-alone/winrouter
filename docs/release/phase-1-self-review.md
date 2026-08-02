# Phase 1 Preview Self-Review

> Date: 2026-08-01  
> Candidate: `0.1.0-preview`  
> Scope: implemented `direct-split` MVP on the recorded Windows 11 x64 environment

## Outcome

The implemented Phase 1 scope has no known blocker-level or severe product defect. Configuration,
interface selection, dual-egress routing, DNS, IPv6 blocking, lifecycle, bounded crash recovery, RPC,
and diagnostic redaction have executable or captured evidence in the Phase 1 validation record.

This is an implementation acceptance, not approval for public or commercial distribution. The
candidate remains unsigned, was built without Git metadata, has not been exercised on a second clean
Windows environment. WinRouter source is licensed under GPL-3.0-or-later; independent review of
all conveyed dependencies and the corresponding-source process remains outstanding.

## Security review

| Check | Result | Evidence or limit |
|---|---|---|
| UI runs without elevation | Pass | `helperclient.Launch` rejects an elevated UI and uses UAC only for the helper |
| Named Pipe ACL and session authentication | Pass | SID-scoped pipe, DACL, 256-bit token hash, constant-time comparison, integration tests |
| RPC command and path boundary | Pass | Structured method allowlist; no caller executable, command line, or path input |
| Acceptance-only crash injection excluded | Pass | Normal helper exits with code 2 when the test switch is supplied; dedicated test build is required |
| Core provenance and integrity | Pass for locked artifact | Version 1.13.15 and SHA-256 are checked before use; manifest records upstream tag and revision |
| Owned-resource cleanup | Pass | Fixed state filenames, controlled process Job Object, 100-cycle and crash cleanup evidence |
| Diagnostic redaction | Pass | Sentinel ZIP test, fixed entry list, binary rule-set exclusion, user-initiated export |
| Arbitrary execution/update mechanism | Pass | No general command RPC or updater is present in Phase 1 |
| Installed-file tamper protection | Not complete | Candidate binaries are unsigned and portable; administrator-ACL installation is not validated |

## Code and dependency review

- `tools.cmd check` passed: clean npm install, zero npm audit findings, Vue type check and production
  build, `gofmt`, `go vet`, and all Go tests.
- `go mod verify` passed for every downloaded Go module.
- The build is deterministic in configuration output and records tool versions and dependency trees.
- Go race testing was not executed because the current toolchain has CGO disabled.
- Search of process launch, file mutation, network listeners, and RPC dispatch found only the fixed
  core/helper paths and development experiment utilities documented by the project.

## Package review

Candidate: `build/release/WinRouter-0.1.0-preview-windows-amd64.zip`  
SHA-256: `D7B02EBB319457A9B2534A317B411CB95FB2ACCBEFFE1752B0BCF5E84EF1CE68`

- Contains UI, normal helper, locked core, core GPL text, preview notes, third-party notices, dependency
  inventories, and per-file SHA-256 values.
- Excludes `WinRouter-core-smoke.exe`, diagnostic sentinel tools, captures, and test-only helper builds.
- Build metadata reports `0.1.0-preview`; commit is `unknown` because this workspace has no `.git`.
- UI, helper, and sing-box Authenticode status is `NotSigned`.

## Open release conditions

1. Complete independent review of all conveyed dependencies and the GPL corresponding-source procedure.
2. Build from a Git checkout so the candidate records an immutable commit, then sign the UI, helper,
   and distribution artifacts.
3. Validate install, upgrade, rollback, uninstall, and supported compatibility on a second clean
   Windows environment. Active Hyper-V/WSL/Docker/VPN combinations remain uncovered.

Until these conditions are resolved, the ZIP is a local preview candidate and must not be described
as a public or commercial release.
