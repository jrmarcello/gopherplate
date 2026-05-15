package config

// DefaultSecretPatterns is the canonical set of regex patterns used by the
// sanitize layer to redact secrets from prompts and responses (REQ-4).
var DefaultSecretPatterns = []SecretPattern{
	{Kind: "aws_key", Pattern: `AKIA[0-9A-Z]{16}`},
	{Kind: "token", Pattern: `sk-(ant|proj)?-?[A-Za-z0-9_-]{20,}`},
	{Kind: "token", Pattern: `ghp_[A-Za-z0-9]{36}`},
	{Kind: "token", Pattern: `github_pat_[A-Za-z0-9_]{82}`},
	{Kind: "token", Pattern: `xox[abprs]-[A-Za-z0-9-]+`},
	{Kind: "ssh_path", Pattern: `/\.ssh/[A-Za-z0-9_.-]+`},
	{Kind: "env_value", Pattern: `^[A-Z][A-Z0-9_]*=.+$`},
}

// DefaultConfig returns a Config populated with the canonical defaults
// described in REQ-24 of the learning-loop-harness spec.
func DefaultConfig() *Config {
	patterns := make([]SecretPattern, len(DefaultSecretPatterns))
	copy(patterns, DefaultSecretPatterns)
	return &Config{
		MinPatternFreq:        3,
		SimilarityThreshold:   0.6,
		NudgeThreshold:        5,
		TTLDays:               90,
		RecallMinScore:        0.4,
		MinPromptLenForRecall: 20,
		RecallTopK:            3,
		RecallMaxTokens:       500,
		RecallTimeoutSeconds:  2,
		LLMModel:              "",
		SecretPatterns:        patterns,
	}
}
