package learn

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TC-D-10: parser de config.yml com defaults — load with path="" returns defaultLoopConfig().
func TestLoad_EmptyPath_ReturnsDefaults(t *testing.T) {
	cfg, loadErr := loadConfig("")
	if loadErr != nil {
		t.Fatalf("unexpected error: %v", loadErr)
	}
	want := defaultLoopConfig()
	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("loadConfig(\"\") = %+v, want %+v", cfg, want)
	}
}

func TestLoad_NonexistentPath_ReturnsDefaults(t *testing.T) {
	cfg, loadErr := loadConfig("/nonexistent/path/to/config.yml")
	if loadErr != nil {
		t.Fatalf("unexpected error: %v", loadErr)
	}
	want := defaultLoopConfig()
	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("got %+v, want %+v", cfg, want)
	}
}

func TestLoad_YAMLPartialPreservesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	content := []byte("nudge_threshold: 10\n")
	if writeErr := os.WriteFile(path, content, 0o600); writeErr != nil {
		t.Fatalf("write fixture: %v", writeErr)
	}
	cfg, loadErr := loadConfig(path)
	if loadErr != nil {
		t.Fatalf("unexpected error: %v", loadErr)
	}
	if cfg.NudgeThreshold != 10 {
		t.Errorf("NudgeThreshold = %d, want 10", cfg.NudgeThreshold)
	}
	// Defaults preserved for unset fields.
	if cfg.TTLDays != 90 {
		t.Errorf("TTLDays = %d, want default 90", cfg.TTLDays)
	}
	if cfg.MinPatternFreq != 3 {
		t.Errorf("MinPatternFreq = %d, want default 3", cfg.MinPatternFreq)
	}
}

// TC-D-11: LEARN_NUDGE_THRESHOLD=3 overrides YAML
func TestLoad_EnvOverrides(t *testing.T) {
	tests := []struct {
		name   string
		env    map[string]string
		assert func(*testing.T, *loopConfig)
	}{
		{
			name: "TC-D-11: LEARN_NUDGE_THRESHOLD overrides",
			env:  map[string]string{"LEARN_NUDGE_THRESHOLD": "3"},
			assert: func(t *testing.T, c *loopConfig) {
				if c.NudgeThreshold != 3 {
					t.Errorf("NudgeThreshold = %d, want 3", c.NudgeThreshold)
				}
			},
		},
		{
			name: "TC-D-11a: LEARN_TTL_DAYS overrides",
			env:  map[string]string{"LEARN_TTL_DAYS": "30"},
			assert: func(t *testing.T, c *loopConfig) {
				if c.TTLDays != 30 {
					t.Errorf("TTLDays = %d, want 30", c.TTLDays)
				}
			},
		},
		{
			name: "TC-D-11b: LEARN_RECALL_MIN_SCORE overrides",
			env:  map[string]string{"LEARN_RECALL_MIN_SCORE": "0.55"},
			assert: func(t *testing.T, c *loopConfig) {
				if c.RecallMinScore != 0.55 {
					t.Errorf("RecallMinScore = %v, want 0.55", c.RecallMinScore)
				}
			},
		},
		{
			name: "LEARN_MIN_PATTERN_FREQ overrides",
			env:  map[string]string{"LEARN_MIN_PATTERN_FREQ": "7"},
			assert: func(t *testing.T, c *loopConfig) {
				if c.MinPatternFreq != 7 {
					t.Errorf("MinPatternFreq = %d, want 7", c.MinPatternFreq)
				}
			},
		},
		{
			name: "LEARN_SIMILARITY_THRESHOLD overrides",
			env:  map[string]string{"LEARN_SIMILARITY_THRESHOLD": "0.8"},
			assert: func(t *testing.T, c *loopConfig) {
				if c.SimilarityThreshold != 0.8 {
					t.Errorf("SimilarityThreshold = %v, want 0.8", c.SimilarityThreshold)
				}
			},
		},
		{
			name: "LEARN_LLM_MODEL overrides",
			env:  map[string]string{"LEARN_LLM_MODEL": "claude-opus-4-7"},
			assert: func(t *testing.T, c *loopConfig) {
				if c.LLMModel != "claude-opus-4-7" {
					t.Errorf("LLMModel = %q, want %q", c.LLMModel, "claude-opus-4-7")
				}
			},
		},
		{
			name: "LEARN_MIN_PROMPT_LEN_FOR_RECALL overrides",
			env:  map[string]string{"LEARN_MIN_PROMPT_LEN_FOR_RECALL": "50"},
			assert: func(t *testing.T, c *loopConfig) {
				if c.MinPromptLenForRecall != 50 {
					t.Errorf("MinPromptLenForRecall = %d, want 50", c.MinPromptLenForRecall)
				}
			},
		},
		{
			name: "LEARN_RECALL_TOP_K overrides",
			env:  map[string]string{"LEARN_RECALL_TOP_K": "5"},
			assert: func(t *testing.T, c *loopConfig) {
				if c.RecallTopK != 5 {
					t.Errorf("RecallTopK = %d, want 5", c.RecallTopK)
				}
			},
		},
		{
			name: "LEARN_RECALL_MAX_TOKENS overrides",
			env:  map[string]string{"LEARN_RECALL_MAX_TOKENS": "1000"},
			assert: func(t *testing.T, c *loopConfig) {
				if c.RecallMaxTokens != 1000 {
					t.Errorf("RecallMaxTokens = %d, want 1000", c.RecallMaxTokens)
				}
			},
		},
		{
			name: "LEARN_RECALL_TIMEOUT_SECONDS overrides",
			env:  map[string]string{"LEARN_RECALL_TIMEOUT_SECONDS": "5"},
			assert: func(t *testing.T, c *loopConfig) {
				if c.RecallTimeoutSeconds != 5 {
					t.Errorf("RecallTimeoutSeconds = %d, want 5", c.RecallTimeoutSeconds)
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			cfg, loadErr := loadConfig("")
			if loadErr != nil {
				t.Fatalf("unexpected error: %v", loadErr)
			}
			tc.assert(t, cfg)
		})
	}
}

// TC-D-11c: LEARN_SECRET_PATTERNS (YAML inline) overrides default.
func TestLoad_EnvSecretPatternsOverride(t *testing.T) {
	yamlInline := `- kind: custom
  pattern: "FOO[0-9]+"
- kind: token
  pattern: "BAR-[A-Z]+"`
	t.Setenv("LEARN_SECRET_PATTERNS", yamlInline)
	cfg, loadErr := loadConfig("")
	if loadErr != nil {
		t.Fatalf("unexpected error: %v", loadErr)
	}
	if len(cfg.SecretPatterns) != 2 {
		t.Fatalf("SecretPatterns len = %d, want 2", len(cfg.SecretPatterns))
	}
	if cfg.SecretPatterns[0].Kind != "custom" || cfg.SecretPatterns[0].Pattern != "FOO[0-9]+" {
		t.Errorf("SecretPatterns[0] = %+v", cfg.SecretPatterns[0])
	}
	if cfg.SecretPatterns[1].Kind != "token" || cfg.SecretPatterns[1].Pattern != "BAR-[A-Z]+" {
		t.Errorf("SecretPatterns[1] = %+v", cfg.SecretPatterns[1])
	}
}

// Env overrides YAML (priority test).
func TestLoad_EnvOverridesYAMLFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	content := []byte("nudge_threshold: 7\nttl_days: 60\n")
	if writeErr := os.WriteFile(path, content, 0o600); writeErr != nil {
		t.Fatalf("write fixture: %v", writeErr)
	}
	t.Setenv("LEARN_NUDGE_THRESHOLD", "99")
	cfg, loadErr := loadConfig(path)
	if loadErr != nil {
		t.Fatalf("unexpected error: %v", loadErr)
	}
	if cfg.NudgeThreshold != 99 {
		t.Errorf("NudgeThreshold = %d, want env value 99", cfg.NudgeThreshold)
	}
	if cfg.TTLDays != 60 {
		t.Errorf("TTLDays = %d, want yaml value 60", cfg.TTLDays)
	}
}

// TC-D-12: YAML malformed → error contains line/column.
func TestLoad_MalformedYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	content := []byte("nudge_threshold: \"unterminated\n  ttl_days: 30\n")
	if writeErr := os.WriteFile(path, content, 0o600); writeErr != nil {
		t.Fatalf("write fixture: %v", writeErr)
	}
	_, loadErr := loadConfig(path)
	if loadErr == nil {
		t.Fatal("expected error, got nil")
	}
	msg := loadErr.Error()
	if !strings.Contains(msg, "line") {
		t.Errorf("error message %q should contain line info", msg)
	}
}

// TC-D-13: file with ttl_days: -1 → error cites ttl_days and range.
func TestLoad_ValidationErrors(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		env         map[string]string
		wantSubstrs []string
	}{
		{
			name:        "TC-D-13: ttl_days negative",
			yaml:        "ttl_days: -1\n",
			wantSubstrs: []string{"ttl_days", "-1", "minimum 1"},
		},
		{
			name:        "TC-D-13b: recall_min_score above max",
			yaml:        "recall_min_score: 1.5\n",
			wantSubstrs: []string{"recall_min_score", "1.5", "maximum 1"},
		},
		{
			name:        "similarity_threshold negative",
			yaml:        "similarity_threshold: -0.1\n",
			wantSubstrs: []string{"similarity_threshold", "-0.1", "minimum 0"},
		},
		{
			name:        "min_pattern_freq zero",
			yaml:        "min_pattern_freq: 0\n",
			wantSubstrs: []string{"min_pattern_freq", "0", "minimum 1"},
		},
		{
			name:        "nudge_threshold zero",
			yaml:        "nudge_threshold: 0\n",
			wantSubstrs: []string{"nudge_threshold", "0", "minimum 1"},
		},
		{
			name:        "recall_top_k zero",
			yaml:        "recall_top_k: 0\n",
			wantSubstrs: []string{"recall_top_k", "0", "minimum 1"},
		},
		{
			name:        "recall_max_tokens zero",
			yaml:        "recall_max_tokens: 0\n",
			wantSubstrs: []string{"recall_max_tokens", "0", "minimum 1"},
		},
		{
			name:        "recall_timeout_seconds zero",
			yaml:        "recall_timeout_seconds: 0\n",
			wantSubstrs: []string{"recall_timeout_seconds", "0", "minimum 1"},
		},
		{
			name:        "min_prompt_len_for_recall zero",
			yaml:        "min_prompt_len_for_recall: 0\n",
			wantSubstrs: []string{"min_prompt_len_for_recall", "0", "minimum 1"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.yml")
			if writeErr := os.WriteFile(path, []byte(tc.yaml), 0o600); writeErr != nil {
				t.Fatalf("write fixture: %v", writeErr)
			}
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			_, loadErr := loadConfig(path)
			if loadErr == nil {
				t.Fatal("expected error, got nil")
			}
			msg := loadErr.Error()
			for _, sub := range tc.wantSubstrs {
				if !strings.Contains(msg, sub) {
					t.Errorf("error %q missing substring %q", msg, sub)
				}
			}
		})
	}
}

// TC-D-13c (extra boundary): inclusive limits at 0.0 and 1.0 for float ranges
// MUST be accepted. Spec REQ-24 says `float ∈ [0.0, 1.0]` (inclusive on both
// ends). A mutant that uses `<` instead of `<=` in the bound check would let
// a value at the boundary fail; this test pins the inclusive semantics.
func TestLoad_BoundaryInclusiveLimits(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{"recall_min_score=0.0 valid", "recall_min_score: 0.0\n"},
		{"recall_min_score=1.0 valid", "recall_min_score: 1.0\n"},
		{"similarity_threshold=0.0 valid", "similarity_threshold: 0.0\n"},
		{"similarity_threshold=1.0 valid", "similarity_threshold: 1.0\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.yml")
			if writeErr := os.WriteFile(path, []byte(tc.yaml), 0o600); writeErr != nil {
				t.Fatalf("write fixture: %v", writeErr)
			}
			if _, loadErr := loadConfig(path); loadErr != nil {
				t.Errorf("expected nil error at inclusive boundary, got %v", loadErr)
			}
		})
	}
}

// TC-D-13a: env LEARN_TTL_DAYS=abc → error cites field name + invalid value.
func TestLoad_EnvInvalidInteger(t *testing.T) {
	t.Setenv("LEARN_TTL_DAYS", "abc")
	_, loadErr := loadConfig("")
	if loadErr == nil {
		t.Fatal("expected error, got nil")
	}
	msg := loadErr.Error()
	if !strings.Contains(msg, "LEARN_TTL_DAYS") {
		t.Errorf("error %q should cite env var name", msg)
	}
	if !strings.Contains(msg, "abc") {
		t.Errorf("error %q should cite invalid value", msg)
	}
}

func TestLoad_EnvInvalidFloat(t *testing.T) {
	t.Setenv("LEARN_RECALL_MIN_SCORE", "xyz")
	_, loadErr := loadConfig("")
	if loadErr == nil {
		t.Fatal("expected error, got nil")
	}
	msg := loadErr.Error()
	if !strings.Contains(msg, "LEARN_RECALL_MIN_SCORE") {
		t.Errorf("error %q should cite env var name", msg)
	}
	if !strings.Contains(msg, "xyz") {
		t.Errorf("error %q should cite invalid value", msg)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := defaultLoopConfig()
	if cfg.MinPatternFreq != 3 {
		t.Errorf("MinPatternFreq = %d, want 3", cfg.MinPatternFreq)
	}
	if cfg.SimilarityThreshold != 0.6 {
		t.Errorf("SimilarityThreshold = %v, want 0.6", cfg.SimilarityThreshold)
	}
	if cfg.NudgeThreshold != 5 {
		t.Errorf("NudgeThreshold = %d, want 5", cfg.NudgeThreshold)
	}
	if cfg.TTLDays != 90 {
		t.Errorf("TTLDays = %d, want 90", cfg.TTLDays)
	}
	if cfg.RecallMinScore != 0.4 {
		t.Errorf("RecallMinScore = %v, want 0.4", cfg.RecallMinScore)
	}
	if cfg.MinPromptLenForRecall != 20 {
		t.Errorf("MinPromptLenForRecall = %d, want 20", cfg.MinPromptLenForRecall)
	}
	if cfg.RecallTopK != 3 {
		t.Errorf("RecallTopK = %d, want 3", cfg.RecallTopK)
	}
	if cfg.RecallMaxTokens != 500 {
		t.Errorf("RecallMaxTokens = %d, want 500", cfg.RecallMaxTokens)
	}
	if cfg.RecallTimeoutSeconds != 2 {
		t.Errorf("RecallTimeoutSeconds = %d, want 2", cfg.RecallTimeoutSeconds)
	}
	if cfg.LLMModel != "" {
		t.Errorf("LLMModel = %q, want empty", cfg.LLMModel)
	}
	if len(cfg.SecretPatterns) != len(defaultSecretPatterns) {
		t.Errorf("SecretPatterns len = %d, want %d", len(cfg.SecretPatterns), len(defaultSecretPatterns))
	}
}
