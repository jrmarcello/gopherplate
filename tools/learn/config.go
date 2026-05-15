package learn

// Loads the learning-loop runtime configuration from a YAML file with
// environment-variable overrides. Canonical defaults are colocated below in
// defaultLoopConfig / defaultSecretPatterns; Load merges file content over
// defaults, then applies LEARN_* env overrides, and finally validates the
// resulting struct.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// loopConfig holds all tunable knobs of the learning loop (REQ-24).
type loopConfig struct {
	MinPatternFreq        int             `yaml:"min_pattern_freq"`
	SimilarityThreshold   float64         `yaml:"similarity_threshold"`
	NudgeThreshold        int             `yaml:"nudge_threshold"`
	TTLDays               int             `yaml:"ttl_days"`
	RecallMinScore        float64         `yaml:"recall_min_score"`
	MinPromptLenForRecall int             `yaml:"min_prompt_len_for_recall"`
	RecallTopK            int             `yaml:"recall_top_k"`
	RecallMaxTokens       int             `yaml:"recall_max_tokens"`
	RecallTimeoutSeconds  int             `yaml:"recall_timeout_seconds"`
	LLMModel              string          `yaml:"llm_model"`
	SecretPatterns        []secretPattern `yaml:"secret_patterns"`
}

// secretPattern is one RE2 regex used by the sanitize layer (REQ-4).
type secretPattern struct {
	Kind    string `yaml:"kind"`
	Pattern string `yaml:"pattern"`
}

// defaultSecretPatterns is the canonical set of regex patterns used by the
// sanitize layer to redact secrets from prompts and responses (REQ-4).
var defaultSecretPatterns = []secretPattern{
	{Kind: "aws_key", Pattern: `AKIA[0-9A-Z]{16}`},
	{Kind: "token", Pattern: `sk-(ant|proj)?-?[A-Za-z0-9_-]{20,}`},
	{Kind: "token", Pattern: `ghp_[A-Za-z0-9]{36}`},
	{Kind: "token", Pattern: `github_pat_[A-Za-z0-9_]{82}`},
	{Kind: "token", Pattern: `xox[abprs]-[A-Za-z0-9-]+`},
	{Kind: "ssh_path", Pattern: `/\.ssh/[A-Za-z0-9_.-]+`},
	{Kind: "env_value", Pattern: `^[A-Z][A-Z0-9_]*=.+$`},
}

// defaultLoopConfig returns a loopConfig populated with the canonical defaults
// described in REQ-24 of the learning-loop-harness spec.
func defaultLoopConfig() *loopConfig {
	patterns := make([]secretPattern, len(defaultSecretPatterns))
	copy(patterns, defaultSecretPatterns)
	return &loopConfig{
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

// Load reads configuration from path (if non-empty and existing), applies
// LEARN_* env overrides, and validates the result. The returned loopConfig is
// always non-nil on success.
func loadConfig(path string) (*loopConfig, error) {
	cfg := defaultLoopConfig()

	if path != "" {
		data, readErr := os.ReadFile(path) //nolint:gosec // caller-supplied config path; the entire purpose of Load is to read from this path
		switch {
		case readErr == nil:
			if unmarshalErr := yaml.Unmarshal(data, cfg); unmarshalErr != nil {
				return nil, fmt.Errorf("config parse %s: %w", path, unmarshalErr)
			}
		case errors.Is(readErr, fs.ErrNotExist):
			// Missing file is not an error — defaults stand.
		default:
			return nil, fmt.Errorf("config read %s: %w", path, readErr)
		}
	}

	if envErr := applyEnvOverrides(cfg); envErr != nil {
		return nil, envErr
	}

	if validateErr := validate(cfg); validateErr != nil {
		return nil, validateErr
	}
	return cfg, nil
}

// applyEnvOverrides walks the LEARN_* namespace and overrides the
// corresponding field in cfg. Parse failures return an error citing both the
// env var name and the invalid value.
func applyEnvOverrides(cfg *loopConfig) error {
	intOverrides := []struct {
		env   string
		field *int
	}{
		{"LEARN_MIN_PATTERN_FREQ", &cfg.MinPatternFreq},
		{"LEARN_NUDGE_THRESHOLD", &cfg.NudgeThreshold},
		{"LEARN_TTL_DAYS", &cfg.TTLDays},
		{"LEARN_MIN_PROMPT_LEN_FOR_RECALL", &cfg.MinPromptLenForRecall},
		{"LEARN_RECALL_TOP_K", &cfg.RecallTopK},
		{"LEARN_RECALL_MAX_TOKENS", &cfg.RecallMaxTokens},
		{"LEARN_RECALL_TIMEOUT_SECONDS", &cfg.RecallTimeoutSeconds},
	}
	for _, o := range intOverrides {
		raw, ok := os.LookupEnv(o.env)
		if !ok {
			continue
		}
		v, parseErr := strconv.Atoi(raw)
		if parseErr != nil {
			return fmt.Errorf("config validation: env %s: invalid integer %q", o.env, raw)
		}
		*o.field = v
	}

	floatOverrides := []struct {
		env   string
		field *float64
	}{
		{"LEARN_SIMILARITY_THRESHOLD", &cfg.SimilarityThreshold},
		{"LEARN_RECALL_MIN_SCORE", &cfg.RecallMinScore},
	}
	for _, o := range floatOverrides {
		raw, ok := os.LookupEnv(o.env)
		if !ok {
			continue
		}
		v, parseErr := strconv.ParseFloat(raw, 64)
		if parseErr != nil {
			return fmt.Errorf("config validation: env %s: invalid float %q", o.env, raw)
		}
		*o.field = v
	}

	if v, ok := os.LookupEnv("LEARN_LLM_MODEL"); ok {
		cfg.LLMModel = v
	}

	if raw, ok := os.LookupEnv("LEARN_SECRET_PATTERNS"); ok {
		var patterns []secretPattern
		if unmarshalErr := yaml.Unmarshal([]byte(raw), &patterns); unmarshalErr != nil {
			return fmt.Errorf("config validation: env LEARN_SECRET_PATTERNS: %w", unmarshalErr)
		}
		cfg.SecretPatterns = patterns
	}
	return nil
}

// validate enforces the ranges documented in REQ-24.
func validate(cfg *loopConfig) error {
	intChecks := []struct {
		name string
		val  int
		min  int
	}{
		{"min_pattern_freq", cfg.MinPatternFreq, 1},
		{"nudge_threshold", cfg.NudgeThreshold, 1},
		{"ttl_days", cfg.TTLDays, 1},
		{"min_prompt_len_for_recall", cfg.MinPromptLenForRecall, 1},
		{"recall_top_k", cfg.RecallTopK, 1},
		{"recall_max_tokens", cfg.RecallMaxTokens, 1},
		{"recall_timeout_seconds", cfg.RecallTimeoutSeconds, 1},
	}
	for _, c := range intChecks {
		if c.val < c.min {
			return fmt.Errorf("config validation: field %s: value %d below minimum %d", c.name, c.val, c.min)
		}
	}

	floatChecks := []struct {
		name string
		val  float64
	}{
		{"similarity_threshold", cfg.SimilarityThreshold},
		{"recall_min_score", cfg.RecallMinScore},
	}
	for _, c := range floatChecks {
		if c.val < 0.0 {
			return fmt.Errorf("config validation: field %s: value %v below minimum 0", c.name, c.val)
		}
		if c.val > 1.0 {
			return fmt.Errorf("config validation: field %s: value %v above maximum 1", c.name, c.val)
		}
	}
	return nil
}
