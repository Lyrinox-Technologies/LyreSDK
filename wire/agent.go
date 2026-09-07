package wire

import (
	"bytes"

	"github.com/LyrinoxTechnologies/ridged-proto/rdgproto"
)

// AgentRegisterRequestPayload creates an independent, non-human Lyre
// principal. The resulting API key is returned exactly once in the response.
type AgentRegisterRequestPayload struct {
	Name        string
	Description string
}

func (p *AgentRegisterRequestPayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteString(buf, p.Name)
	rdgproto.WriteString(buf, p.Description)
	return buf.Bytes(), nil
}

func (p *AgentRegisterRequestPayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	var err error
	if p.Name, err = rdgproto.ReadString(r); err != nil {
		return err
	}
	p.Description, err = rdgproto.ReadString(r)
	return err
}

type AgentRegisterResponsePayload struct {
	Success bool
	AgentID string
	APIKey  string
	Message string
}

func (p *AgentRegisterResponsePayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteBool(buf, p.Success)
	rdgproto.WriteString(buf, p.AgentID)
	rdgproto.WriteString(buf, p.APIKey)
	rdgproto.WriteString(buf, p.Message)
	return buf.Bytes(), nil
}

func (p *AgentRegisterResponsePayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	var err error
	if p.Success, err = rdgproto.ReadBool(r); err != nil {
		return err
	}
	if p.AgentID, err = rdgproto.ReadString(r); err != nil {
		return err
	}
	if p.APIKey, err = rdgproto.ReadString(r); err != nil {
		return err
	}
	p.Message, err = rdgproto.ReadString(r)
	return err
}
