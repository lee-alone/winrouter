# Phase 1 Preview Release Notes

## Scope

This preview implements `direct-split` on Windows 11 x64: domestic traffic and DNS use interface A,
other public traffic and global DNS use interface B. IPv6 is explicitly blocked in the MVP. Proxy,
hybrid, custom-rule, process-routing, tray, and startup features are not enabled.

## Installation and upgrade

- This build is an unsigned portable preview, not an installer. Extract it to a directory controlled
  by the user, keep `WinRouter.exe`, `WinRouter-helper.exe`, and `resources/core/` together, and run
  `WinRouter.exe` as a normal user.
- Applying network configuration prompts for UAC because the on-demand helper creates the TUN.
- There is no supported in-place migration contract before configuration schema v1. Stop the app,
  retain `%AppData%\WinRouter\interfaces.json` if desired, and replace the complete preview folder.
- Roll back by stopping WinRouter and restoring the previous complete folder. Do not mix helper or
  core files from different builds.
- Uninstall by stopping WinRouter, confirming `WinRouter-TUN` is absent, and deleting the folder.
  Per-user interface selection remains under `%AppData%\WinRouter`; elevated core state remains under
  `%ProgramData%\WinRouter\state\<SID>` and may be removed manually only after WinRouter is stopped.

## Known limitations

- Validated on one Windows 11 x64 dual-NIC environment. A second Windows environment and active
  Hyper-V, WSL, Docker, and common VPN combinations have not been covered.
- The 24-hour CPU, memory, and packet-loss test was explicitly waived for this preview and must not
  be interpreted as long-term stability evidence.
- Physical adapter disconnect/reconnect and automatic dual-egress reconstruction are deferred to
  Phase 2. An interface change is reported to the UI but does not yet automatically stop the core.
- Real WebView2 appearance at 125% and 150% Windows scaling has not been captured.
- Local builds without Git metadata report commit `unknown`; such builds are not traceable release
  artifacts. The current binaries are unsigned.

## Diagnostics and stop conditions

Logs are bounded locally. Diagnostic bundles are exported only by explicit user action, redact known
credentials and UUIDs, and exclude complete rule-set binaries. Include the app version, error
correlation ID, occurrence time, and reproduction frequency with a report.

Stop distributing this preview if traffic uses the wrong interface, TUN/routes cannot be cleaned,
the core restarts without a bound, or crashes persist. These are not acceptable preview limitations.

## Verification

The Phase 1 evidence and accepted risks are recorded in `docs/testing/phase-1-验证记录.md`.
This preview is not approved for public or commercial distribution until signing, clean-machine
installation/upgrade/uninstall checks, independent dependency-license review, and the GPL corresponding-source
procedure are complete. WinRouter itself is licensed under GPL-3.0-or-later.
