# WinRouter

[![License: GPL v3 or later](https://img.shields.io/badge/License-GPL_v3_or_later-blue.svg)](LICENSE)

WinRouter has completed the implemented Phase 1 `direct-split` MVP and is preparing an unsigned,
small-scope preview candidate. It is not yet approved for public or commercial distribution. See
`docs/release/phase-1-preview.md` for limitations and release boundaries.

Run formatting, static analysis, tests, and the desktop build with one command:

```powershell
.\tools.cmd
```

## Core prerequisite

The repository does not track third-party core binaries. Before running core validation or a desktop build,
place the Windows AMD64 artifact for sing-box at `resources/core/sing-box.exe`. Keep any accompanying runtime DLLs
in the same directory. The upstream license remains part of this repository.

## License

WinRouter is licensed under GPL-3.0-or-later. See `LICENSE`. Third-party components retain their own
licenses; principal runtime notices are listed in `docs/release/THIRD-PARTY-NOTICES.md`.

Build the frontend, desktop backend, privileged helper, smoke tool, and bundled core resources without running the test suite:

```powershell
.\build.bat
.\build.bat 0.1.0-preview
```

The frontend is compiled and embedded into `build/bin/WinRouter.exe`. The same command also produces
`WinRouter-helper.exe`, `WinRouter-core-smoke.exe`, and `build/bin/resources/core/`.

The individual checks and build remain available with:

```powershell
.\tools.cmd check
.\tools.cmd build
```

Run the interface inventory probe from a non-elevated terminal:

```powershell
go run ./cmd/winrouter-probe
```

The command emits JSON containing stable interface identifiers, addresses,
prefix lengths, gateways, DNS servers, metrics, status, and an initial
physical/virtual classification. No network configuration is changed.

The `internal/interfaces` package also provides `IdentityFromAdapter` and
`Resolve`. Resolution always prefers the Interface GUID; MAC address, IPv4
prefixes, and friendly name are auxiliary evidence only. An ambiguous match
returns `ErrAmbiguousMatch` with the candidates instead of selecting one.
Weak evidence such as a friendly name alone returns `ErrSelectionNeeded` and
requires explicit confirmation.

Generate direct-prefix mappings and overlap diagnostics with:

```powershell
go run ./cmd/winrouter-probe --topology
```

Select a conflict-free TUN prefix after checking both interfaces and the full
Windows route table:

```powershell
go run ./cmd/winrouter-probe --tun-prefix --preferred 172.19.0.0/30
```

Generate or validate the minimal configuration against the locked core:

```powershell
go run ./cmd/winrouter-probe --tun-config --stack system
go run ./cmd/winrouter-probe --check-tun --stack system
```

The elevated lifecycle experiment is bounded and verifies cleanup:

```powershell
.\bin\winrouter-probe.exe --run-tun --stack system
```
