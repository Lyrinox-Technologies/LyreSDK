package wire

// ProviderExtension is a declarative adapter for a provider capability. It is
// stored and transmitted as metadata only; Lyre never executes adapter code.
type ProviderExtension struct {
	ID               string                   `json:"id"`
	Name             string                   `json:"name"`
	Description      string                   `json:"description"`
	CallerSchema     map[string]interface{}   `json:"caller_schema,omitempty"`
	ProviderEndpoint string                   `json:"provider_endpoint,omitempty"`
	RequestFields    map[string]string        `json:"request_fields,omitempty"`
	RequestDefaults  map[string]interface{}   `json:"request_defaults,omitempty"`
	ResponseFields   map[string]string        `json:"response_fields,omitempty"`
	Errors           []ProviderExtensionError `json:"errors,omitempty"`
	Execution        Execution                `json:"execution,omitempty"`
}

// ProviderExtensionError maps a provider-private error code to a caller-facing
// code for a ProviderExtension.
type ProviderExtensionError struct {
	ProviderCode string `json:"provider_code"`
	CallerCode   string `json:"caller_code"`
	Retryable    bool   `json:"retryable"`
	Description  string `json:"description"`
}

// Extension is a deprecated source-compatible alias for ProviderExtension.
// Deprecated: use ProviderExtension.
type Extension = ProviderExtension

// ExtensionError is a deprecated source-compatible alias for
// ProviderExtensionError.
// Deprecated: use ProviderExtensionError.
type ExtensionError = ProviderExtensionError

type Execution struct {
	Idempotent bool `json:"idempotent"`
	Retryable  bool `json:"retryable"`
}
