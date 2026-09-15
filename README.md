# LyreSDK

The default Go core SDK for Lyre-native software. This module owns protocol,
transport, authentication, generic capability invocation, service/provider
hosting, and the small lifecycle primitives needed to connect to The Lyre. It
depends only on **Go 1.25+ and RDGProto v1.0.0**.

Typed domain APIs such as charts and cache live in first-party Forge Mods. Forge
combines LyreSDK core with selected Mods when it builds a customized runtime.

```go
import lyresdk "github.com/Lyrinox-Technologies/LyreSDK"
```

This is a local development module, not a published release. From a sibling
first-party application, use:

```go
require github.com/Lyrinox-Technologies/LyreSDK v0.0.0
replace github.com/Lyrinox-Technologies/LyreSDK => ../../Libraries/LyreSDK
```

## Call capabilities

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
client, err := lyresdk.Dial(ctx, os.Getenv("LYRE_URL"), lyresdk.Config{})
if err != nil { return err }
defer client.Close()
if err := client.LoginAgent(ctx, os.Getenv("LYRE_AGENT_KEY")); err != nil { return err }

var result struct { UUIDs []string `json:"uuids"` }
if err := client.Call(ctx, "lyre.uuid.generate@v1",
    map[string]any{"version": "v4", "count": 1}, &result); err != nil { return err }
```

`Call` accepts a JSON object (Go structs or maps) and decodes into your type.
`CallResponse` exposes success, the raw JSON payload, and provider error text.
Untyped numeric values use `json.Number` to preserve precision. `Call` returns
`*CapabilityError` for a provider failure and `*ProtocolError` for a Lyre error;
use `errors.As` to inspect them. Canonical references are
`lyre.family[@provider.implementation][@vN]`. Private endpoints remain accessible
with `CallService`.

## Authenticate and host services

`Authenticate(ctx, wire.AuthRequestPayload{...}, mfaCallback)` supports standard
users, explicit machine identities, token resumption, and MFA. `Login` and
`LoginWithToken` are conveniences. Device authentication requires both
`Config.DeviceID` and `Config.EphemeralDeviceID`; the application generates and
stores these. The SDK does not fingerprint hardware, automatically persist
credentials, or choose MFA policy. Tokens are available through `Token()` for
application-owned session storage. Use a fresh client when changing identity.

```go
service, err := lyresdk.NewService(lyresdk.ServiceConfig{
    ServiceID: "example-service", Secret: os.Getenv("LYRE_SERVICE_SECRET"),
    ServerURL: os.Getenv("LYRE_SERVICE_URL"), Endpoints: []string{"echo"},
})
if err != nil { return err }
service.Handle("echo", func(req *lyresdk.Request) *lyresdk.Response {
    return req.Success(map[string]any{"text": req.Payload["text"]})
})
return service.RunPersistent(ctx)
```

Publish approved capability contracts through `ServiceConfig.Capabilities` and
supply the publisher user/private-key proof required by Lyre. Private endpoints
need no public capability declaration. `ProviderExtension` declarations are
provider-specific capability adapters: they contain bounded schemas and field
maps only, keep the wire JSON name `extensions`, and never execute code. The old
`Extension` and `ExtensionError` names remain deprecated aliases for source
compatibility.

`Request.Principal` exposes the identity forwarded by Lyre. Handlers may call
capabilities while the reader continues receiving responses. Handlers are bounded
(64 by default), and panics become error responses. `RunPersistent` reconnects
with backoff, republishes declarations, sends heartbeats, and stops when its
context is canceled. In-flight calls are never replayed: retrying a mutation may
duplicate its effect.

## Raw RDGProto remains available

There is no additional protocol layered on RDGProto. `NewClient(conn, config)`
accepts any `rdgproto.Connection`, including `net.Conn`. `DialTransport` provides
stdlib `tcp://`, `tls://`, `ws://`, and `wss://` adapters. Current Lyre servers use
`/ws` for users/agents and `/service/ws` for services; TCP/TLS require a server
with a raw stream listener. TLS verifies certificates by default.

To own the protocol yourself, use RDGProto directly and optionally import
`LyreSDK/wire` for payload definitions and `wire.NewRegistry()` for an isolated
payload registry. `Client.Send` and `Client.Request` expose lower-level messages,
including account, organization, permission, agent, and administrative payloads.
Do not start another reader on a connection owned by a Client.

Capability calls correlate independently and support concurrent use. Cancellation
discards late call replies, but it does **not** cancel work already sent to Lyre.
A canceled partial write closes the connection. Uncorrelated control exchanges
are serialized and close on timeout to avoid matching stale responses. A generic
uncorrelated Lyre error fails active requests; the SDK cannot infer which call
caused it. Event callbacks and provider handlers should return promptly; caller
code remains responsible for canceling its own work.

## Forge foundation

`Caller` is the small interface shared by clients and services:

```go
type Caller interface {
    Call(context.Context, string, any, any) error
}
```

Forge Mods use this core seam for typed capability bindings, validation, codecs,
and hybrid local/remote behavior. LyreSDK core does not include domain-specific
chart or cache helpers; those live under Forge Mods and are selected by Forge
project manifests.

## Verification

```sh
go test -race ./...
go vet ./...
go list -m all
```

Tests cover RDGProto authentication/MFA, agent credentials, concurrent
correlation, late responses, control timeouts, nested provider calls, publication,
heartbeats, WebSocket masking/fragmentation/control frames/length boundaries, and
TLS checks. See [MIGRATION.md](MIGRATION.md) for the consumer audit.
