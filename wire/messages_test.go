package wire

import (
	"bytes"
	"testing"

	"github.com/Lyrinox-Technologies/ridged-proto/rdgproto"
)

func TestAuthRequestSupportsLegacyPayloads(t *testing.T) {
	legacy := new(bytes.Buffer)
	legacy.WriteByte(byte(AuthTypeStandard))
	rdgproto.WriteString(legacy, "lyrinox")
	rdgproto.WriteString(legacy, "password")
	rdgproto.WriteString(legacy, "")
	rdgproto.WriteString(legacy, "")
	rdgproto.WriteString(legacy, "")

	var request AuthRequestPayload
	if err := request.Unmarshal(legacy.Bytes()); err != nil {
		t.Fatalf("unmarshal legacy auth request: %v", err)
	}
	if request.ServiceID != "" {
		t.Fatalf("legacy request set service id %q", request.ServiceID)
	}
}

func TestServiceCredentialPayloadRoundTrip(t *testing.T) {
	payload := &ServiceCredentialSetPayload{ServiceID: "lyrinox-market", Password: "independent-password"}
	encoded, err := payload.Marshal()
	if err != nil {
		t.Fatalf("marshal service credential: %v", err)
	}

	var decoded ServiceCredentialSetPayload
	if err := decoded.Unmarshal(encoded); err != nil {
		t.Fatalf("unmarshal service credential: %v", err)
	}
	if decoded != *payload {
		t.Fatalf("round trip mismatch: %#v", decoded)
	}
}

func TestPasswordPayloadRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		payload rdgproto.PayloadMarshaler
		decoded rdgproto.PayloadUnmarshaler
	}{
		{"reset request", &PasswordResetRequestPayload{Identifier: "user@example.test"}, &PasswordResetRequestPayload{}},
		{"reset confirm", &PasswordResetConfirmPayload{Token: "opaque-token", Password: "Password123!"}, &PasswordResetConfirmPayload{}},
		{"password change", &PasswordChangeRequestPayload{CurrentPassword: "OldPassword123!", NewPassword: "NewPassword123!"}, &PasswordChangeRequestPayload{}},
		{"password response", &PasswordChangeResponsePayload{Success: true, Message: "updated"}, &PasswordChangeResponsePayload{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := test.payload.Marshal()
			if err != nil {
				t.Fatal(err)
			}
			if err := test.decoded.Unmarshal(encoded); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAdminPayloadSupportsRoleAssignments(t *testing.T) {
	payload := &AdminRequestPayload{Action: AdminActionSetRole, UserID: "user-123", Role: "administrator"}
	encoded, err := payload.Marshal()
	if err != nil {
		t.Fatalf("marshal administrator request: %v", err)
	}

	var decoded AdminRequestPayload
	if err := decoded.Unmarshal(encoded); err != nil {
		t.Fatalf("unmarshal administrator request: %v", err)
	}
	if decoded != *payload {
		t.Fatalf("round trip mismatch: %#v", decoded)
	}
}

func TestAgentRegistrationPayloadRoundTrip(t *testing.T) {
	payload := &AgentRegisterRequestPayload{Name: "release-bot", Description: "Publishes preview builds"}
	encoded, err := payload.Marshal()
	if err != nil {
		t.Fatalf("marshal agent registration: %v", err)
	}

	var decoded AgentRegisterRequestPayload
	if err := decoded.Unmarshal(encoded); err != nil {
		t.Fatalf("unmarshal agent registration: %v", err)
	}
	if decoded != *payload {
		t.Fatalf("round trip mismatch: %#v", decoded)
	}
}

func TestServiceAuthPayloadCapabilityRoundTrip(t *testing.T) {
	payload := &ServiceAuthPayload{
		ServiceID: "crypto-service",
		Secret:    "a-long-service-secret",
		Endpoints: []string{"internal.encrypt"},
		Capabilities: []ServiceCapability{{
			Name: "crypto.encrypt", Version: 1, Endpoint: "internal.encrypt",
			InputSchema: `{"type":"object"}`, OutputSchema: `{"type":"object"}`, Priority: 10,
		}},
	}
	encoded, err := payload.Marshal()
	if err != nil {
		t.Fatalf("marshal service auth: %v", err)
	}
	var decoded ServiceAuthPayload
	if err := decoded.Unmarshal(encoded); err != nil {
		t.Fatalf("unmarshal service auth: %v", err)
	}
	if len(decoded.Capabilities) != 1 || decoded.Capabilities[0].Name != payload.Capabilities[0].Name || decoded.Capabilities[0].Version != payload.Capabilities[0].Version || decoded.Capabilities[0].Endpoint != payload.Capabilities[0].Endpoint {
		t.Fatalf("capability round trip mismatch: %#v", decoded.Capabilities)
	}
}
