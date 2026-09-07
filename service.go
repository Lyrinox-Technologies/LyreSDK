package lyresdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Lyrinox-Technologies/LyreSDK/wire"
	"github.com/LyrinoxTechnologies/ridged-proto/rdgproto"
)

type Capability = wire.ServiceCapability
type Extension = wire.Extension
type ExtensionError = wire.ExtensionError
type Execution = wire.Execution
type Principal struct{ Type, ID, Username, Email, AgentName string }
type Request struct {
	MessageID   string
	FromService string
	FromUser    string
	Principal   Principal
	Endpoint    string
	Payload     map[string]any
	RawPayload  json.RawMessage
}

func (r *Request) Success(payload map[string]any) *Response {
	return &Response{Success: true, Payload: payload}
}
func (r *Request) Error(message string) *Response { return &Response{Success: false, Error: message} }
func (r *Request) Errorf(format string, args ...any) *Response {
	return r.Error(fmt.Sprintf(format, args...))
}

type HandlerFunc func(*Request) *Response

type ServiceConfig struct {
	ServiceID, ServiceName, ServiceType, Description, Secret, ServerURL string
	PublisherUserID, PublisherPrivateKey, PublisherPrivateKeyFile       string
	Endpoints                                                           []string
	Capabilities                                                        []Capability
	HeartbeatInterval, ReconnectDelay                                   time.Duration
	ReconnectSchedule                                                   []time.Duration
	Logger                                                              *log.Logger
	Client                                                              Config
}
type Service struct {
	config    ServiceConfig
	mu        sync.RWMutex
	connectMu sync.Mutex
	client    *Client
	handlers  map[string]HandlerFunc
}

func NewService(cfg ServiceConfig) (*Service, error) {
	if cfg.ServiceID == "" || cfg.Secret == "" || cfg.ServerURL == "" {
		return nil, errors.New("service ID, secret and server URL are required")
	}
	if !identifier.MatchString(cfg.ServiceID) {
		return nil, errors.New("invalid service ID")
	}
	if cfg.ServiceName == "" {
		cfg.ServiceName = cfg.ServiceID
	}
	if cfg.ServiceType == "" {
		cfg.ServiceType = "backend"
	}
	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = 30 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = log.Default()
	}
	if len(cfg.ReconnectSchedule) == 0 {
		if cfg.ReconnectDelay > 0 {
			cfg.ReconnectSchedule = []time.Duration{cfg.ReconnectDelay}
		} else {
			cfg.ReconnectSchedule = DefaultReconnectSchedule()
		}
	}
	for _, delay := range cfg.ReconnectSchedule {
		if delay <= 0 {
			return nil, errors.New("reconnect delays must be positive")
		}
	}
	endpoints := map[string]bool{}
	for _, e := range cfg.Endpoints {
		if !identifier.MatchString(e) {
			return nil, errors.New("invalid endpoint")
		}
		endpoints[e] = true
	}
	for _, cap := range cfg.Capabilities {
		if !capabilityName.MatchString(cap.Name) || !endpoints[cap.Endpoint] {
			return nil, errors.New("capability must name a contract and an owned endpoint")
		}
		if cap.InputSchema != "" && !json.Valid([]byte(cap.InputSchema)) || cap.OutputSchema != "" && !json.Valid([]byte(cap.OutputSchema)) {
			return nil, errors.New("invalid capability schema")
		}
	}
	return &Service{config: cfg, handlers: map[string]HandlerFunc{}}, nil
}
func DefaultReconnectSchedule() []time.Duration {
	return []time.Duration{5 * time.Minute, 10 * time.Minute, 20 * time.Minute, 40 * time.Minute, 80 * time.Minute, 160 * time.Minute, 24 * time.Hour}
}
func (s *Service) Handle(endpoint string, handler HandlerFunc) {
	s.mu.Lock()
	s.handlers[endpoint] = handler
	s.mu.Unlock()
}
func (s *Service) Connect() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return s.ConnectContext(ctx)
}
func (s *Service) ConnectContext(ctx context.Context) error {
	s.connectMu.Lock()
	defer s.connectMu.Unlock()
	if s.IsConnected() {
		return errors.New("service is already connected")
	}
	c, err := Dial(ctx, s.config.ServerURL, s.config.Client)
	if err != nil {
		return err
	}
	ready := make(chan struct{})
	c.mu.Lock()
	c.handler = func(r *Request) *Response {
		select {
		case <-ready:
		case <-c.Done():
			return r.Error("service disconnected during authentication")
		}
		s.mu.RLock()
		h := s.handlers[r.Endpoint]
		if h == nil {
			h = s.handlers["*"]
		}
		s.mu.RUnlock()
		if h == nil {
			return r.Error("unknown endpoint")
		}
		return h(r)
	}
	c.mu.Unlock()
	key := s.config.PublisherPrivateKey
	if key == "" && s.config.PublisherPrivateKeyFile != "" {
		data, e := os.ReadFile(s.config.PublisherPrivateKeyFile)
		if e != nil {
			c.Close()
			return e
		}
		key = string(data)
	}
	message, err := c.Request(ctx, wire.MsgTypeServiceAuth, &wire.ServiceAuthPayload{ServiceID: s.config.ServiceID, Secret: s.config.Secret, Endpoints: s.config.Endpoints, Name: s.config.ServiceName, Type: s.config.ServiceType, Description: s.config.Description, Capabilities: s.config.Capabilities, PublisherUserID: s.config.PublisherUserID, PublisherPrivateKey: key}, wire.MsgTypeServiceAuthResponse)
	if err != nil {
		c.Close()
		return err
	}
	var reply wire.ServiceAuthResponsePayload
	if err = reply.Unmarshal(message.Payload); err != nil {
		c.Close()
		return err
	}
	if !reply.Success {
		c.Close()
		return errors.New("service authentication failed: " + reply.Message)
	}
	c.mu.Lock()
	c.authenticated = true
	c.serviceID = s.config.ServiceID
	c.mu.Unlock()
	s.mu.Lock()
	s.client = c
	s.mu.Unlock()
	close(ready)
	s.config.Logger.Printf("[%s] Connected and authenticated", s.config.ServiceID)
	go func() {
		ticker := time.NewTicker(s.config.HeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-c.done:
				return
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), c.cfg.Timeout)
				err := c.send(ctx, wire.MsgTypeServiceHeartbeat, &wire.ServiceHeartbeatPayload{ServiceID: s.config.ServiceID, Timestamp: time.Now().Unix()})
				cancel()
				if err != nil {
					return
				}
			}
		}
	}()
	return nil
}
func (s *Service) connection() (*Client, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.client == nil {
		return nil, ErrClosed
	}
	return s.client, nil
}
func (s *Service) IsConnected() bool { c, e := s.connection(); return e == nil && c.IsConnected() }
func (s *Service) Run() error {
	c, e := s.connection()
	if e != nil {
		return e
	}
	return c.Wait()
}
func (s *Service) Close() error {
	c, e := s.connection()
	if e != nil {
		return nil
	}
	return c.Close()
}
func (s *Service) RunPersistent(ctx context.Context) error {
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		dialCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		err := s.ConnectContext(dialCtx)
		cancel()
		if err == nil {
			attempt = 0
			c, _ := s.connection()
			select {
			case <-ctx.Done():
				c.Close()
				return ctx.Err()
			case <-c.Done():
				err = c.Wait()
			}
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		s.config.Logger.Printf("[%s] Connection ended: %v", s.config.ServiceID, err)
		index := attempt
		if index >= len(s.config.ReconnectSchedule) {
			index = len(s.config.ReconnectSchedule) - 1
		}
		timer := time.NewTimer(s.config.ReconnectSchedule[index])
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
func (s *Service) Call(ctx context.Context, ref string, input, output any) error {
	c, e := s.connection()
	if e != nil {
		return e
	}
	return c.Call(ctx, ref, input, output)
}
func (s *Service) CallCapability(ref string, input map[string]any, timeout time.Duration) (*Response, error) {
	c, e := s.connection()
	if e != nil {
		return nil, e
	}
	return c.CallCapability(ref, input, timeout)
}
func (s *Service) CallCapabilityVersion(name string, version uint32, input map[string]any, timeout time.Duration) (*Response, error) {
	r, e := ParseReference(name)
	if e != nil {
		return nil, e
	}
	r.ContractVersion = version
	return s.CallCapability(r.String(), input, timeout)
}
func (s *Service) CallProviderCapability(name, provider, implementation string, input map[string]any, timeout time.Duration) (*Response, error) {
	r, e := ParseReference(name)
	if e != nil {
		return nil, e
	}
	r.ProviderID = provider
	r.ProviderCapabilityID = implementation
	return s.CallCapability(r.String(), input, timeout)
}
func (s *Service) CallProviderCapabilityVersion(name, provider, implementation string, version uint32, input map[string]any, timeout time.Duration) (*Response, error) {
	r, e := ParseReference(name)
	if e != nil {
		return nil, e
	}
	r.ProviderID = provider
	r.ProviderCapabilityID = implementation
	r.ContractVersion = version
	return s.CallCapability(r.String(), input, timeout)
}
func (s *Service) CallProviderExtension(name, provider, implementation, extension string, input, options map[string]any, timeout time.Duration) (*Response, error) {
	copy := map[string]any{}
	for k, v := range input {
		copy[k] = v
	}
	copy["extension"] = map[string]any{"id": extension, "options": options}
	return s.CallProviderCapability(name, provider, implementation, copy, timeout)
}
func GenerateSecret() (string, error) { return requestID() }

func (c *Client) dispatch(m *rdgproto.Message) {
	var p wire.ServiceMessagePayload
	if err := p.Unmarshal(m.Payload); err != nil {
		c.shutdown(err)
		return
	}
	body := map[string]any{}
	if len(p.Payload) > 0 {
		if err := decodeJSON(p.Payload, &body); err != nil {
			c.respond(p.MessageID, &Response{Error: "invalid JSON payload"})
			return
		}
	}
	r := &Request{MessageID: p.MessageID, FromService: p.FromService, Endpoint: p.Endpoint, Payload: body, RawPayload: p.Payload}
	kind, _ := body["_principal_type"].(string)
	switch kind {
	case "user":
		r.Principal.Type = kind
		r.Principal.ID, _ = body["_user_id"].(string)
		r.Principal.Username, _ = body["_username"].(string)
		r.Principal.Email, _ = body["_email"].(string)
		r.FromUser = r.Principal.ID
	case "agent":
		r.Principal.Type = kind
		r.Principal.ID, _ = body["_agent_id"].(string)
		r.Principal.AgentName, _ = body["_agent_name"].(string)
	case "service":
		if !strings.HasPrefix(p.FromService, "user:") {
			r.Principal = Principal{Type: kind, ID: p.FromService}
		}
	}
	select {
	case c.workers <- struct{}{}:
		go func() {
			defer func() { <-c.workers }()
			var response *Response
			func() {
				defer func() {
					if recover() != nil {
						response = r.Error("provider handler failed")
					}
				}()
				c.mu.RLock()
				h := c.handler
				c.mu.RUnlock()
				if h == nil {
					response = r.Error("no request handler")
				} else {
					response = h(r)
				}
			}()
			if response == nil {
				response = r.Error("handler returned no response")
			}
			c.respond(p.MessageID, response)
		}()
	default:
		c.respond(p.MessageID, r.Error("handler capacity exceeded"))
	}
}
func (c *Client) respond(id string, r *Response) {
	data, e := json.Marshal(r.Payload)
	if e != nil {
		r = &Response{Error: "handler response is not JSON"}
		data = []byte("{}")
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.cfg.Timeout)
	defer cancel()
	_ = c.send(ctx, wire.MsgTypeServiceResponse, &wire.ServiceResponsePayload{MessageID: id, Success: r.Success, Payload: data, Error: r.Error})
}
