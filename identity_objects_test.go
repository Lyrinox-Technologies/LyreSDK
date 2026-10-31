package lyresdk

import (
	"context"
	"github.com/Lyrinox-Technologies/LyreSDK/wire"
	"github.com/Lyrinox-Technologies/ridged-proto/rdgproto"
	"net"
	"testing"
	"time"
)

func TestIdentityControlRequest(t *testing.T) {
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer listener.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, e := listener.Accept()
		if e != nil {
			t.Error(e)
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
		p := rdgproto.NewProtocol(conn, &rdgproto.MessageOptions{Registry: rdgproto.NewPayloadRegistry()})
		var request wire.IdentityObjectRequestPayload
		receive(t, p, wire.MsgTypeIdentityObjectRequest, &request)
		r := request
		if r.Kind != "server" || r.Server.MachineID != "pc" || r.Owner != "" {
			t.Error("invalid attach request")
		}
		sendTest(t, p, wire.MsgTypeIdentityObjectResponse, &wire.IdentityObjectResponsePayload{Success: true, Objects: []wire.IdentityObject{{ID: "object", Owner: "alice", Service: "syscodex-provider", Kind: "server", Server: &wire.IdentityServer{MachineID: "pc"}}}})
	}()
	c, e := Dial(context.Background(), "tcp://"+listener.Addr().String(), Config{DeviceID: "pc", EphemeralDeviceID: "ephemeral"})
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	o, e := c.AttachServer(ctx, "syscodex-provider", "PC", "linux")
	if e != nil || o.Server == nil || o.Server.MachineID != "pc" {
		t.Fatal("attach failed", e)
	}
	<-done
}
