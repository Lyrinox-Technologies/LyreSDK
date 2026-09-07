// Package lyresdk provides the default Go conveniences for speaking Lyre over
// RDGProto. It depends only on RDGProto and the Go standard library.
package lyresdk

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/Lyrinox-Technologies/LyreSDK/wire"
	"github.com/LyrinoxTechnologies/ridged-proto/rdgproto"
)

var ErrClosed = errors.New("Lyre connection closed")

type Config struct {
	Timeout               time.Duration
	Transport             TransportOptions
	DeviceID              string
	EphemeralDeviceID     string
	MaxConcurrentHandlers int
}
type Response struct {
	Success    bool
	Payload    map[string]any
	RawPayload json.RawMessage
	Error      string
}
type CapabilityError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

func (e *CapabilityError) Error() string {
	if e.Code == "" {
		return e.Message
	}
	return e.Code + ": " + e.Message
}

type ProtocolError struct {
	Code    wire.ErrorCode
	Message string
}

func (e *ProtocolError) Error() string { return fmt.Sprintf("Lyre error %d: %s", e.Code, e.Message) }

type result struct {
	response *Response
	err      error
}
type packet struct {
	message *rdgproto.Message
	err     error
}
type Client struct {
	conn          rdgproto.Connection
	proto         *rdgproto.Protocol
	cfg           Config
	done          chan struct{}
	closeOnce     sync.Once
	mu            sync.RWMutex
	cause         error
	pending       map[string]chan result
	control       chan packet
	controlTypes  map[byte]bool
	controlLock   chan struct{}
	writeLock     chan struct{}
	handler       func(*Request) *Response
	workers       chan struct{}
	serviceID     string
	token, userID string
	expiresAt     time.Time
	authenticated bool
	mfaHandler    func(string, string) string
	onMessage     func(string, string)
	onPing        func(string, string)
	onPong        func(string, int64)
	onServicePush func(string, string, map[string]any)
	onDisconnect  func(error)
}

// NewClient takes ownership of a raw RDGProto connection and starts its reader.
// The supplied connection can be net.Conn, a pipe, or a caller-owned transport.
func NewClient(conn rdgproto.Connection, cfg Config) (*Client, error) {
	if conn == nil {
		return nil, errors.New("connection is required")
	}
	if (cfg.DeviceID == "") != (cfg.EphemeralDeviceID == "") {
		return nil, errors.New("device authentication requires both device identities")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.MaxConcurrentHandlers <= 0 {
		cfg.MaxConcurrentHandlers = 64
	}
	c := &Client{conn: conn, cfg: cfg, proto: rdgproto.NewProtocol(conn, &rdgproto.MessageOptions{Registry: rdgproto.NewPayloadRegistry()}), done: make(chan struct{}), pending: make(map[string]chan result), controlLock: make(chan struct{}, 1), writeLock: make(chan struct{}, 1), workers: make(chan struct{}, cfg.MaxConcurrentHandlers)}
	go c.readLoop()
	return c, nil
}
func Dial(ctx context.Context, address string, cfg Config) (*Client, error) {
	conn, err := DialTransport(ctx, address, cfg.Transport)
	if err != nil {
		return nil, err
	}
	c, err := NewClient(conn, cfg)
	if err != nil {
		conn.Close()
	}
	return c, err
}
func (c *Client) Done() <-chan struct{} { return c.done }
func (c *Client) IsConnected() bool {
	select {
	case <-c.done:
		return false
	default:
		return true
	}
}
func (c *Client) IsAuthenticated() bool      { c.mu.RLock(); defer c.mu.RUnlock(); return c.authenticated }
func (c *Client) UserID() string             { c.mu.RLock(); defer c.mu.RUnlock(); return c.userID }
func (c *Client) Token() string              { c.mu.RLock(); defer c.mu.RUnlock(); return c.token }
func (c *Client) MachineID() string          { return c.cfg.DeviceID }
func (c *Client) EphemeralMachineID() string { return c.cfg.EphemeralDeviceID }
func (c *Client) Wait() error                { <-c.done; c.mu.RLock(); defer c.mu.RUnlock(); return c.cause }
func (c *Client) Close() error               { c.shutdown(ErrClosed); return nil }
func (c *Client) shutdown(cause error) {
	c.closeOnce.Do(func() {
		c.mu.Lock()
		c.cause = cause
		c.authenticated = false
		callback := c.onDisconnect
		c.mu.Unlock()
		_ = c.conn.Close()
		close(c.done)
		if callback != nil {
			go callback(cause)
		}
	})
}

func (c *Client) send(ctx context.Context, kind byte, payload any) error {
	select {
	case c.writeLock <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		return ErrClosed
	}
	defer func() { <-c.writeLock }()
	if err := ctx.Err(); err != nil {
		return err
	}
	// A canceled blocked write cannot leave a partial RDGProto frame reusable.
	stop := context.AfterFunc(ctx, func() { c.shutdown(ctx.Err()) })
	defer stop()
	_, err := c.proto.Send(kind, payload)
	if err != nil {
		c.shutdown(err)
	}
	return err
}

// Send exposes typed/raw RDGProto payload sending without a second protocol.
// Use Request for control responses and Call for correlated capability calls.
func (c *Client) Send(ctx context.Context, kind byte, payload any) error {
	return c.send(ctx, kind, payload)
}
func (c *Client) Request(ctx context.Context, kind byte, payload any, responseTypes ...byte) (*rdgproto.Message, error) {
	select {
	case c.controlLock <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.done:
		return nil, ErrClosed
	}
	defer func() { <-c.controlLock }()
	return c.request(ctx, kind, payload, responseTypes...)
}
func (c *Client) request(ctx context.Context, kind byte, payload any, responseTypes ...byte) (*rdgproto.Message, error) {
	ch := make(chan packet, 1)
	types := map[byte]bool{}
	for _, t := range responseTypes {
		types[t] = true
	}
	c.mu.Lock()
	c.control = ch
	c.controlTypes = types
	c.mu.Unlock()
	defer func() { c.mu.Lock(); c.control = nil; c.controlTypes = nil; c.mu.Unlock() }()
	if err := c.send(ctx, kind, payload); err != nil {
		return nil, err
	}
	select {
	case p := <-ch:
		return p.message, p.err
	case <-c.done:
		return nil, c.Wait()
	case <-ctx.Done():
		c.shutdown(ctx.Err())
		return nil, ctx.Err()
	}
}

func (c *Client) readLoop() {
	for {
		m, _, err := c.proto.ReceiveMessage()
		if err != nil {
			c.shutdown(err)
			return
		}
		if m.Type == wire.MsgTypeError {
			var e wire.ErrorPayload
			if err = e.Unmarshal(m.Payload); err != nil {
				c.shutdown(err)
				return
			}
			failure := &ProtocolError{Code: e.Code, Message: e.Message}
			c.mu.Lock()
			if c.control != nil {
				select {
				case c.control <- packet{err: failure}:
				default:
				}
			}
			for _, ch := range c.pending {
				select {
				case ch <- result{err: failure}:
				default:
				}
			}
			c.mu.Unlock()
			continue
		}
		if m.Type == wire.MsgTypeServiceResponse {
			var response wire.ServiceResponsePayload
			if err = response.Unmarshal(m.Payload); err != nil {
				c.shutdown(err)
				return
			}
			body := map[string]any{}
			if len(response.Payload) > 0 {
				if err = decodeJSON(response.Payload, &body); err != nil {
					c.shutdown(err)
					return
				}
			}
			c.mu.RLock()
			ch := c.pending[response.MessageID]
			c.mu.RUnlock()
			if ch != nil {
				select {
				case ch <- result{response: &Response{Success: response.Success, Error: response.Error, Payload: body, RawPayload: response.Payload}}:
				default:
				}
			}
			continue
		}
		if m.Type == wire.MsgTypeServiceMessage {
			c.dispatch(m)
			continue
		}
		c.mu.RLock()
		control, matched := c.control, c.controlTypes[m.Type]
		c.mu.RUnlock()
		if control != nil && matched {
			select {
			case control <- packet{message: m}:
			default:
			}
			continue
		}
		c.event(m)
	}
}
func decodeJSON(data []byte, target any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	if err := d.Decode(target); err != nil {
		return err
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return errors.New("expected one JSON value")
	}
	return nil
}
func requestID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// CallResponse returns the full correlated response, including provider errors.
func (c *Client) CallResponse(ctx context.Context, reference string, input any) (*Response, error) {
	if _, err := ParseReference(reference); err != nil {
		return nil, err
	}
	return c.call(ctx, reference, "", input)
}
func (c *Client) call(ctx context.Context, target, endpoint string, input any) (*Response, error) {
	if !c.IsAuthenticated() {
		return nil, errors.New("Lyre authentication required")
	}
	data, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	if string(data) == "null" {
		data = []byte("{}")
	}
	var obj map[string]any
	if err := decodeJSON(data, &obj); err != nil {
		return nil, errors.New("capability input must be a JSON object")
	}
	id, err := requestID()
	if err != nil {
		return nil, err
	}
	ch := make(chan result, 1)
	c.mu.Lock()
	c.pending[id] = ch
	serviceID := c.serviceID
	c.mu.Unlock()
	defer func() { c.mu.Lock(); delete(c.pending, id); c.mu.Unlock() }()
	if serviceID != "" {
		err = c.send(ctx, wire.MsgTypeServiceMessage, &wire.ServiceMessagePayload{MessageID: id, ToService: target, Endpoint: endpoint, Payload: data})
	} else {
		err = c.send(ctx, wire.MsgTypeClientToService, &wire.ClientToServicePayload{MessageID: id, ToService: target, Endpoint: endpoint, Payload: data})
	}
	if err != nil {
		return nil, err
	}
	select {
	case r := <-ch:
		return r.response, r.err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.done:
		return nil, c.Wait()
	}
}

// Call decodes a contract response into an application-owned Go value.
func (c *Client) Call(ctx context.Context, reference string, input, output any) error {
	r, err := c.CallResponse(ctx, reference, input)
	if err != nil {
		return err
	}
	if !r.Success {
		return responseError(r.Error)
	}
	if output == nil {
		return nil
	}
	return decodeJSON(r.RawPayload, output)
}
func responseError(message string) error {
	var e CapabilityError
	if json.Unmarshal([]byte(message), &e) == nil && e.Code != "" {
		return &e
	}
	return &CapabilityError{Message: message}
}
func (c *Client) CallCapability(name string, payload map[string]any, timeout time.Duration) (*Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return c.CallResponse(ctx, name, payload)
}
func (c *Client) CallService(service, endpoint string, payload map[string]any) (map[string]any, error) {
	return c.CallServiceWithTimeout(service, endpoint, payload, c.cfg.Timeout)
}
func (c *Client) CallServiceWithTimeout(service, endpoint string, payload map[string]any, timeout time.Duration) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	r, e := c.call(ctx, service, endpoint, payload)
	if e != nil {
		return nil, e
	}
	if !r.Success {
		return nil, responseError(r.Error)
	}
	return r.Payload, nil
}
