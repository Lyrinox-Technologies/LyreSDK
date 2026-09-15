package wire

import "github.com/Lyrinox-Technologies/ridged-proto/rdgproto"

// NewRegistry isolates Lyre payload types from application RDGProto registries.
func NewRegistry() *rdgproto.PayloadRegistry {
	r := rdgproto.NewPayloadRegistry()
	RegisterPayloadTypes(r)
	RegisterServicePayloadTypes(r)
	return r
}
