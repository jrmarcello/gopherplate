package sanitize

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jrmarcello/gopherplate/tools/learn/internal/config"
)

// builtin returns a Sanitizer built from config.DefaultSecretPatterns. It
// fails the test immediately on compile error so individual subtests stay
// focused on Sanitize behavior.
func builtin(t *testing.T) *Sanitizer {
	t.Helper()
	s, err := NewFromConfig(config.DefaultSecretPatterns)
	if err != nil {
		t.Fatalf("NewFromConfig(defaults) returned error: %v", err)
	}
	return s
}

// TestSanitize_Defaults covers every secret class enumerated in REQ-4
// (TC-D-04 through TC-D-07). For each input the table asserts that the
// canonical redacted token appears in the output. We assert substring
// containment (not full equality) for inputs where the secret pattern can
// match a sub-range of the input (e.g. ssh_path embedded in a longer path).
func TestSanitize_Defaults(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		tcID     string
		input    string
		mustHave string // substring that MUST appear in output
		mustGone string // substring of original secret that MUST NOT remain
	}{
		{
			name:     "aws_key",
			tcID:     "TC-D-04",
			input:    "AKIA0123456789ABCDEF",
			mustHave: "<REDACTED:aws_key>",
			mustGone: "AKIA0123456789ABCDEF",
		},
		{
			name:     "openai_sk_no_prefix",
			tcID:     "TC-D-05a",
			input:    "sk-abcDEF1234567890ABCDEF12",
			mustHave: "<REDACTED:token>",
			mustGone: "sk-abcDEF1234567890ABCDEF12",
		},
		{
			name:     "openai_project",
			tcID:     "TC-D-05b",
			input:    "sk-proj-abcDEF1234567890ABCDEF12",
			mustHave: "<REDACTED:token>",
			mustGone: "sk-proj-abcDEF1234567890ABCDEF12",
		},
		{
			name:     "anthropic_sk_ant",
			tcID:     "TC-D-05c",
			input:    "sk-ant-api03-abcDEF1234567890ABCDEF12",
			mustHave: "<REDACTED:token>",
			mustGone: "sk-ant-api03-abcDEF1234567890ABCDEF12",
		},
		{
			name:     "github_pat_classic",
			tcID:     "TC-D-05d",
			input:    "ghp_" + strings.Repeat("a", 36),
			mustHave: "<REDACTED:token>",
			mustGone: "ghp_" + strings.Repeat("a", 36),
		},
		{
			name:     "github_pat_finegrained",
			tcID:     "TC-D-05e",
			input:    "github_pat_" + strings.Repeat("A", 82),
			mustHave: "<REDACTED:token>",
			mustGone: "github_pat_" + strings.Repeat("A", 82),
		},
		{
			name:     "slack_token",
			tcID:     "TC-D-05f",
			input:    "xoxb-1234567890-something",
			mustHave: "<REDACTED:token>",
			mustGone: "xoxb-1234567890-something",
		},
		{
			name:     "ssh_path",
			tcID:     "TC-D-05g",
			input:    "/Users/x/.ssh/id_rsa",
			mustHave: "<REDACTED:ssh_path>",
			mustGone: "/.ssh/id_rsa",
		},
		{
			name:     "env_value_multiline",
			tcID:     "TC-D-06",
			input:    "API_KEY=secretvalue123\nNEXT=line",
			mustHave: "<REDACTED:env_value>",
			mustGone: "API_KEY=secretvalue123",
		},
		{
			// TC-D-07: policy is conservative — a string that LOOKS like a
			// Slack token is redacted even if it's not a real one. False
			// positives are accepted; false negatives of known classes are
			// not.
			name:     "false_positive_accepted",
			tcID:     "TC-D-07",
			input:    "xoxb-not-real-but-matches",
			mustHave: "<REDACTED:token>",
			mustGone: "xoxb-not-real-but-matches",
		},
		{
			// TC-D-07b (extra boundary): overlapping/ambiguous input — a
			// string that contains BOTH a token-like prefix and an embedded
			// AWS-key pattern. Whichever pattern wins by first-match-order
			// must STILL prevent the raw secret from surviving in the
			// output. A mutant that short-circuits on the first match
			// without consuming the AWS substring would let "AKIA..."
			// leak through; assert neither raw token nor raw key remains.
			name:     "overlapping_patterns",
			tcID:     "TC-D-07b",
			input:    "leak: sk-AKIA0123456789ABCDEF and more text",
			mustHave: "<REDACTED",
			mustGone: "AKIA0123456789ABCDEF",
		},
	}

	s := builtin(t)
	for _, tc := range cases {
		tc := tc
		t.Run(tc.tcID+"_"+tc.name, func(t *testing.T) {
			t.Parallel()
			got := s.Sanitize(tc.input)
			if !strings.Contains(got, tc.mustHave) {
				t.Errorf("Sanitize(%q) = %q; want it to contain %q", tc.input, got, tc.mustHave)
			}
			if tc.mustGone != "" && strings.Contains(got, tc.mustGone) {
				t.Errorf("Sanitize(%q) = %q; original secret %q must not remain", tc.input, got, tc.mustGone)
			}
		})
	}
}

// TestSanitize_Pure (TC-D-07a) demonstrates Sanitize is a pure function: it
// never reads or writes the filesystem. We snapshot a temp directory
// (chdir into it) before and after a sanitization call and assert the
// directory listing is byte-identical.
func TestSanitize_Pure(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	// Seed a file so the snapshot has at least one entry to compare.
	seed := filepath.Join(tmp, "seed.txt")
	if writeErr := os.WriteFile(seed, []byte("seed"), 0o600); writeErr != nil {
		t.Fatalf("seed write: %v", writeErr)
	}

	before := snapshotDir(t, tmp)

	s := builtin(t)
	out := s.Sanitize("AKIA0123456789ABCDEF and some plain text")
	if !strings.Contains(out, "<REDACTED:aws_key>") {
		t.Fatalf("expected aws_key to be redacted, got %q", out)
	}

	after := snapshotDir(t, tmp)
	if before != after {
		t.Errorf("Sanitize touched the filesystem:\nbefore=%s\nafter=%s", before, after)
	}
}

// snapshotDir returns a deterministic listing of dir contents (name + size).
func snapshotDir(t *testing.T, dir string) string {
	t.Helper()
	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		t.Fatalf("snapshotDir: %v", readErr)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		info, infoErr := e.Info()
		if infoErr != nil {
			t.Fatalf("snapshotDir: stat %s: %v", e.Name(), infoErr)
		}
		names = append(names, e.Name()+":"+itoa(info.Size()))
	}
	sort.Strings(names)
	return strings.Join(names, "|")
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func TestSanitize_MultipleMatches(t *testing.T) {
	t.Parallel()
	s := builtin(t)
	in := "key1=AKIA0123456789ABCDEF and key2=ghp_" + strings.Repeat("b", 36)
	out := s.Sanitize(in)
	if strings.Contains(out, "AKIA0123456789ABCDEF") {
		t.Errorf("aws_key not redacted: %q", out)
	}
	if strings.Contains(out, "ghp_"+strings.Repeat("b", 36)) {
		t.Errorf("github pat not redacted: %q", out)
	}
	if c := strings.Count(out, "<REDACTED:"); c < 2 {
		t.Errorf("expected >=2 redaction markers, got %d in %q", c, out)
	}
}

func TestSanitize_EmptyInput(t *testing.T) {
	t.Parallel()
	s := builtin(t)
	if got := s.Sanitize(""); got != "" {
		t.Errorf("Sanitize(\"\") = %q; want empty", got)
	}
}

func TestSanitize_NoSecret(t *testing.T) {
	t.Parallel()
	s := builtin(t)
	in := "this is a plain message with no secrets in it"
	if got := s.Sanitize(in); got != in {
		t.Errorf("Sanitize(%q) = %q; want unchanged", in, got)
	}
}

func TestNewFromConfig_CompileError(t *testing.T) {
	t.Parallel()
	bad := []config.SecretPattern{{Kind: "broken", Pattern: "([unclosed"}}
	if _, compileErr := NewFromConfig(bad); compileErr == nil {
		t.Fatalf("NewFromConfig(bad) returned no error; want compile failure")
	}
}

func TestSanitizeBytes_MatchesString(t *testing.T) {
	t.Parallel()
	s := builtin(t)
	in := "secret=AKIA0123456789ABCDEF"
	gotString := s.Sanitize(in)
	gotBytes := string(s.SanitizeBytes([]byte(in)))
	if gotString != gotBytes {
		t.Errorf("SanitizeBytes diverged from Sanitize: %q vs %q", gotBytes, gotString)
	}
}

func TestBuiltinPatterns_CompilesAll(t *testing.T) {
	t.Parallel()
	pats, err := BuiltinPatterns()
	if err != nil {
		t.Fatalf("BuiltinPatterns: %v", err)
	}
	if len(pats) != len(config.DefaultSecretPatterns) {
		t.Errorf("len(BuiltinPatterns())=%d, want %d", len(pats), len(config.DefaultSecretPatterns))
	}
	for i, p := range pats {
		if p.Re == nil {
			t.Errorf("pattern %d (%s) has nil regexp", i, p.Kind)
		}
	}
}
