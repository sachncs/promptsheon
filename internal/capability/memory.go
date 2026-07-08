package capability

// MemoryConfig defines how a capability retains and recalls information
// across executions. Memory is separate from knowledge — memory captures
// conversational and working state, while knowledge captures reference
// information.
type MemoryConfig struct {
	SessionMemory      bool `json:"session_memory"`
	ConversationMemory bool `json:"conversation_memory"`
	WorkingMemory      bool `json:"working_memory"`
	LongTermMemory     bool `json:"long_term_memory"`
	SharedMemory       bool `json:"shared_memory"`
	MaxSessionTokens   int  `json:"max_session_tokens,omitempty"`
}
