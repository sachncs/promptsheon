package llm

// ProviderName is the canonical identifier for a registered LLM
// provider. The string values are stable; downstream code can
// switch on them without parsing free-form strings.
type ProviderName string

const (
	// ProviderOpenAI is the OpenAI provider identifier.
	ProviderOpenAI ProviderName = "openai"
	// ProviderAnthropic is the Anthropic provider identifier.
	ProviderAnthropic ProviderName = "anthropic"
)

// KnownProviders returns the closed set of providers the daemon
// ships with. Embedders that want to add more should call
// Registry.Register from their own code rather than mutating this
// list.
func KnownProviders() []ProviderName {
	return []ProviderName{ProviderOpenAI, ProviderAnthropic}
}

// String returns the underlying string form so ProviderName
// satisfies fmt.Stringer and can be used directly as a map key
// without conversion.
func (n ProviderName) String() string { return string(n) }
