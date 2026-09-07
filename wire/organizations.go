package wire

import (
	"bytes"
	"encoding/json"
	"github.com/LyrinoxTechnologies/ridged-proto/rdgproto"
)

type OrganizationRequestPayload struct {
	Action         string `json:"action"`
	OrganizationID string `json:"organization_id"`
	UserID         string `json:"user_id"`
	Role           string `json:"role"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Email          string `json:"email"`
}

func (p *OrganizationRequestPayload) Marshal() ([]byte, error) {
	b, _ := json.Marshal(p)
	var out bytes.Buffer
	_ = rdgproto.WriteString(&out, string(b))
	return out.Bytes(), nil
}
func (p *OrganizationRequestPayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	raw, err := rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(raw), p)
}

type OrganizationResponsePayload struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    string `json:"data"`
}

func (p *OrganizationResponsePayload) Marshal() ([]byte, error) {
	b, _ := json.Marshal(p)
	var out bytes.Buffer
	_ = rdgproto.WriteString(&out, string(b))
	return out.Bytes(), nil
}
func (p *OrganizationResponsePayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	raw, err := rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(raw), p)
}
