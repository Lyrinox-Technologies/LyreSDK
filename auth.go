package lyresdk

import (
	"context"
	"errors"
	"time"

	"github.com/Lyrinox-Technologies/LyreSDK/wire"
	"github.com/Lyrinox-Technologies/ridged-proto/rdgproto"
)

// Authenticate supports users, explicit device identities, agent API keys and
// token resumption. Lyre owns MFA policy; the SDK only presents challenges.
func (c *Client) Authenticate(ctx context.Context, request wire.AuthRequestPayload, mfa func(context.Context, wire.MFARequiredPayload) (string, error)) error {
	select {
	case c.controlLock <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		return ErrClosed
	}
	defer func() { <-c.controlLock }()
	if request.AuthType == 0 {
		request.AuthType = wire.AuthTypeStandard
	}
	kind := wire.MsgTypeAuthRequest
	var payload any = &request
	for {
		msg, err := c.request(ctx, kind, payload, wire.MsgTypeAuthResponse, wire.MsgTypeAuthMFAResponse, wire.MsgTypeAuthMFARequired, wire.MsgTypeNewDeviceMFA)
		if err != nil {
			return err
		}
		if msg.Type == wire.MsgTypeAuthMFARequired || msg.Type == wire.MsgTypeNewDeviceMFA {
			var challenge wire.MFARequiredPayload
			if err = challenge.Unmarshal(msg.Payload); err != nil {
				return err
			}
			if mfa == nil {
				return &MFARequiredError{Challenge: challenge}
			}
			code, err := mfa(ctx, challenge)
			if err != nil {
				return err
			}
			kind = wire.MsgTypeAuthMFASubmit
			if msg.Type == wire.MsgTypeNewDeviceMFA {
				kind = wire.MsgTypeNewDeviceMFASubmit
			}
			payload = &wire.MFASubmitPayload{ChallengeID: challenge.ChallengeID, Code: code}
			continue
		}
		var response wire.AuthResponsePayload
		if err = response.Unmarshal(msg.Payload); err != nil {
			return err
		}
		if !response.Success {
			return errors.New(response.Message)
		}
		c.mu.Lock()
		c.userID = response.UserID
		c.token = response.Token
		c.expiresAt = time.Unix(response.ExpiresAt, 0)
		c.authenticated = true
		c.mu.Unlock()
		return nil
	}
}

type MFARequiredError struct{ Challenge wire.MFARequiredPayload }

func (e *MFARequiredError) Error() string {
	return "Lyre requires " + e.Challenge.MFAType + " verification"
}
func (c *Client) Login(username, password string) error {
	return c.LoginWithTimeout(username, password, c.cfg.Timeout)
}
func (c *Client) LoginWithTimeout(username, password string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req := wire.AuthRequestPayload{AuthType: wire.AuthTypeStandard, Username: username, Password: password}
	if c.cfg.DeviceID != "" || c.cfg.EphemeralDeviceID != "" {
		if c.cfg.DeviceID == "" || c.cfg.EphemeralDeviceID == "" {
			return errors.New("device authentication requires both device identities")
		}
		req.AuthType = wire.AuthTypeMachine
		req.ReMachID = c.cfg.DeviceID
		req.EMachID = c.cfg.EphemeralDeviceID
	}
	c.mu.RLock()
	handler := c.mfaHandler
	c.mu.RUnlock()
	var callback func(context.Context, wire.MFARequiredPayload) (string, error)
	if handler != nil {
		callback = func(_ context.Context, ch wire.MFARequiredPayload) (string, error) {
			return handler(ch.MFAType, ch.Hint), nil
		}
	}
	return c.Authenticate(ctx, req, callback)
}
func (c *Client) LoginWithToken(token string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.cfg.Timeout)
	defer cancel()
	req := wire.AuthRequestPayload{AuthType: wire.AuthTypeStandard, Token: token}
	if c.cfg.DeviceID != "" {
		req.AuthType = wire.AuthTypeMachine
		req.ReMachID = c.cfg.DeviceID
		req.EMachID = c.cfg.EphemeralDeviceID
	}
	return c.Authenticate(ctx, req, nil)
}
func (c *Client) LoginAgent(ctx context.Context, apiKey string) error {
	return c.Authenticate(ctx, wire.AuthRequestPayload{AuthType: wire.AuthTypeAgent, Password: apiKey}, nil)
}
func (c *Client) OnMFARequired(handler func(string, string) string) {
	c.mu.Lock()
	c.mfaHandler = handler
	c.mu.Unlock()
}
func (c *Client) Register(username, email, password, realName, country string) (*wire.RegisterResponsePayload, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.cfg.Timeout)
	defer cancel()
	m, e := c.Request(ctx, wire.MsgTypeRegisterRequest, &wire.RegisterRequestPayload{Username: username, Email: email, Password: password, RealName: realName, Country: country}, wire.MsgTypeRegisterResponse)
	if e != nil {
		return nil, e
	}
	var r wire.RegisterResponsePayload
	e = r.Unmarshal(m.Payload)
	return &r, e
}
func (c *Client) VerifyEmail(token string) (*wire.VerifyEmailResponsePayload, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.cfg.Timeout)
	defer cancel()
	m, e := c.Request(ctx, wire.MsgTypeVerifyEmailRequest, &wire.VerifyEmailRequestPayload{Token: token}, wire.MsgTypeVerifyEmailResponse)
	if e != nil {
		return nil, e
	}
	var r wire.VerifyEmailResponsePayload
	e = r.Unmarshal(m.Payload)
	return &r, e
}
func (c *Client) Logout(ctx context.Context) error {
	message, err := c.Request(ctx, wire.MsgTypeAuthLogout, &wire.LogoutPayload{Token: c.Token()}, wire.MsgTypeAuthLogoutResponse)
	if err == nil {
		var response wire.LogoutResponsePayload
		if err = response.Unmarshal(message.Payload); err != nil {
			return err
		}
		if !response.Success {
			return errors.New(response.Message)
		}
		c.mu.Lock()
		c.authenticated = false
		c.token = ""
		c.userID = ""
		c.mu.Unlock()
	}
	return err
}

func (c *Client) OnMessage(h func(string, string)) { c.mu.Lock(); c.onMessage = h; c.mu.Unlock() }
func (c *Client) OnPing(h func(string, string))    { c.mu.Lock(); c.onPing = h; c.mu.Unlock() }
func (c *Client) OnPong(h func(string, int64))     { c.mu.Lock(); c.onPong = h; c.mu.Unlock() }
func (c *Client) OnServicePush(h func(string, string, map[string]any)) {
	c.mu.Lock()
	c.onServicePush = h
	c.mu.Unlock()
}
func (c *Client) OnDisconnect(h func(error)) { c.mu.Lock(); c.onDisconnect = h; c.mu.Unlock() }
func (c *Client) SendMessage(target, message string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.cfg.Timeout)
	defer cancel()
	return c.send(ctx, wire.MsgTypeDirectMessage, &wire.DirectMessagePayload{TargetUser: target, Message: message, Timestamp: time.Now().UnixMilli()})
}
func (c *Client) Ping(target, message string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.cfg.Timeout)
	defer cancel()
	return c.send(ctx, wire.MsgTypePing, &wire.PingPayload{TargetUser: target, Message: message, Timestamp: time.Now().UnixMilli()})
}
func (c *Client) event(m *rdgproto.Message) {
	c.mu.RLock()
	onMessage, onPing, onPong, onPush := c.onMessage, c.onPing, c.onPong, c.onServicePush
	c.mu.RUnlock()
	var callback func()
	switch m.Type {
	case wire.MsgTypeDirectMessage:
		var p wire.DirectMessagePayload
		if p.Unmarshal(m.Payload) == nil && onMessage != nil {
			callback = func() { onMessage(p.TargetUser, p.Message) }
		}
	case wire.MsgTypePing:
		var p wire.PingPayload
		if p.Unmarshal(m.Payload) == nil {
			callback = func() {
				ctx, cancel := context.WithTimeout(context.Background(), c.cfg.Timeout)
				defer cancel()
				_ = c.send(ctx, wire.MsgTypePong, &wire.PongPayload{FromUser: p.TargetUser, Message: p.Message, OrigTimestamp: p.Timestamp, Timestamp: time.Now().UnixMilli()})
				if onPing != nil {
					onPing(p.TargetUser, p.Message)
				}
			}
		}
	case wire.MsgTypePong:
		var p wire.PongPayload
		if p.Unmarshal(m.Payload) == nil && onPong != nil {
			callback = func() { onPong(p.FromUser, time.Now().UnixMilli()-int64(p.OrigTimestamp)) }
		}
	case wire.MsgTypeServiceToClient:
		var p wire.ServiceToClientPayload
		if p.Unmarshal(m.Payload) == nil && onPush != nil {
			var body map[string]any
			if decodeJSON(p.Payload, &body) == nil {
				callback = func() { onPush(p.FromService, p.Endpoint, body) }
			}
		}
	}
	if callback != nil {
		select {
		case c.workers <- struct{}{}:
			go func() { defer func() { <-c.workers; _ = recover() }(); callback() }()
		default:
			c.shutdown(errors.New("event handler capacity exhausted"))
		}
	}
}
