// Package wire provides service communication payloads for Lyre-Server.
package wire

import (
	"bytes"
	"encoding/json"

	"github.com/LyrinoxTechnologies/ridged-proto/rdgproto"
)

// ServiceAuthPayload is sent by services to authenticate with Lyre-Server.
type ServiceAuthPayload struct {
	ServiceID           string              // Service identifier from config
	Secret              string              // Shared secret for this service
	Endpoints           []string            // Endpoints this service instance provides
	Name                string              // Optional public service name for self-registration
	Type                string              // Optional public service type for self-registration
	Description         string              // Optional public service description for self-registration
	Capabilities        []ServiceCapability // Provider capabilities published by this service
	PublisherUserID     string              // Lyre user asserting publisher ownership
	PublisherPrivateKey string              // Ephemeral RSA private key proof; never persisted
}

type ServiceCapability struct {
	Name                    string                 `json:"capability"`
	Version                 uint32                 `json:"version,omitempty"`
	SupportedVersions       []uint32               `json:"supported_contract_versions,omitempty"`
	ProviderID              string                 `json:"provider_id,omitempty"`
	ProviderCapabilityID    string                 `json:"provider_capability_id,omitempty"`
	Endpoint                string                 `json:"endpoint"`
	InputSchema             string                 `json:"input_schema,omitempty"`
	OutputSchema            string                 `json:"output_schema,omitempty"`
	Priority                int                    `json:"priority,omitempty"`
	ProviderSoftwareVersion string                 `json:"provider_software_version,omitempty"`
	Description             string                 `json:"description,omitempty"`
	Metadata                map[string]interface{} `json:"metadata,omitempty"`
	Extensions              []Extension            `json:"extensions,omitempty"`
}

func (p *ServiceAuthPayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteString(buf, p.ServiceID)
	rdgproto.WriteString(buf, p.Secret)
	// Encode endpoints as JSON array
	endpointsJSON, _ := json.Marshal(p.Endpoints)
	rdgproto.WriteString(buf, string(endpointsJSON))
	rdgproto.WriteString(buf, p.Name)
	rdgproto.WriteString(buf, p.Type)
	rdgproto.WriteString(buf, p.Description)
	capabilitiesJSON, err := json.Marshal(p.Capabilities)
	if err != nil {
		return nil, err
	}
	rdgproto.WriteString(buf, string(capabilitiesJSON))
	rdgproto.WriteString(buf, p.PublisherUserID)
	rdgproto.WriteString(buf, p.PublisherPrivateKey)
	return buf.Bytes(), nil
}

func (p *ServiceAuthPayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	var err error
	p.ServiceID, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.Secret, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	endpointsJSON, err := rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	if endpointsJSON != "" {
		if err := json.Unmarshal([]byte(endpointsJSON), &p.Endpoints); err != nil {
			return err
		}
	}
	if r.Len() == 0 {
		return nil
	}
	p.Name, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.Type, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.Description, err = rdgproto.ReadString(r)
	if err != nil || r.Len() == 0 {
		return err
	}
	capabilitiesJSON, err := rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	if capabilitiesJSON != "" {
		if err := json.Unmarshal([]byte(capabilitiesJSON), &p.Capabilities); err != nil {
			return err
		}
	}
	if r.Len() == 0 {
		return nil
	}
	p.PublisherUserID, err = rdgproto.ReadString(r)
	if err != nil || r.Len() == 0 {
		return err
	}
	p.PublisherPrivateKey, err = rdgproto.ReadString(r)
	return nil
}

// ServiceAuthResponsePayload is sent to services after authentication.
type ServiceAuthResponsePayload struct {
	Success   bool
	ServiceID string
	Message   string
}

func (p *ServiceAuthResponsePayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteBool(buf, p.Success)
	rdgproto.WriteString(buf, p.ServiceID)
	rdgproto.WriteString(buf, p.Message)
	return buf.Bytes(), nil
}

func (p *ServiceAuthResponsePayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	var err error
	p.Success, err = rdgproto.ReadBool(r)
	if err != nil {
		return err
	}
	p.ServiceID, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.Message, err = rdgproto.ReadString(r)
	return err
}

// ServiceHeartbeatPayload is sent periodically to keep connection alive.
type ServiceHeartbeatPayload struct {
	ServiceID string
	Timestamp int64 // Unix timestamp
}

func (p *ServiceHeartbeatPayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteString(buf, p.ServiceID)
	rdgproto.WriteUint64(buf, uint64(p.Timestamp))
	return buf.Bytes(), nil
}

func (p *ServiceHeartbeatPayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	var err error
	p.ServiceID, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	ts, err := rdgproto.ReadUint64(r)
	if err != nil {
		return err
	}
	p.Timestamp = int64(ts)
	return nil
}

// ServiceMessagePayload is an inter-service message routed through Lyre.
type ServiceMessagePayload struct {
	MessageID   string // Unique message ID for tracking
	FromService string // Source service ID
	ToService   string // Target service ID
	Endpoint    string // Endpoint/action on target service
	Payload     []byte // JSON-encoded payload data
	ReplyTo     string // Message ID this is replying to (for responses)
}

func (p *ServiceMessagePayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteString(buf, p.MessageID)
	rdgproto.WriteString(buf, p.FromService)
	rdgproto.WriteString(buf, p.ToService)
	rdgproto.WriteString(buf, p.Endpoint)
	rdgproto.WriteBytes(buf, p.Payload)
	rdgproto.WriteString(buf, p.ReplyTo)
	return buf.Bytes(), nil
}

func (p *ServiceMessagePayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	var err error
	p.MessageID, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.FromService, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.ToService, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.Endpoint, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.Payload, err = rdgproto.ReadBytes(r)
	if err != nil {
		return err
	}
	p.ReplyTo, err = rdgproto.ReadString(r)
	return err
}

// ServiceResponsePayload is a response to a service message.
type ServiceResponsePayload struct {
	MessageID string // Original message ID being responded to
	Success   bool
	Payload   []byte // JSON-encoded response data
	Error     string // Error message if not successful
}

func (p *ServiceResponsePayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteString(buf, p.MessageID)
	rdgproto.WriteBool(buf, p.Success)
	rdgproto.WriteBytes(buf, p.Payload)
	rdgproto.WriteString(buf, p.Error)
	return buf.Bytes(), nil
}

func (p *ServiceResponsePayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	var err error
	p.MessageID, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.Success, err = rdgproto.ReadBool(r)
	if err != nil {
		return err
	}
	p.Payload, err = rdgproto.ReadBytes(r)
	if err != nil {
		return err
	}
	p.Error, err = rdgproto.ReadString(r)
	return err
}

// ServiceListPayload requests a list of available services.
type ServiceListPayload struct {
	TypeFilter string // Optional: filter by service type
}

func (p *ServiceListPayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteString(buf, p.TypeFilter)
	return buf.Bytes(), nil
}

func (p *ServiceListPayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	var err error
	p.TypeFilter, err = rdgproto.ReadString(r)
	return err
}

// ServiceListResponsePayload returns available services.
type ServiceListResponsePayload struct {
	Services []ServiceListEntry
}

// ServiceListEntry describes a service in the list.
type ServiceListEntry struct {
	ID           string
	Name         string
	Type         string
	Description  string
	Connected    bool
	Endpoints    []string
	AdminManaged bool
	Enabled      bool
}

func (p *ServiceListResponsePayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	// Encode as JSON for simplicity
	data, err := json.Marshal(p.Services)
	if err != nil {
		return nil, err
	}
	rdgproto.WriteBytes(buf, data)
	return buf.Bytes(), nil
}

func (p *ServiceListResponsePayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	jsonData, err := rdgproto.ReadBytes(r)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonData, &p.Services)
}

// ClientToServicePayload is a message from a client to a service via Lyre.
type ClientToServicePayload struct {
	MessageID string // Unique message ID
	ToService string // Target service ID or name
	Endpoint  string // Endpoint/action on target service
	Payload   []byte // JSON-encoded payload
	// Auth context is added by server (user info)
}

func (p *ClientToServicePayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteString(buf, p.MessageID)
	rdgproto.WriteString(buf, p.ToService)
	rdgproto.WriteString(buf, p.Endpoint)
	rdgproto.WriteBytes(buf, p.Payload)
	return buf.Bytes(), nil
}

func (p *ClientToServicePayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	var err error
	p.MessageID, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.ToService, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.Endpoint, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.Payload, err = rdgproto.ReadBytes(r)
	return err
}

// ServiceToClientPayload is a message from a service to a client via Lyre.
type ServiceToClientPayload struct {
	MessageID   string // Unique message ID
	FromService string // Source service ID
	ToUser      string // Target user ID
	Endpoint    string // Optional: action type
	Payload     []byte // JSON-encoded payload
	ReplyTo     string // Message ID this is replying to
}

func (p *ServiceToClientPayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteString(buf, p.MessageID)
	rdgproto.WriteString(buf, p.FromService)
	rdgproto.WriteString(buf, p.ToUser)
	rdgproto.WriteString(buf, p.Endpoint)
	rdgproto.WriteBytes(buf, p.Payload)
	rdgproto.WriteString(buf, p.ReplyTo)
	return buf.Bytes(), nil
}

func (p *ServiceToClientPayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	var err error
	p.MessageID, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.FromService, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.ToUser, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.Endpoint, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.Payload, err = rdgproto.ReadBytes(r)
	if err != nil {
		return err
	}
	p.ReplyTo, err = rdgproto.ReadString(r)
	return err
}

// RegisterServicePayloadTypes registers service-related payload types.
func RegisterServicePayloadTypes(registry *rdgproto.PayloadRegistry) {
	registry.Register(MsgTypeServiceAuth, func() rdgproto.PayloadUnmarshaler {
		return &ServiceAuthPayload{}
	})
	registry.Register(MsgTypeServiceAuthResponse, func() rdgproto.PayloadUnmarshaler {
		return &ServiceAuthResponsePayload{}
	})
	registry.Register(MsgTypeServiceHeartbeat, func() rdgproto.PayloadUnmarshaler {
		return &ServiceHeartbeatPayload{}
	})
	registry.Register(MsgTypeServiceMessage, func() rdgproto.PayloadUnmarshaler {
		return &ServiceMessagePayload{}
	})
	registry.Register(MsgTypeServiceResponse, func() rdgproto.PayloadUnmarshaler {
		return &ServiceResponsePayload{}
	})
	registry.Register(MsgTypeServiceList, func() rdgproto.PayloadUnmarshaler {
		return &ServiceListPayload{}
	})
	registry.Register(MsgTypeServiceListResponse, func() rdgproto.PayloadUnmarshaler {
		return &ServiceListResponsePayload{}
	})
	registry.Register(MsgTypeClientToService, func() rdgproto.PayloadUnmarshaler {
		return &ClientToServicePayload{}
	})
	registry.Register(MsgTypeServiceToClient, func() rdgproto.PayloadUnmarshaler {
		return &ServiceToClientPayload{}
	})
}
