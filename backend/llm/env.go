package llm

import "os"

// envJudgeProvider returns the provider name configured via
// PROMPTSHEON_LLM_JUDGE_PROVIDER. Empty string means "use the
// first registered provider".
func envJudgeProvider() string {
	return os.Getenv("PROMPTSHEON_LLM_JUDGE_PROVIDER")
}
