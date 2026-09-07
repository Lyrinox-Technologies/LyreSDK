package wire

import (
	"bytes"
	"github.com/LyrinoxTechnologies/ridged-proto/rdgproto"
)

type ResourceAccessRequestPayload struct{ Action, ServiceID, UserID, Role string }

func (p *ResourceAccessRequestPayload) Marshal() ([]byte, error) {
	b := new(bytes.Buffer)
	rdgproto.WriteString(b, p.Action)
	rdgproto.WriteString(b, p.ServiceID)
	rdgproto.WriteString(b, p.UserID)
	rdgproto.WriteString(b, p.Role)
	return b.Bytes(), nil
}
func (p *ResourceAccessRequestPayload) Unmarshal(d []byte) error {
	r := bytes.NewReader(d)
	var e error
	if p.Action, e = rdgproto.ReadString(r); e != nil {
		return e
	}
	if p.ServiceID, e = rdgproto.ReadString(r); e != nil {
		return e
	}
	if p.UserID, e = rdgproto.ReadString(r); e != nil {
		return e
	}
	p.Role, e = rdgproto.ReadString(r)
	return e
}

type ResourceAccessResponsePayload struct {
	Success bool
	Message string
}

func (p *ResourceAccessResponsePayload) Marshal() ([]byte, error) {
	b := new(bytes.Buffer)
	rdgproto.WriteBool(b, p.Success)
	rdgproto.WriteString(b, p.Message)
	return b.Bytes(), nil
}
func (p *ResourceAccessResponsePayload) Unmarshal(d []byte) error {
	r := bytes.NewReader(d)
	var e error
	p.Success, e = rdgproto.ReadBool(r)
	if e != nil {
		return e
	}
	p.Message, e = rdgproto.ReadString(r)
	return e
}
