package wire

type Extension struct {
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	Description      string                 `json:"description"`
	CallerSchema     map[string]interface{} `json:"caller_schema,omitempty"`
	ProviderEndpoint string                 `json:"provider_endpoint,omitempty"`
	RequestFields    map[string]string      `json:"request_fields,omitempty"`
	RequestDefaults  map[string]interface{} `json:"request_defaults,omitempty"`
	ResponseFields   map[string]string      `json:"response_fields,omitempty"`
	Errors           []ExtensionError       `json:"errors,omitempty"`
	Execution        Execution              `json:"execution,omitempty"`
}
type ExtensionError struct {
	ProviderCode string `json:"provider_code"`
	CallerCode   string `json:"caller_code"`
	Retryable    bool   `json:"retryable"`
	Description  string `json:"description"`
}
type Execution struct {
	Idempotent bool `json:"idempotent"`
	Retryable  bool `json:"retryable"`
}
