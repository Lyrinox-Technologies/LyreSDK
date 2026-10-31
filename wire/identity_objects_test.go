package wire

import (
	"bytes"
	"github.com/Lyrinox-Technologies/ridged-proto/rdgproto"
	"reflect"
	"testing"
	"time"
)

func TestIdentityNativeCodec(t *testing.T) {
	p := IdentityObjectRequestPayload{Action: "attach", Service: "syscodex-provider", Kind: "server", ExternalID: "pc", Name: "PC", Metadata: map[string]any{"label": "desk"}, Server: &IdentityServer{MachineID: "pc", Platform: "linux"}}
	raw, e := p.Marshal()
	if e != nil {
		t.Fatal(e)
	}
	// Action is a native RDGProto length-prefixed string, not a JSON envelope.
	r := bytes.NewReader(raw)
	action, e := rdgproto.ReadString(r)
	if e != nil || action != "attach" {
		t.Fatal("not native RDGProto")
	}
	var got IdentityObjectRequestPayload
	if e = got.Unmarshal(raw); e != nil || !reflect.DeepEqual(p, got) {
		t.Fatal("request mismatch", e)
	}
	if got.Unmarshal(append(raw, 0)) == nil {
		t.Fatal("trailing bytes allowed")
	}
	response := IdentityObjectResponsePayload{Success: true, Objects: []IdentityObject{{ID: "id", Owner: "alice", Service: "syscodex-provider", Kind: "server", Server: p.Server, Metadata: p.Metadata, Updated: time.Unix(123, 0).UTC()}}}
	raw, e = response.Marshal()
	if e != nil {
		t.Fatal(e)
	}
	var answer IdentityObjectResponsePayload
	if e = answer.Unmarshal(raw); e != nil || !reflect.DeepEqual(answer, response) {
		t.Fatal("response mismatch", e)
	}
	for i := 0; i < len(raw); i++ {
		if answer.Unmarshal(raw[:i]) == nil {
			t.Fatalf("truncated response %d accepted", i)
		}
	}
	var malicious bytes.Buffer
	_ = rdgproto.WriteVarint(&malicious, 1<<29)
	if _, e = readMetadata(bytes.NewReader(malicious.Bytes())); e == nil {
		t.Fatal("unbounded metadata allocation")
	}
	response.Objects = make([]IdentityObject, 101)
	if _, e = response.Marshal(); e == nil {
		t.Fatal("unbounded page")
	}
}
