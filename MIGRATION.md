# Migration to LyreSDK — 2026-09-07

All nine active first-party Go consumers now require
`github.com/Lyrinox-Technologies/LyreSDK`, with a relative local replacement until
the SDK receives a published version:

- Linux/Lyrinox-Client-Daemon
- Linux/lyrinox-command-service
- Linux/lyrinox-dashboard-service
- Linux/lyrinox-metrics-service
- Linux/lyrinox-package-service
- Server/lyre-cli
- Server/lyre-test-service
- Server/lyrinox-lyre-services (shared web identity bridge)
- Lyre-Services/lyrinox-lyre-provider (including examples and tests)

The old `Libraries/liblyre` and `Libraries/liblyre-svc` repositories remain as
retired historical checkouts. No active consumer imports them. Stale duplicate
checkouts under `Projects/Lyrinox-Technologies` are not authoritative. Lyre Server
itself continues to use raw RDGProto, as intended. Source-License SDKs and unrelated
native library generators are separate products and were not replaced.

## API changes

| Previous API | LyreSDK |
| --- | --- |
| user `ConnectWithConfig` | `Dial(ctx, address, Config{...})` |
| provider `New(Config{...})` | `NewService(ServiceConfig{...})` |
| separate client/provider request types | one Client, Service, Request, Response and wire package |
| direct endpoint calls only in user API | typed `Call`, `CallResponse`, plus private `CallService` |
| external hardware fingerprinting | explicit device identities supplied by the application |
| untyped JSON numbers as float64 | json.Number (or decode into application structs) |
| SDK-managed credential/session files | application-owned storage via Token and Authenticate |
| embedded administrative password hashing | server provisioning tools own hashing |

Capability callers must use canonical `lyre.family[@provider.implementation][@vN]`
references. CLI `-call` accepts those references or private `service/endpoint`.
Machine mode requires paired `-device-id` and `-ephemeral-device-id` flags (or CLI
environment variables LYRE_DEVICE_ID/LYRE_EPHEMERAL_DEVICE_ID). The daemon's
provisioning command writes `device_id` and `ephemeral_device_id`. Existing
`machid_salt` configurations fail with migration instructions rather than silently
switching to standard authentication. Existing enrolled IDs may be supplied;
new device identities may require enrollment/MFA according to server policy.

Deployment/build contexts must include the sibling `Libraries/LyreSDK` directory
while using local replacements. Before publishing independent consumers, publish
and tag LyreSDK, replace v0.0.0 with that version, and remove the local replacement.
This migration does not publish a module or deploy binaries to production.

## Completed local verification

- SDK: race-enabled suite passed 30 consecutive runs; go vet passed.
- All nine migrated consumers: go test -race ./... and go vet ./... passed.
- Official provider: real cache handlers tested through SDK/raw RDGProto.
- Lyre Server: capability parser tests passed, including a provider beginning with v.
- Forge: 10 tests, 34 assertions passed; site Ruby syntax passed.
- SDK cross-builds: linux/amd64, linux/arm64, darwin/arm64, windows/amd64.
- Module/import audit: SDK has only RDGProto outside the standard library;
  remaining old SDK module references are retired checkouts and Go module caches.
