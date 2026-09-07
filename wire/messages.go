// Package wire defines rdgproto message types and payloads for Lyre-Server.
package wire

import (
	"bytes"
	"fmt"

	"github.com/LyrinoxTechnologies/ridged-proto/rdgproto"
)

// Reserved message types for authentication (1-20)
const (
	// Authentication messages
	MsgTypeAuthRequest        byte = 1  // Initial auth request (standard or machine)
	MsgTypeAuthResponse       byte = 2  // Auth response with token
	MsgTypeAuthMFARequired    byte = 3  // MFA required response
	MsgTypeAuthMFASubmit      byte = 4  // MFA code submission
	MsgTypeAuthMFAResponse    byte = 5  // MFA verification result
	MsgTypeAuthRefresh        byte = 6  // Token refresh (for standard auth)
	MsgTypeAuthLogout         byte = 7  // Logout request
	MsgTypeAuthLogoutResponse byte = 8  // Logout confirmation
	MsgTypeNewDeviceMFA       byte = 9  // New device requires email MFA
	MsgTypeNewDeviceMFASubmit byte = 10 // New device MFA code submission

	// Session messages
	MsgTypeSessionValidate byte = 11 // Validate current session
	MsgTypeSessionInvalid  byte = 12 // Session is invalid

	// Registration messages
	MsgTypeRegisterRequest     byte = 13 // Account registration request
	MsgTypeRegisterResponse    byte = 14 // Registration response
	MsgTypeVerifyEmailRequest  byte = 15 // Email verification request
	MsgTypeVerifyEmailResponse byte = 16 // Email verification response
	MsgTypeResendVerification  byte = 17 // Resend verification email

	// TOTP MFA setup messages
	MsgTypeTOTPSetupRequest  byte = 18 // Request TOTP setup (get secret/QR)
	MsgTypeTOTPSetupResponse byte = 19 // TOTP setup response with secret

	// Error messages
	MsgTypeError byte = 20 // Generic error response

	// App authentication (reserved 250-254)
	MsgTypeAppChallenge     byte = 250 // Server sends challenge to app
	MsgTypeAppChallengeResp byte = 251 // App signs challenge with private key
	MsgTypeAppAuthSuccess   byte = 252 // App authentication successful
	MsgTypeAppAuthFailed    byte = 253 // App authentication failed

	// Service communication (reserved 240-249)
	MsgTypeServiceAuth         byte = 240 // Service authentication
	MsgTypeServiceAuthResponse byte = 241 // Service auth response
	MsgTypeServiceHeartbeat    byte = 242 // Service heartbeat
	MsgTypeServiceMessage      byte = 243 // Inter-service message
	MsgTypeServiceResponse     byte = 244 // Service response
	MsgTypeServiceList         byte = 245 // List available services
	MsgTypeServiceListResponse byte = 246 // Service list response
	MsgTypeClientToService     byte = 247 // Client -> Service message (via Lyre)
	MsgTypeServiceToClient     byte = 248 // Service -> Client message (via Lyre)
)

// Custom message types start at 21 and go up to 249
const (
	CustomMessageTypeStart byte = 21
	CustomMessageTypeEnd   byte = 249
)

// Built-in custom message types for testing/utility (21-30)
const (
	MsgTypePing                      byte = 21 // Ping to a specific user
	MsgTypePong                      byte = 22 // Pong response
	MsgTypeBroadcast                 byte = 23 // Broadcast message to all connections of a user
	MsgTypeDirectMessage             byte = 24 // Direct message to a specific user
	MsgTypeServiceCredentialSet      byte = 25 // Set or remove a service-specific password
	MsgTypeServiceCredentialResponse byte = 26 // Service credential update result
	MsgTypeAdminRequest              byte = 27 // Administrator status, user list, and user updates
	MsgTypeAdminResponse             byte = 28 // Administrator operation response
	MsgTypeAgentRegisterRequest      byte = 29 // Independent agent registration
	MsgTypeAgentRegisterResponse     byte = 30 // One-time agent API key response
	MsgTypeResourceAccessRequest     byte = 31 // Claim or delegate a scoped service role
	MsgTypeResourceAccessResponse    byte = 32 // Scoped access operation result
	MsgTypePasswordResetRequest      byte = 33 // Request a one-time password reset link
	MsgTypePasswordResetConfirm      byte = 34 // Consume a reset token and set a password
	MsgTypePasswordChangeRequest     byte = 35 // Change an authenticated user's shared password
	MsgTypePasswordChangeResponse    byte = 36 // Password reset/change result
	MsgTypePublisherKeySet           byte = 37 // Add or replace a user publisher public key
	MsgTypePublisherKeyResponse      byte = 38 // Publisher key operation result
	MsgTypeOrganizationRequest       byte = 39 // Authenticated organization operations
	MsgTypeOrganizationResponse      byte = 40
)

// AuthType indicates the type of authentication.
type AuthType byte

const (
	AuthTypeStandard AuthType = 1 // Standard web auth (username/password/MFA)
	AuthTypeMachine  AuthType = 2 // Machine-bound auth (username/password/reMachID/eMachID)
	AuthTypeAgent    AuthType = 3 // Non-human agent auth (agent API key)
)

// ErrorCode represents specific error conditions.
type ErrorCode uint32

const (
	ErrCodeUnknown              ErrorCode = 0
	ErrCodeInvalidCredentials   ErrorCode = 1
	ErrCodeAccountLocked        ErrorCode = 2
	ErrCodeMFARequired          ErrorCode = 3
	ErrCodeMFAInvalid           ErrorCode = 4
	ErrCodeTokenExpired         ErrorCode = 5
	ErrCodeTokenInvalid         ErrorCode = 6
	ErrCodeSessionInvalid       ErrorCode = 7
	ErrCodeMachineIDMismatch    ErrorCode = 8
	ErrCodeStolenEMachID        ErrorCode = 9
	ErrCodeNewDeviceRequiresMFA ErrorCode = 10
	ErrCodeUsernameExists       ErrorCode = 11
	ErrCodeEmailExists          ErrorCode = 12
	ErrCodeWeakPassword         ErrorCode = 13
	ErrCodeInvalidEmail         ErrorCode = 14
	ErrCodeAccountNotActive     ErrorCode = 15
	ErrCodeVerificationExpired  ErrorCode = 16
	ErrCodeInvalidVerification  ErrorCode = 17
	ErrCodeInternalError        ErrorCode = 99
)

// AuthRequestPayload is sent by clients to authenticate.
type AuthRequestPayload struct {
	AuthType AuthType
	Username string
	Password string
	// For machine auth only
	ReMachID string
	EMachID  string
	// For subsequent requests (token auth)
	Token string
	// ServiceID is optional. When supplied, Lyre will use the user's
	// service-specific password for that registered service if one exists.
	ServiceID string
}

func (p *AuthRequestPayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(p.AuthType))
	rdgproto.WriteString(buf, p.Username)
	rdgproto.WriteString(buf, p.Password)
	rdgproto.WriteString(buf, p.ReMachID)
	rdgproto.WriteString(buf, p.EMachID)
	rdgproto.WriteString(buf, p.Token)
	rdgproto.WriteString(buf, p.ServiceID)
	return buf.Bytes(), nil
}

func (p *AuthRequestPayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	authType, err := r.ReadByte()
	if err != nil {
		return err
	}
	p.AuthType = AuthType(authType)
	p.Username, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.Password, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.ReMachID, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.EMachID, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.Token, err = rdgproto.ReadString(r)
	if err != nil || r.Len() == 0 {
		return err
	}
	p.ServiceID, err = rdgproto.ReadString(r)
	return err
}

// ServiceCredentialSetPayload lets an authenticated user choose a password
// for one registered Lyre service. An empty password removes the override.
type ServiceCredentialSetPayload struct {
	ServiceID string
	Password  string
}

func (p *ServiceCredentialSetPayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteString(buf, p.ServiceID)
	rdgproto.WriteString(buf, p.Password)
	return buf.Bytes(), nil
}

func (p *ServiceCredentialSetPayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	var err error
	p.ServiceID, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.Password, err = rdgproto.ReadString(r)
	return err
}

type ServiceCredentialResponsePayload struct {
	Success   bool
	ServiceID string
	Enabled   bool
	Message   string
}

// PasswordResetRequestPayload accepts a username or email address. Responses
// intentionally do not reveal whether the account exists.
type PasswordResetRequestPayload struct{ Identifier string }

func (p *PasswordResetRequestPayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteString(buf, p.Identifier)
	return buf.Bytes(), nil
}

func (p *PasswordResetRequestPayload) Unmarshal(data []byte) error {
	value, err := rdgproto.ReadString(bytes.NewReader(data))
	p.Identifier = value
	return err
}

// PasswordResetConfirmPayload consumes an opaque, one-time reset token.
type PasswordResetConfirmPayload struct {
	Token    string
	Password string
}

func (p *PasswordResetConfirmPayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteString(buf, p.Token)
	rdgproto.WriteString(buf, p.Password)
	return buf.Bytes(), nil
}

func (p *PasswordResetConfirmPayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	var err error
	p.Token, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.Password, err = rdgproto.ReadString(r)
	return err
}

// PasswordChangeRequestPayload requires the current shared password for an
// already authenticated user. Service-specific credentials are unaffected.
type PasswordChangeRequestPayload struct {
	CurrentPassword string
	NewPassword     string
}

func (p *PasswordChangeRequestPayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteString(buf, p.CurrentPassword)
	rdgproto.WriteString(buf, p.NewPassword)
	return buf.Bytes(), nil
}

func (p *PasswordChangeRequestPayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	var err error
	p.CurrentPassword, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.NewPassword, err = rdgproto.ReadString(r)
	return err
}

type PasswordChangeResponsePayload struct {
	Success bool
	Message string
}

func (p *PasswordChangeResponsePayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteBool(buf, p.Success)
	rdgproto.WriteString(buf, p.Message)
	return buf.Bytes(), nil
}

func (p *PasswordChangeResponsePayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	var err error
	p.Success, err = rdgproto.ReadBool(r)
	if err != nil {
		return err
	}
	p.Message, err = rdgproto.ReadString(r)
	return err
}

type PublisherKeySetPayload struct {
	Name      string
	PublicKey string
}

func (p *PublisherKeySetPayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteString(buf, p.Name)
	rdgproto.WriteString(buf, p.PublicKey)
	return buf.Bytes(), nil
}

func (p *PublisherKeySetPayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	var err error
	p.Name, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.PublicKey, err = rdgproto.ReadString(r)
	return err
}

type PublisherKeyResponsePayload struct {
	Success     bool
	Fingerprint string
	Message     string
}

func (p *PublisherKeyResponsePayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteBool(buf, p.Success)
	rdgproto.WriteString(buf, p.Fingerprint)
	rdgproto.WriteString(buf, p.Message)
	return buf.Bytes(), nil
}

func (p *PublisherKeyResponsePayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	var err error
	p.Success, err = rdgproto.ReadBool(r)
	if err != nil {
		return err
	}
	p.Fingerprint, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.Message, err = rdgproto.ReadString(r)
	return err
}

func (p *ServiceCredentialResponsePayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteBool(buf, p.Success)
	rdgproto.WriteString(buf, p.ServiceID)
	rdgproto.WriteBool(buf, p.Enabled)
	rdgproto.WriteString(buf, p.Message)
	return buf.Bytes(), nil
}

func (p *ServiceCredentialResponsePayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	var err error
	p.Success, err = rdgproto.ReadBool(r)
	if err != nil {
		return err
	}
	p.ServiceID, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.Enabled, err = rdgproto.ReadBool(r)
	if err != nil {
		return err
	}
	p.Message, err = rdgproto.ReadString(r)
	return err
}

// AuthResponsePayload is sent by the server after successful authentication.
type AuthResponsePayload struct {
	Success   bool
	Token     string // New single-use token
	UserID    string
	ExpiresAt int64 // Unix timestamp
	Message   string
}

func (p *AuthResponsePayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteBool(buf, p.Success)
	rdgproto.WriteString(buf, p.Token)
	rdgproto.WriteString(buf, p.UserID)
	rdgproto.WriteUint64(buf, uint64(p.ExpiresAt))
	rdgproto.WriteString(buf, p.Message)
	return buf.Bytes(), nil
}

func (p *AuthResponsePayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	var err error
	p.Success, err = rdgproto.ReadBool(r)
	if err != nil {
		return err
	}
	p.Token, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.UserID, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	expiresAt, err := rdgproto.ReadUint64(r)
	if err != nil {
		return err
	}
	p.ExpiresAt = int64(expiresAt)
	p.Message, err = rdgproto.ReadString(r)
	return err
}

// MFARequiredPayload indicates MFA is needed.
type MFARequiredPayload struct {
	MFAType     string // "totp", "email"
	Hint        string // e.g., masked email "j***@example.com"
	ChallengeID string // Server-side challenge identifier
}

func (p *MFARequiredPayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteString(buf, p.MFAType)
	rdgproto.WriteString(buf, p.Hint)
	rdgproto.WriteString(buf, p.ChallengeID)
	return buf.Bytes(), nil
}

func (p *MFARequiredPayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	var err error
	p.MFAType, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.Hint, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.ChallengeID, err = rdgproto.ReadString(r)
	return err
}

// MFASubmitPayload is sent by clients to submit MFA code.
type MFASubmitPayload struct {
	ChallengeID string
	Code        string
}

func (p *MFASubmitPayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteString(buf, p.ChallengeID)
	rdgproto.WriteString(buf, p.Code)
	return buf.Bytes(), nil
}

func (p *MFASubmitPayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	var err error
	p.ChallengeID, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.Code, err = rdgproto.ReadString(r)
	return err
}

// SessionValidatePayload is sent with each message for machine auth.
type SessionValidatePayload struct {
	Token    string
	ReMachID string
	EMachID  string
}

func (p *SessionValidatePayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteString(buf, p.Token)
	rdgproto.WriteString(buf, p.ReMachID)
	rdgproto.WriteString(buf, p.EMachID)
	return buf.Bytes(), nil
}

func (p *SessionValidatePayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	var err error
	p.Token, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.ReMachID, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.EMachID, err = rdgproto.ReadString(r)
	return err
}

// ErrorPayload is sent when an error occurs.
type ErrorPayload struct {
	Code    ErrorCode
	Message string
}

func (p *ErrorPayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteUint32(buf, uint32(p.Code))
	rdgproto.WriteString(buf, p.Message)
	return buf.Bytes(), nil
}

func (p *ErrorPayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	code, err := rdgproto.ReadUint32(r)
	if err != nil {
		return err
	}
	p.Code = ErrorCode(code)
	p.Message, err = rdgproto.ReadString(r)
	return err
}

// LogoutPayload is sent to terminate a session.
type LogoutPayload struct {
	Token string
}

func (p *LogoutPayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteString(buf, p.Token)
	return buf.Bytes(), nil
}

func (p *LogoutPayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	var err error
	p.Token, err = rdgproto.ReadString(r)
	return err
}

// LogoutResponsePayload confirms logout.
type LogoutResponsePayload struct {
	Success bool
	Message string
}

func (p *LogoutResponsePayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteBool(buf, p.Success)
	rdgproto.WriteString(buf, p.Message)
	return buf.Bytes(), nil
}

func (p *LogoutResponsePayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	var err error
	p.Success, err = rdgproto.ReadBool(r)
	if err != nil {
		return err
	}
	p.Message, err = rdgproto.ReadString(r)
	return err
}

// NewDeviceMFAPayload indicates a new device needs email verification.
type NewDeviceMFAPayload struct {
	Email       string // Masked email address
	ChallengeID string
	DeviceInfo  string // Info about the new device
}

func (p *NewDeviceMFAPayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteString(buf, p.Email)
	rdgproto.WriteString(buf, p.ChallengeID)
	rdgproto.WriteString(buf, p.DeviceInfo)
	return buf.Bytes(), nil
}

func (p *NewDeviceMFAPayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	var err error
	p.Email, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.ChallengeID, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.DeviceInfo, err = rdgproto.ReadString(r)
	return err
}

// RegisterRequestPayload is sent to register a new account.
type RegisterRequestPayload struct {
	Username string
	Email    string
	Password string
	RealName string
	Country  string
}

func (p *RegisterRequestPayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteString(buf, p.Username)
	rdgproto.WriteString(buf, p.Email)
	rdgproto.WriteString(buf, p.Password)
	rdgproto.WriteString(buf, p.RealName)
	rdgproto.WriteString(buf, p.Country)
	return buf.Bytes(), nil
}

func (p *RegisterRequestPayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	var err error
	p.Username, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.Email, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.Password, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.RealName, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.Country, err = rdgproto.ReadString(r)
	return err
}

// RegisterResponsePayload is sent after registration attempt.
type RegisterResponsePayload struct {
	Success bool
	UserID  string
	Message string // e.g., "Verification email sent to j***@example.com"
}

func (p *RegisterResponsePayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteBool(buf, p.Success)
	rdgproto.WriteString(buf, p.UserID)
	rdgproto.WriteString(buf, p.Message)
	return buf.Bytes(), nil
}

func (p *RegisterResponsePayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	var err error
	p.Success, err = rdgproto.ReadBool(r)
	if err != nil {
		return err
	}
	p.UserID, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.Message, err = rdgproto.ReadString(r)
	return err
}

// VerifyEmailRequestPayload is sent to verify an email.
type VerifyEmailRequestPayload struct {
	Token string // Verification token from email link
}

func (p *VerifyEmailRequestPayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteString(buf, p.Token)
	return buf.Bytes(), nil
}

func (p *VerifyEmailRequestPayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	var err error
	p.Token, err = rdgproto.ReadString(r)
	return err
}

// VerifyEmailResponsePayload is sent after email verification.
type VerifyEmailResponsePayload struct {
	Success  bool
	Username string
	Message  string
}

func (p *VerifyEmailResponsePayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteBool(buf, p.Success)
	rdgproto.WriteString(buf, p.Username)
	rdgproto.WriteString(buf, p.Message)
	return buf.Bytes(), nil
}

func (p *VerifyEmailResponsePayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	var err error
	p.Success, err = rdgproto.ReadBool(r)
	if err != nil {
		return err
	}
	p.Username, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.Message, err = rdgproto.ReadString(r)
	return err
}

// ResendVerificationPayload is sent to resend verification email.
type ResendVerificationPayload struct {
	Email string
}

func (p *ResendVerificationPayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteString(buf, p.Email)
	return buf.Bytes(), nil
}

func (p *ResendVerificationPayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	var err error
	p.Email, err = rdgproto.ReadString(r)
	return err
}

// TOTPSetupRequestPayload is sent to request TOTP setup.
type TOTPSetupRequestPayload struct {
	Action string // "setup" = get new secret, "verify" = verify and enable, "disable" = disable TOTP
	Code   string // For "verify" action, the TOTP code to confirm
}

func (p *TOTPSetupRequestPayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteString(buf, p.Action)
	rdgproto.WriteString(buf, p.Code)
	return buf.Bytes(), nil
}

func (p *TOTPSetupRequestPayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	var err error
	p.Action, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.Code, err = rdgproto.ReadString(r)
	return err
}

// TOTPSetupResponsePayload contains TOTP setup information.
type TOTPSetupResponsePayload struct {
	Success         bool
	Secret          string // Base32 encoded secret (only for "setup" action)
	ProvisioningURI string // otpauth:// URI for QR code (only for "setup" action)
	Message         string
	TOTPEnabled     bool // Current TOTP status
}

func (p *TOTPSetupResponsePayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteBool(buf, p.Success)
	rdgproto.WriteString(buf, p.Secret)
	rdgproto.WriteString(buf, p.ProvisioningURI)
	rdgproto.WriteString(buf, p.Message)
	rdgproto.WriteBool(buf, p.TOTPEnabled)
	return buf.Bytes(), nil
}

func (p *TOTPSetupResponsePayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	var err error
	p.Success, err = rdgproto.ReadBool(r)
	if err != nil {
		return err
	}
	p.Secret, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.ProvisioningURI, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.Message, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.TOTPEnabled, err = rdgproto.ReadBool(r)
	return err
}

// PingPayload is sent to ping another user.
type PingPayload struct {
	TargetUser string // Username to ping (empty = broadcast to all own connections)
	Message    string // Optional message
	Timestamp  int64  // Sender's timestamp
}

func (p *PingPayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteString(buf, p.TargetUser)
	rdgproto.WriteString(buf, p.Message)
	rdgproto.WriteUint64(buf, uint64(p.Timestamp))
	return buf.Bytes(), nil
}

func (p *PingPayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	var err error
	p.TargetUser, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.Message, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	ts, err := rdgproto.ReadUint64(r)
	if err != nil {
		return err
	}
	p.Timestamp = int64(ts)
	return nil
}

// PongPayload is the response to a ping.
type PongPayload struct {
	FromUser      string // Username who sent the pong
	Message       string // Optional message
	OrigTimestamp int64  // Original ping timestamp
	Timestamp     int64  // Pong timestamp
}

func (p *PongPayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteString(buf, p.FromUser)
	rdgproto.WriteString(buf, p.Message)
	rdgproto.WriteUint64(buf, uint64(p.OrigTimestamp))
	rdgproto.WriteUint64(buf, uint64(p.Timestamp))
	return buf.Bytes(), nil
}

func (p *PongPayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	var err error
	p.FromUser, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.Message, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	ots, err := rdgproto.ReadUint64(r)
	if err != nil {
		return err
	}
	p.OrigTimestamp = int64(ots)
	ts, err := rdgproto.ReadUint64(r)
	if err != nil {
		return err
	}
	p.Timestamp = int64(ts)
	return nil
}

// DirectMessagePayload is for sending direct messages between users.
type DirectMessagePayload struct {
	TargetUser string // Username to send to
	Message    string // The message content
	Timestamp  int64  // Sender's timestamp
}

func (p *DirectMessagePayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteString(buf, p.TargetUser)
	rdgproto.WriteString(buf, p.Message)
	rdgproto.WriteUint64(buf, uint64(p.Timestamp))
	return buf.Bytes(), nil
}

func (p *DirectMessagePayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	var err error
	p.TargetUser, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	p.Message, err = rdgproto.ReadString(r)
	if err != nil {
		return err
	}
	ts, err := rdgproto.ReadUint64(r)
	if err != nil {
		return err
	}
	p.Timestamp = int64(ts)
	return nil
}

// RegisterPayloadTypes registers all built-in payload types with rdgproto.
func RegisterPayloadTypes(registry *rdgproto.PayloadRegistry) {
	registry.Register(MsgTypeAuthRequest, func() rdgproto.PayloadUnmarshaler {
		return &AuthRequestPayload{}
	})
	registry.Register(MsgTypeAgentRegisterRequest, func() rdgproto.PayloadUnmarshaler {
		return &AgentRegisterRequestPayload{}
	})
	registry.Register(MsgTypeAgentRegisterResponse, func() rdgproto.PayloadUnmarshaler {
		return &AgentRegisterResponsePayload{}
	})
	registry.Register(MsgTypeAuthResponse, func() rdgproto.PayloadUnmarshaler {
		return &AuthResponsePayload{}
	})
	registry.Register(MsgTypeServiceCredentialSet, func() rdgproto.PayloadUnmarshaler {
		return &ServiceCredentialSetPayload{}
	})
	registry.Register(MsgTypeServiceCredentialResponse, func() rdgproto.PayloadUnmarshaler {
		return &ServiceCredentialResponsePayload{}
	})
	registry.Register(MsgTypePasswordResetRequest, func() rdgproto.PayloadUnmarshaler { return &PasswordResetRequestPayload{} })
	registry.Register(MsgTypePasswordResetConfirm, func() rdgproto.PayloadUnmarshaler { return &PasswordResetConfirmPayload{} })
	registry.Register(MsgTypePasswordChangeRequest, func() rdgproto.PayloadUnmarshaler { return &PasswordChangeRequestPayload{} })
	registry.Register(MsgTypePasswordChangeResponse, func() rdgproto.PayloadUnmarshaler { return &PasswordChangeResponsePayload{} })
	registry.Register(MsgTypePublisherKeySet, func() rdgproto.PayloadUnmarshaler { return &PublisherKeySetPayload{} })
	registry.Register(MsgTypePublisherKeyResponse, func() rdgproto.PayloadUnmarshaler { return &PublisherKeyResponsePayload{} })
	registry.Register(MsgTypeOrganizationRequest, func() rdgproto.PayloadUnmarshaler { return &OrganizationRequestPayload{} })
	registry.Register(MsgTypeOrganizationResponse, func() rdgproto.PayloadUnmarshaler { return &OrganizationResponsePayload{} })
	registry.Register(MsgTypeAdminRequest, func() rdgproto.PayloadUnmarshaler {
		return &AdminRequestPayload{}
	})
	registry.Register(MsgTypeAdminResponse, func() rdgproto.PayloadUnmarshaler {
		return &AdminResponsePayload{}
	})
	registry.Register(MsgTypeAuthMFARequired, func() rdgproto.PayloadUnmarshaler {
		return &MFARequiredPayload{}
	})
	registry.Register(MsgTypeAuthMFASubmit, func() rdgproto.PayloadUnmarshaler {
		return &MFASubmitPayload{}
	})
	registry.Register(MsgTypeAuthMFAResponse, func() rdgproto.PayloadUnmarshaler {
		return &AuthResponsePayload{} // Same as auth response
	})
	registry.Register(MsgTypeAuthLogout, func() rdgproto.PayloadUnmarshaler {
		return &LogoutPayload{}
	})
	registry.Register(MsgTypeAuthLogoutResponse, func() rdgproto.PayloadUnmarshaler {
		return &LogoutResponsePayload{}
	})
	registry.Register(MsgTypeNewDeviceMFA, func() rdgproto.PayloadUnmarshaler {
		return &NewDeviceMFAPayload{}
	})
	registry.Register(MsgTypeNewDeviceMFASubmit, func() rdgproto.PayloadUnmarshaler {
		return &MFASubmitPayload{} // Same as MFA submit
	})
	registry.Register(MsgTypeSessionValidate, func() rdgproto.PayloadUnmarshaler {
		return &SessionValidatePayload{}
	})
	registry.Register(MsgTypeError, func() rdgproto.PayloadUnmarshaler {
		return &ErrorPayload{}
	})
	// Registration messages
	registry.Register(MsgTypeRegisterRequest, func() rdgproto.PayloadUnmarshaler {
		return &RegisterRequestPayload{}
	})
	registry.Register(MsgTypeRegisterResponse, func() rdgproto.PayloadUnmarshaler {
		return &RegisterResponsePayload{}
	})
	registry.Register(MsgTypeVerifyEmailRequest, func() rdgproto.PayloadUnmarshaler {
		return &VerifyEmailRequestPayload{}
	})
	registry.Register(MsgTypeVerifyEmailResponse, func() rdgproto.PayloadUnmarshaler {
		return &VerifyEmailResponsePayload{}
	})
	registry.Register(MsgTypeResendVerification, func() rdgproto.PayloadUnmarshaler {
		return &ResendVerificationPayload{}
	})
	// TOTP setup messages
	registry.Register(MsgTypeTOTPSetupRequest, func() rdgproto.PayloadUnmarshaler {
		return &TOTPSetupRequestPayload{}
	})
	registry.Register(MsgTypeTOTPSetupResponse, func() rdgproto.PayloadUnmarshaler {
		return &TOTPSetupResponsePayload{}
	})
	// Ping/Pong messages
	registry.Register(MsgTypePing, func() rdgproto.PayloadUnmarshaler {
		return &PingPayload{}
	})
	registry.Register(MsgTypePong, func() rdgproto.PayloadUnmarshaler {
		return &PongPayload{}
	})
	registry.Register(MsgTypeDirectMessage, func() rdgproto.PayloadUnmarshaler {
		return &DirectMessagePayload{}
	})
	registry.Register(MsgTypeResourceAccessRequest, func() rdgproto.PayloadUnmarshaler { return &ResourceAccessRequestPayload{} })
	registry.Register(MsgTypeResourceAccessResponse, func() rdgproto.PayloadUnmarshaler { return &ResourceAccessResponsePayload{} })
}

// ErrorCodeString returns a human-readable error code description.
func ErrorCodeString(code ErrorCode) string {
	switch code {
	case ErrCodeInvalidCredentials:
		return "Invalid credentials"
	case ErrCodeAccountLocked:
		return "Account locked"
	case ErrCodeMFARequired:
		return "MFA required"
	case ErrCodeMFAInvalid:
		return "Invalid MFA code"
	case ErrCodeTokenExpired:
		return "Token expired"
	case ErrCodeTokenInvalid:
		return "Invalid token"
	case ErrCodeSessionInvalid:
		return "Session invalid"
	case ErrCodeMachineIDMismatch:
		return "Machine ID mismatch - re-authentication required"
	case ErrCodeStolenEMachID:
		return "Authentication rejected - security violation"
	case ErrCodeNewDeviceRequiresMFA:
		return "New device requires email verification"
	case ErrCodeInternalError:
		return "Internal server error"
	default:
		return fmt.Sprintf("Unknown error (%d)", code)
	}
}
