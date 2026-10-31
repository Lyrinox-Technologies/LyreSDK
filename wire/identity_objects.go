package wire

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/Lyrinox-Technologies/ridged-proto/rdgproto"
	"time"
)

type IdentityServer struct {
	MachineID string `json:"machineId"`
	Platform  string `json:"platform,omitempty"`
}
type IdentityObject struct {
	ID         string          `json:"id"`
	Owner      string          `json:"owner"`
	Service    string          `json:"service"`
	Kind       string          `json:"kind"`
	ExternalID string          `json:"externalId"`
	Name       string          `json:"name"`
	Metadata   map[string]any  `json:"metadata"`
	Server     *IdentityServer `json:"server,omitempty"`
	Revoked    bool            `json:"revoked"`
	Updated    time.Time       `json:"updated"`
}
type IdentityObjectRequestPayload struct {
	Action     string          `json:"action"`
	Owner      string          `json:"owner,omitempty"`
	Service    string          `json:"service"`
	Kind       string          `json:"kind,omitempty"`
	ID         string          `json:"id,omitempty"`
	ExternalID string          `json:"externalId,omitempty"`
	Name       string          `json:"name,omitempty"`
	Metadata   map[string]any  `json:"metadata,omitempty"`
	Server     *IdentityServer `json:"server,omitempty"`
	After      string          `json:"after,omitempty"`
}

// These are reserved native Lyre control-plane messages, not capabilities.
const MsgTypeIdentityObjectRequest byte = 41
const MsgTypeIdentityObjectResponse byte = 42

func (p *IdentityObjectRequestPayload) Marshal() ([]byte, error) {
	var b bytes.Buffer
	for _, s := range []string{p.Action, p.Owner, p.Service, p.Kind, p.ID, p.ExternalID, p.Name, p.After} {
		if e := rdgproto.WriteString(&b, s); e != nil {
			return nil, e
		}
	}
	if e := writeMetadata(&b, p.Metadata); e != nil {
		return nil, e
	}
	if e := writeServer(&b, p.Server); e != nil {
		return nil, e
	}
	if b.Len() > 16384 {
		return nil, errors.New("identity request too large")
	}
	return b.Bytes(), nil
}
func (p *IdentityObjectRequestPayload) Unmarshal(data []byte) error {
	if len(data) > 16384 {
		return errors.New("identity request too large")
	}
	r := bytes.NewReader(data)
	for _, dst := range []*string{&p.Action, &p.Owner, &p.Service, &p.Kind, &p.ID, &p.ExternalID, &p.Name, &p.After} {
		s, e := readIdentityString(r)
		if e != nil {
			return e
		}
		*dst = s
	}
	var e error
	p.Metadata, e = readMetadata(r)
	if e != nil {
		return e
	}
	p.Server, e = readServer(r)
	if e != nil {
		return e
	}
	if r.Len() != 0 {
		return errors.New("trailing identity request")
	}
	return nil
}

type IdentityObjectResponsePayload struct {
	Success bool
	Message string
	Objects []IdentityObject
}

func (p *IdentityObjectResponsePayload) Marshal() ([]byte, error) {
	if len(p.Objects) > 100 {
		return nil, errors.New("identity page too large")
	}
	var b bytes.Buffer
	_ = rdgproto.WriteBool(&b, p.Success)
	_ = rdgproto.WriteString(&b, p.Message)
	_ = rdgproto.WriteUint32(&b, uint32(len(p.Objects)))
	for _, o := range p.Objects {
		for _, s := range []string{o.ID, o.Owner, o.Service, o.Kind, o.ExternalID, o.Name} {
			_ = rdgproto.WriteString(&b, s)
		}
		if e := writeMetadata(&b, o.Metadata); e != nil {
			return nil, e
		}
		if e := writeServer(&b, o.Server); e != nil {
			return nil, e
		}
		_ = rdgproto.WriteBool(&b, o.Revoked)
		_ = rdgproto.WriteUint64(&b, uint64(o.Updated.Unix()))
	}
	if b.Len() > 512*1024 {
		return nil, errors.New("identity response too large")
	}
	return b.Bytes(), nil
}
func (p *IdentityObjectResponsePayload) Unmarshal(data []byte) error {
	if len(data) > 512*1024 {
		return errors.New("identity response too large")
	}
	r := bytes.NewReader(data)
	var e error
	p.Success, e = rdgproto.ReadBool(r)
	if e != nil {
		return e
	}
	p.Message, e = readIdentityString(r)
	if e != nil {
		return e
	}
	n, e := rdgproto.ReadUint32(r)
	if e != nil {
		return e
	}
	if n > 100 {
		return errors.New("identity page too large")
	}
	p.Objects = make([]IdentityObject, 0, n)
	for i := uint32(0); i < n; i++ {
		var o IdentityObject
		for _, dst := range []*string{&o.ID, &o.Owner, &o.Service, &o.Kind, &o.ExternalID, &o.Name} {
			s, e := readIdentityString(r)
			if e != nil {
				return e
			}
			*dst = s
		}
		o.Metadata, e = readMetadata(r)
		if e != nil {
			return e
		}
		o.Server, e = readServer(r)
		if e != nil {
			return e
		}
		o.Revoked, e = rdgproto.ReadBool(r)
		if e != nil {
			return e
		}
		stamp, e := rdgproto.ReadUint64(r)
		if e != nil {
			return e
		}
		o.Updated = time.Unix(int64(stamp), 0).UTC()
		p.Objects = append(p.Objects, o)
	}
	if r.Len() != 0 {
		return errors.New("trailing identity response")
	}
	return nil
}

// Only arbitrary application metadata uses JSON; the control fields and server
// bindings above are typed RDGProto fields and never generic JSON RPC envelopes.
func writeMetadata(b *bytes.Buffer, m map[string]any) error {
	data, e := json.Marshal(m)
	if e != nil {
		return e
	}
	if len(data) > 4096 {
		return errors.New("metadata too large")
	}
	return rdgproto.WriteBytes(b, data)
}
func readMetadata(r *bytes.Reader) (map[string]any, error) {
	if e := identityLength(r, 4096); e != nil {
		return nil, e
	}
	data, e := rdgproto.ReadBytes(r)
	if e != nil {
		return nil, e
	}
	if len(data) > 4096 {
		return nil, errors.New("metadata too large")
	}
	var m map[string]any
	e = json.Unmarshal(data, &m)
	return m, e
}
func writeServer(b *bytes.Buffer, s *IdentityServer) error {
	_ = rdgproto.WriteBool(b, s != nil)
	if s == nil {
		return nil
	}
	_ = rdgproto.WriteString(b, s.MachineID)
	return rdgproto.WriteString(b, s.Platform)
}
func readServer(r *bytes.Reader) (*IdentityServer, error) {
	has, e := rdgproto.ReadBool(r)
	if e != nil {
		return nil, e
	}
	if !has {
		return nil, nil
	}
	s := &IdentityServer{}
	s.MachineID, e = readIdentityString(r)
	if e != nil {
		return nil, e
	}
	s.Platform, e = readIdentityString(r)
	return s, e
}

// Bound native length-prefixed values before RDGProto allocates their buffers.
func identityLength(r *bytes.Reader, max uint64) error {
	peek := *r
	n, e := rdgproto.ReadVarint(&peek)
	if e != nil {
		return e
	}
	if n > max || n > uint64(peek.Len()) {
		return errors.New("identity field length invalid")
	}
	return nil
}
func readIdentityString(r *bytes.Reader) (string, error) {
	if e := identityLength(r, 4096); e != nil {
		return "", e
	}
	return rdgproto.ReadString(r)
}
