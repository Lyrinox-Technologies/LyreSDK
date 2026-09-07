package wire

import (
	"encoding/json"
	"testing"
)

func TestOrganizationResponseUsesPortalJSONFields(t *testing.T) {
	payload := &OrganizationResponsePayload{Success: true, Message: "ok", Data: `{"organizations":[]}`}
	raw, err := payload.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	var decoded OrganizationResponsePayload
	if err := decoded.Unmarshal(raw); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(decoded)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]interface{}
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	if fields["success"] != true || fields["message"] != "ok" || fields["data"] != `{"organizations":[]}` {
		t.Fatalf("unexpected JSON fields: %s", encoded)
	}
}
