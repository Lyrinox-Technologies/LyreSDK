package lyresdk

import "context"

// Caller is the neutral core seam for typed Forge Mods, generated wrappers,
// chains, instrumentation, and application-owned capability bindings.
type Caller interface {
	Call(context.Context, string, any, any) error
}
