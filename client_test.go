package lyresdk

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Lyrinox-Technologies/LyreSDK/wire"
	"github.com/LyrinoxTechnologies/ridged-proto/rdgproto"
	"net"
	"sync"
	"testing"
	"time"
)

func pair(t *testing.T) (*Client, *rdgproto.Protocol) {
	t.Helper()
	a, b := net.Pipe()
	c, e := NewClient(a, Config{Timeout: time.Second})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { c.Close(); b.Close() })
	_ = b.SetDeadline(time.Now().Add(5 * time.Second))
	return c, rdgproto.NewProtocol(b, &rdgproto.MessageOptions{Registry: rdgproto.NewPayloadRegistry()})
}
func receive(t *testing.T, p *rdgproto.Protocol, kind byte, out interface{ Unmarshal([]byte) error }) {
	t.Helper()
	m, _, e := p.ReceiveMessage()
	if e != nil {
		t.Error(e)
		return
	}
	if m.Type != kind {
		t.Errorf("type %x, want %x", m.Type, kind)
		return
	}
	if e = out.Unmarshal(m.Payload); e != nil {
		t.Error(e)
	}
}
func sendTest(t *testing.T, p *rdgproto.Protocol, kind byte, v any) {
	t.Helper()
	if _, e := p.Send(kind, v); e != nil {
		t.Error(e)
	}
}
func TestAuthenticationMFAAndAgent(t *testing.T) {
	for _, agent := range []bool{false, true} {
		t.Run(map[bool]string{false: "user MFA", true: "agent"}[agent], func(t *testing.T) {
			c, p := pair(t)
			finished := make(chan struct{})
			go func() {
				defer close(finished)
				var auth wire.AuthRequestPayload
				receive(t, p, wire.MsgTypeAuthRequest, &auth)
				if agent {
					if auth.AuthType != wire.AuthTypeAgent || auth.Password != "agent-key" {
						t.Errorf("wrong agent envelope: %+v", auth)
					}
				} else {
					if auth.Username != "alice" {
						t.Error("missing username")
					}
					sendTest(t, p, wire.MsgTypeAuthMFARequired, &wire.MFARequiredPayload{ChallengeID: "challenge", MFAType: "totp"})
					var submit wire.MFASubmitPayload
					receive(t, p, wire.MsgTypeAuthMFASubmit, &submit)
					if submit.Code != "123456" || submit.ChallengeID != "challenge" {
						t.Error("incorrect MFA")
					}
				}
				sendTest(t, p, wire.MsgTypeAuthResponse, &wire.AuthResponsePayload{Success: true, UserID: "id", Token: "token", ExpiresAt: time.Now().Add(time.Hour).Unix()})
			}()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			var e error
			if agent {
				e = c.LoginAgent(ctx, "agent-key")
			} else {
				e = c.Authenticate(ctx, wire.AuthRequestPayload{Username: "alice", Password: "password"}, func(context.Context, wire.MFARequiredPayload) (string, error) { return "123456", nil })
			}
			if e != nil {
				t.Fatal(e)
			}
			if !c.IsAuthenticated() || c.UserID() != "id" {
				t.Error("identity not stored")
			}
			<-finished
		})
	}
}
func TestConcurrentCallsReverseRepliesAndPrecision(t *testing.T) {
	c, p := pair(t)
	c.mu.Lock()
	c.authenticated = true
	c.mu.Unlock()
	const n = 24
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		requests := make([]wire.ClientToServicePayload, n)
		for i := range requests {
			receive(t, p, wire.MsgTypeClientToService, &requests[i])
			if requests[i].ToService != "lyre.json.parse@v1" {
				t.Error("reference lost")
			}
		}
		for i := n - 1; i >= 0; i-- {
			var in map[string]any
			_ = json.Unmarshal(requests[i].Payload, &in)
			body, _ := json.Marshal(map[string]any{"index": in["index"], "large": json.Number("9007199254740993")})
			sendTest(t, p, wire.MsgTypeServiceResponse, &wire.ServiceResponsePayload{MessageID: requests[i].MessageID, Success: true, Payload: body})
		}
	}()
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			var out struct {
				Index int
				Large json.Number
			}
			e := c.Call(ctx, "lyre.json.parse@v1", map[string]int{"index": i}, &out)
			if e != nil {
				t.Error(e)
			} else if out.Index != i || out.Large != "9007199254740993" {
				t.Errorf("bad result: %+v", out)
			}
		}(i)
	}
	wg.Wait()
	<-finished
}
func TestCanceledCallDiscardsLateReply(t *testing.T) {
	c, p := pair(t)
	c.mu.Lock()
	c.authenticated = true
	c.mu.Unlock()
	received := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		var first, second wire.ClientToServicePayload
		receive(t, p, wire.MsgTypeClientToService, &first)
		close(received)
		<-release
		sendTest(t, p, wire.MsgTypeServiceResponse, &wire.ServiceResponsePayload{MessageID: first.MessageID, Success: true, Payload: []byte(`{"value":"late"}`)})
		receive(t, p, wire.MsgTypeClientToService, &second)
		sendTest(t, p, wire.MsgTypeServiceResponse, &wire.ServiceResponsePayload{MessageID: second.MessageID, Success: true, Payload: []byte(`{"value":"current"}`)})
	}()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.Call(ctx, "lyre.json.parse@v1", nil, nil) }()
	<-received
	// The peer may read the last byte before the writer returns from net.Pipe.
	// Cancel after send has released its lock to exercise a completed write.
	c.writeLock <- struct{}{}
	<-c.writeLock
	cancel()
	if e := <-done; !errors.Is(e, context.Canceled) {
		t.Errorf("got %v", e)
	}
	close(release)
	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second)
	defer cancel2()
	var out map[string]string
	if e := c.Call(ctx2, "lyre.json.parse@v1", nil, &out); e != nil {
		t.Fatal(e)
	}
	if out["value"] != "current" {
		t.Fatal(out)
	}
	<-finished
}
func TestControlTimeoutClosesConnection(t *testing.T) {
	c, p := pair(t)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		var req wire.AuthRequestPayload
		receive(t, p, wire.MsgTypeAuthRequest, &req)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, e := c.Request(ctx, wire.MsgTypeAuthRequest, &wire.AuthRequestPayload{}, wire.MsgTypeAuthResponse)
	if !errors.Is(e, context.DeadlineExceeded) || c.IsConnected() {
		t.Fatalf("error %v, connected %v", e, c.IsConnected())
	}
	<-finished
}
func TestHandlerCanCallCapability(t *testing.T) {
	c, p := pair(t)
	c.mu.Lock()
	c.authenticated = true
	c.serviceID = "service"
	c.handler = func(r *Request) *Response {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		var out map[string]any
		if e := c.Call(ctx, "lyre.json.parse@v1", nil, &out); e != nil {
			return r.Error(e.Error())
		}
		return r.Success(out)
	}
	c.mu.Unlock()
	sendTest(t, p, wire.MsgTypeServiceMessage, &wire.ServiceMessagePayload{MessageID: "outer", FromService: "caller", Endpoint: "nested", Payload: []byte(`{}`)})
	var nested wire.ServiceMessagePayload
	receive(t, p, wire.MsgTypeServiceMessage, &nested)
	sendTest(t, p, wire.MsgTypeServiceResponse, &wire.ServiceResponsePayload{MessageID: nested.MessageID, Success: true, Payload: []byte(`{"ok":true}`)})
	var result wire.ServiceResponsePayload
	receive(t, p, wire.MsgTypeServiceResponse, &result)
	if !result.Success || result.MessageID != "outer" {
		t.Fatalf("%+v", result)
	}
}
func TestReferences(t *testing.T) {
	for _, s := range []string{"lyre.cache.get", "lyre.cache.get@v1", "lyre.cache.get@vetheon.cache.get@v1"} {
		r, e := ParseReference(s)
		if e != nil || r.String() != s {
			t.Errorf("%s: %+v %v", s, r, e)
		}
	}
	for _, s := range []string{"cache.get", "lyre.cache.get@v0", "lyre.cache.get@v1@lyrinox.cache.get", "lyre.cache.get@v1@v2"} {
		if _, e := ParseReference(s); e == nil {
			t.Errorf("accepted %s", s)
		}
	}
}
