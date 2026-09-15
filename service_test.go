package lyresdk

import (
	"context"
	"github.com/Lyrinox-Technologies/LyreSDK/wire"
	"github.com/Lyrinox-Technologies/ridged-proto/rdgproto"
	"net"
	"testing"
	"time"
)

func TestServicePublicationAndShutdown(t *testing.T) {
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer listener.Close()
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		conn, e := listener.Accept()
		if e != nil {
			t.Error(e)
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
		p := rdgproto.NewProtocol(conn, &rdgproto.MessageOptions{Registry: rdgproto.NewPayloadRegistry()})
		var auth wire.ServiceAuthPayload
		receive(t, p, wire.MsgTypeServiceAuth, &auth)
		if auth.ServiceID != "test-service" || len(auth.Capabilities) != 1 || auth.Capabilities[0].Name != "text.echo" || auth.PublisherUserID != "publisher" {
			t.Errorf("bad publication %+v", auth)
		}
		sendTest(t, p, wire.MsgTypeServiceAuthResponse, &wire.ServiceAuthResponsePayload{Success: true})
		sendTest(t, p, wire.MsgTypeServiceMessage, &wire.ServiceMessagePayload{MessageID: "request", FromService: "caller", Endpoint: "echo", Payload: []byte(`{"text":"hello","_principal_type":"service"}`)})
		var result wire.ServiceResponsePayload
		receive(t, p, wire.MsgTypeServiceResponse, &result)
		if !result.Success {
			t.Errorf("handler failed: %s", result.Error)
		}
		var heartbeat wire.ServiceHeartbeatPayload
		receive(t, p, wire.MsgTypeServiceHeartbeat, &heartbeat)
		if heartbeat.ServiceID != "test-service" {
			t.Error("wrong heartbeat")
		}
	}()
	s, e := NewService(ServiceConfig{ServiceID: "test-service", Secret: "test-secret", ServerURL: "tcp://" + listener.Addr().String(), PublisherUserID: "publisher", Endpoints: []string{"echo"}, Capabilities: []Capability{{Name: "text.echo", Endpoint: "echo", Version: 1}}, HeartbeatInterval: 20 * time.Millisecond})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	s.Handle("echo", func(r *Request) *Response {
		if r.Principal.ID != "caller" {
			return r.Error("caller identity missing")
		}
		return r.Success(map[string]any{"text": r.Payload["text"]})
	})
	if e = s.Connect(); e != nil {
		t.Fatal(e)
	}
	<-finished
}
func TestRunPersistentCancellation(t *testing.T) {
	s, e := NewService(ServiceConfig{ServiceID: "test", Secret: "secret", ServerURL: "tcp://127.0.0.1:1"})
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan error, 1)
	go func() { done <- s.RunPersistent(ctx) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("canceled service kept reconnecting")
	}
}
