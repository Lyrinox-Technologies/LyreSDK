package lyresdk

import (
	"context"
	"errors"
	"github.com/Lyrinox-Technologies/LyreSDK/wire"
)

const IdentityServerKind = "server"

type IdentityServer = wire.IdentityServer
type IdentityObject = wire.IdentityObject
type IdentityObjectRequest = wire.IdentityObjectRequestPayload

func (c *Client) IdentityObjects(ctx context.Context, r IdentityObjectRequest) ([]IdentityObject, error) {
	msg, e := c.Request(ctx, wire.MsgTypeIdentityObjectRequest, &r, wire.MsgTypeIdentityObjectResponse)
	if e != nil {
		return nil, e
	}
	var response wire.IdentityObjectResponsePayload
	if e = response.Unmarshal(msg.Payload); e != nil {
		return nil, e
	}
	if !response.Success {
		return nil, errors.New(response.Message)
	}
	return response.Objects, nil
}

// IdentityObjects reads only this service's explicit identity attachments.
// The Lyre server rejects provider writes and access to other service scopes.
func (s *Service) IdentityObjects(ctx context.Context, r IdentityObjectRequest) ([]IdentityObject, error) {
	c, e := s.connection()
	if e != nil {
		return nil, e
	}
	return c.IdentityObjects(ctx, r)
}
func (c *Client) AttachServer(ctx context.Context, service, name, platform string) (IdentityObject, error) {
	var o IdentityObject
	data, e := c.IdentityObjects(ctx, IdentityObjectRequest{Action: "attach", Service: service, Kind: IdentityServerKind, ExternalID: c.cfg.DeviceID, Name: name, Server: &IdentityServer{MachineID: c.cfg.DeviceID, Platform: platform}})
	if e != nil {
		return o, e
	}
	if len(data) != 1 {
		return o, errors.New("invalid server attachment response")
	}
	return data[0], nil
}

// ListIdentityServers returns only active server attachments. Pagination is
// bounded; a caller never receives a silently truncated authorization inventory.
func (s *Service) ListIdentityServers(ctx context.Context, owner string) ([]IdentityObject, error) {
	out := []IdentityObject{}
	after := ""
	for page := 0; page < 10; page++ {
		data, e := s.IdentityObjects(ctx, IdentityObjectRequest{Action: "list", Owner: owner, Kind: IdentityServerKind, After: after})
		if e != nil {
			return nil, e
		}
		batch := data
		for _, o := range batch {
			if o.Owner != owner || o.Service != s.config.ServiceID || o.Kind != IdentityServerKind || o.Server == nil {
				return nil, errors.New("invalid identity server response")
			}
			if !o.Revoked {
				out = append(out, o)
			}
		}
		if len(batch) < 100 {
			return out, nil
		}
		next := batch[len(batch)-1].ID
		if next == after {
			return nil, errors.New("identity pagination did not advance")
		}
		after = next
	}
	return nil, errors.New("identity server inventory exceeds client limit")
}
