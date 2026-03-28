package logutil

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaskEmail(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "normal email", input: "user@example.com", want: "u***@example.com"},
		{name: "single char local", input: "u@example.com", want: "u***@example.com"},
		{name: "empty string", input: "", want: "***"},
		{name: "no at sign", input: "invalid", want: "***"},
		{name: "at sign at start", input: "@example.com", want: "***"},
		{name: "long local part", input: "joao.silva@email.com", want: "j***@email.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskEmail(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMaskDocument(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "CPF 11 digits", input: "12345678901", want: "***8901"},
		{name: "CNPJ 14 digits", input: "12345678000195", want: "***0195"},
		{name: "short value", input: "abc", want: "***"},
		{name: "exactly 4 chars", input: "abcd", want: "***"},
		{name: "5 chars", input: "abcde", want: "***bcde"},
		{name: "empty string", input: "", want: "***"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskDocument(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMaskName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "two parts", input: "Joao Silva", want: "J*** S***"},
		{name: "three parts", input: "Joao Carlos Silva", want: "J*** C*** S***"},
		{name: "single name", input: "Joao", want: "J***"},
		{name: "single char name", input: "J", want: "J"},
		{name: "empty string", input: "", want: "***"},
		{name: "whitespace only", input: "   ", want: "***"},
		{name: "leading/trailing spaces", input: "  Joao Silva  ", want: "J*** S***"},
		{name: "CJK name with space", input: "田中 花子", want: "田*** 花***"},
		{name: "CJK single character", input: "王", want: "王"},
		{name: "cyrillic name", input: "Иван Петров", want: "И*** П***"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskName(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMaskPhone(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "with country code", input: "+5511999998888", want: "+55***8888"},
		{name: "without plus", input: "5511999998888", want: "55***8888"},
		{name: "short number", input: "1234", want: "***"},
		{name: "empty string", input: "", want: "***"},
		{name: "formatted phone", input: "+55 (11) 99999-8888", want: "+55***8888"},
		{name: "5 digits", input: "12345", want: "***"},
		{name: "6 digits", input: "123456", want: "***"},
		{name: "7 digits minimum for partial mask", input: "1234567", want: "12***4567"},
		{name: "UK phone", input: "+447911123456", want: "+44***3456"},
		{name: "US phone", input: "+14155552671", want: "+14***2671"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskPhone(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMaskSensitivePayload(t *testing.T) {
	t.Run("nil input returns nil", func(t *testing.T) {
		result := MaskSensitivePayload(nil)
		assert.Nil(t, result)
	})

	t.Run("non-map input returned unchanged", func(t *testing.T) {
		result := MaskSensitivePayload("just a string")
		assert.Equal(t, "just a string", result)
	})

	t.Run("masks email field", func(t *testing.T) {
		input := map[string]any{"email": "user@example.com", "id": 123}
		result := MaskSensitivePayload(input)

		m, ok := result.(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, "u***@example.com", m["email"])
		assert.Equal(t, 123, m["id"])
	})

	t.Run("masks document field", func(t *testing.T) {
		input := map[string]any{"document": "12345678901"}
		result := MaskSensitivePayload(input)

		m, ok := result.(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, "***8901", m["document"])
	})

	t.Run("masks name field", func(t *testing.T) {
		input := map[string]any{"name": "Joao Silva"}
		result := MaskSensitivePayload(input)

		m, ok := result.(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, "J*** S***", m["name"])
	})

	t.Run("masks phone field", func(t *testing.T) {
		input := map[string]any{"phone": "+5511999998888"}
		result := MaskSensitivePayload(input)

		m, ok := result.(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, "+55***8888", m["phone"])
	})

	t.Run("masks nested maps", func(t *testing.T) {
		input := map[string]any{
			"user": map[string]any{
				"email":    "user@example.com",
				"document": "12345678901",
			},
		}
		result := MaskSensitivePayload(input)

		m, ok := result.(map[string]any)
		assert.True(t, ok)
		nested, ok := m["user"].(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, "u***@example.com", nested["email"])
		assert.Equal(t, "***8901", nested["document"])
	})

	t.Run("masks items in slices", func(t *testing.T) {
		input := map[string]any{
			"users": []any{
				map[string]any{"email": "a@b.com"},
				map[string]any{"email": "c@d.com"},
			},
		}
		result := MaskSensitivePayload(input)

		m, ok := result.(map[string]any)
		assert.True(t, ok)
		users, ok := m["users"].([]any)
		assert.True(t, ok)
		assert.Len(t, users, 2)

		first, ok := users[0].(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, "a***@b.com", first["email"])
	})

	t.Run("skips empty string values", func(t *testing.T) {
		input := map[string]any{"email": "", "name": ""}
		result := MaskSensitivePayload(input)

		m, ok := result.(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, "", m["email"])
		assert.Equal(t, "", m["name"])
	})
}

func TestDefaultBRConfig(t *testing.T) {
	config := DefaultBRConfig()

	expectedFields := []string{
		"email",
		"document",
		"cpf",
		"cnpj",
		"name",
		"full_name",
		"first_name",
		"last_name",
		"phone",
		"telefone",
		"company_name",
		"trade_name",
	}

	assert.Len(t, config.Fields, 12, "DefaultBRConfig should have exactly 12 field mappings")

	for _, field := range expectedFields {
		_, exists := config.Fields[field]
		assert.True(t, exists, "DefaultBRConfig should contain field %q", field)
	}

	t.Run("email maps to MaskEmail", func(t *testing.T) {
		assert.Equal(t, "u***@example.com", config.Fields["email"]("user@example.com"))
	})

	t.Run("document maps to MaskDocument", func(t *testing.T) {
		assert.Equal(t, "***8901", config.Fields["document"]("12345678901"))
	})

	t.Run("cpf maps to MaskDocument", func(t *testing.T) {
		assert.Equal(t, "***8901", config.Fields["cpf"]("12345678901"))
	})

	t.Run("cnpj maps to MaskDocument", func(t *testing.T) {
		assert.Equal(t, "***0195", config.Fields["cnpj"]("12345678000195"))
	})

	t.Run("name maps to MaskName", func(t *testing.T) {
		assert.Equal(t, "J*** S***", config.Fields["name"]("Joao Silva"))
	})

	t.Run("phone maps to MaskPhone", func(t *testing.T) {
		assert.Equal(t, "+55***8888", config.Fields["phone"]("+5511999998888"))
	})

	t.Run("telefone maps to MaskPhone", func(t *testing.T) {
		assert.Equal(t, "+55***8888", config.Fields["telefone"]("+5511999998888"))
	})
}

func TestNewMasker(t *testing.T) {
	t.Run("creates masker with custom config", func(t *testing.T) {
		customMask := func(s string) string {
			return "[REDACTED]"
		}
		config := MaskConfig{
			Fields: map[string]MaskFunc{
				"secret": customMask,
			},
		}

		masker := NewMasker(config)
		assert.NotNil(t, masker)

		input := map[string]any{"secret": "my-api-key", "public": "visible"}
		result, ok := masker.MaskPayload(input).(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, "[REDACTED]", result["secret"])
		assert.Equal(t, "visible", result["public"])
	})

	t.Run("normalizes field names to lowercase", func(t *testing.T) {
		config := MaskConfig{
			Fields: map[string]MaskFunc{
				"MyField": func(s string) string { return "masked" },
			},
		}

		masker := NewMasker(config)
		input := map[string]any{"myfield": "value"}
		result, ok := masker.MaskPayload(input).(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, "masked", result["myfield"])
	})
}

func TestMaskerMaskPayload(t *testing.T) {
	t.Run("nil input returns nil", func(t *testing.T) {
		masker := NewMasker(MaskConfig{Fields: map[string]MaskFunc{}})
		assert.Nil(t, masker.MaskPayload(nil))
	})

	t.Run("non-map input returned unchanged", func(t *testing.T) {
		masker := NewMasker(MaskConfig{Fields: map[string]MaskFunc{}})
		assert.Equal(t, 42, masker.MaskPayload(42))
		assert.Equal(t, "hello", masker.MaskPayload("hello"))
	})

	t.Run("masks custom SSN field", func(t *testing.T) {
		ssnMask := func(s string) string {
			if len(s) < 4 {
				return "***"
			}
			return "***-**-" + s[len(s)-4:]
		}
		config := MaskConfig{
			Fields: map[string]MaskFunc{
				"ssn": ssnMask,
			},
		}
		masker := NewMasker(config)

		input := map[string]any{"ssn": "123-45-6789", "name": "John Doe"}
		result, ok := masker.MaskPayload(input).(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, "***-**-6789", result["ssn"])
		assert.Equal(t, "John Doe", result["name"], "non-configured fields should pass through")
	})

	t.Run("masks custom IBAN field", func(t *testing.T) {
		ibanMask := func(s string) string {
			if len(s) <= 4 {
				return "***"
			}
			return s[:2] + "***" + s[len(s)-4:]
		}
		config := MaskConfig{
			Fields: map[string]MaskFunc{
				"iban": ibanMask,
			},
		}
		masker := NewMasker(config)

		input := map[string]any{"iban": "DE89370400440532013000"}
		result, ok := masker.MaskPayload(input).(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, "DE***3000", result["iban"])
	})

	t.Run("handles nested maps with custom config", func(t *testing.T) {
		config := MaskConfig{
			Fields: map[string]MaskFunc{
				"token": func(s string) string { return "[REDACTED]" },
			},
		}
		masker := NewMasker(config)

		input := map[string]any{
			"auth": map[string]any{
				"token":    "secret-bearer-token",
				"username": "admin",
			},
		}
		result, ok := masker.MaskPayload(input).(map[string]any)
		assert.True(t, ok)
		auth, ok := result["auth"].(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, "[REDACTED]", auth["token"])
		assert.Equal(t, "admin", auth["username"])
	})

	t.Run("handles slices with custom config", func(t *testing.T) {
		config := MaskConfig{
			Fields: map[string]MaskFunc{
				"ssn": func(s string) string { return "***" },
			},
		}
		masker := NewMasker(config)

		input := map[string]any{
			"employees": []any{
				map[string]any{"ssn": "111-22-3333"},
				map[string]any{"ssn": "444-55-6666"},
			},
		}
		result, ok := masker.MaskPayload(input).(map[string]any)
		assert.True(t, ok)
		employees, ok := result["employees"].([]any)
		assert.True(t, ok)
		assert.Len(t, employees, 2)

		first, ok := employees[0].(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, "***", first["ssn"])

		second, ok := employees[1].(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, "***", second["ssn"])
	})

	t.Run("skips empty string values", func(t *testing.T) {
		config := MaskConfig{
			Fields: map[string]MaskFunc{
				"secret": func(s string) string { return "[MASKED]" },
			},
		}
		masker := NewMasker(config)

		input := map[string]any{"secret": ""}
		result, ok := masker.MaskPayload(input).(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, "", result["secret"])
	})
}

func TestMaskConfigMerge(t *testing.T) {
	t.Run("merge two configs", func(t *testing.T) {
		base := MaskConfig{
			Fields: map[string]MaskFunc{
				"email": MaskEmail,
				"name":  MaskName,
			},
		}
		extra := MaskConfig{
			Fields: map[string]MaskFunc{
				"ssn":   func(s string) string { return "***" },
				"phone": MaskPhone,
			},
		}

		merged := base.Merge(extra)
		assert.Len(t, merged.Fields, 4)
		assert.Contains(t, merged.Fields, "email")
		assert.Contains(t, merged.Fields, "name")
		assert.Contains(t, merged.Fields, "ssn")
		assert.Contains(t, merged.Fields, "phone")
	})

	t.Run("later config overrides earlier for same field", func(t *testing.T) {
		original := MaskConfig{
			Fields: map[string]MaskFunc{
				"email": func(s string) string { return "original" },
			},
		}
		override := MaskConfig{
			Fields: map[string]MaskFunc{
				"email": func(s string) string { return "overridden" },
			},
		}

		merged := original.Merge(override)
		assert.Equal(t, "overridden", merged.Fields["email"]("test@example.com"))
	})

	t.Run("merge normalizes keys to lowercase", func(t *testing.T) {
		base := MaskConfig{
			Fields: map[string]MaskFunc{
				"Email": MaskEmail,
			},
		}
		extra := MaskConfig{
			Fields: map[string]MaskFunc{
				"PHONE": MaskPhone,
			},
		}

		merged := base.Merge(extra)
		assert.Contains(t, merged.Fields, "email")
		assert.Contains(t, merged.Fields, "phone")
		assert.NotContains(t, merged.Fields, "Email")
		assert.NotContains(t, merged.Fields, "PHONE")
	})

	t.Run("merge multiple configs at once", func(t *testing.T) {
		a := MaskConfig{Fields: map[string]MaskFunc{"a": MaskEmail}}
		b := MaskConfig{Fields: map[string]MaskFunc{"b": MaskName}}
		c := MaskConfig{Fields: map[string]MaskFunc{"c": MaskPhone}}

		merged := a.Merge(b, c)
		assert.Len(t, merged.Fields, 3)
		assert.Contains(t, merged.Fields, "a")
		assert.Contains(t, merged.Fields, "b")
		assert.Contains(t, merged.Fields, "c")
	})

	t.Run("merge does not mutate receiver", func(t *testing.T) {
		base := MaskConfig{
			Fields: map[string]MaskFunc{
				"email": MaskEmail,
			},
		}
		extra := MaskConfig{
			Fields: map[string]MaskFunc{
				"phone": MaskPhone,
			},
		}

		_ = base.Merge(extra)
		assert.Len(t, base.Fields, 1, "original config should not be modified")
		assert.Contains(t, base.Fields, "email")
		assert.NotContains(t, base.Fields, "phone")
	})

	t.Run("merge empty configs", func(t *testing.T) {
		base := MaskConfig{Fields: map[string]MaskFunc{}}
		extra := MaskConfig{Fields: map[string]MaskFunc{}}

		merged := base.Merge(extra)
		assert.Empty(t, merged.Fields)
	})
}

func TestCaseInsensitiveKeys(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		wantMask string
	}{
		{name: "lowercase email", key: "email", value: "user@example.com", wantMask: "u***@example.com"},
		{name: "uppercase EMAIL", key: "EMAIL", value: "user@example.com", wantMask: "u***@example.com"},
		{name: "mixed case Email", key: "Email", value: "user@example.com", wantMask: "u***@example.com"},
		{name: "camelCase eMaIl", key: "eMaIl", value: "user@example.com", wantMask: "u***@example.com"},
		{name: "lowercase name", key: "name", value: "Joao Silva", wantMask: "J*** S***"},
		{name: "uppercase NAME", key: "NAME", value: "Joao Silva", wantMask: "J*** S***"},
		{name: "mixed Full_Name", key: "Full_Name", value: "Joao Silva", wantMask: "J*** S***"},
		{name: "uppercase PHONE", key: "PHONE", value: "+5511999998888", wantMask: "+55***8888"},
		{name: "uppercase DOCUMENT", key: "DOCUMENT", value: "12345678901", wantMask: "***8901"},
		{name: "uppercase CPF", key: "CPF", value: "12345678901", wantMask: "***8901"},
	}

	masker := NewMasker(DefaultBRConfig())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := map[string]any{tt.key: tt.value}
			result, ok := masker.MaskPayload(input).(map[string]any)
			assert.True(t, ok)
			assert.Equal(t, tt.wantMask, result[tt.key])
		})
	}
}

func TestEmptyConfig(t *testing.T) {
	masker := NewMasker(MaskConfig{Fields: map[string]MaskFunc{}})

	t.Run("passes through all fields unchanged", func(t *testing.T) {
		input := map[string]any{
			"email":    "user@example.com",
			"document": "12345678901",
			"name":     "Joao Silva",
			"phone":    "+5511999998888",
			"id":       123,
		}
		result, ok := masker.MaskPayload(input).(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, "user@example.com", result["email"])
		assert.Equal(t, "12345678901", result["document"])
		assert.Equal(t, "Joao Silva", result["name"])
		assert.Equal(t, "+5511999998888", result["phone"])
		assert.Equal(t, 123, result["id"])
	})

	t.Run("passes through nested maps unchanged", func(t *testing.T) {
		input := map[string]any{
			"user": map[string]any{
				"email": "user@example.com",
				"name":  "Joao",
			},
		}
		result, ok := masker.MaskPayload(input).(map[string]any)
		assert.True(t, ok)
		nested, ok := result["user"].(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, "user@example.com", nested["email"])
		assert.Equal(t, "Joao", nested["name"])
	})

	t.Run("nil input returns nil", func(t *testing.T) {
		assert.Nil(t, masker.MaskPayload(nil))
	})

	t.Run("non-map input returned unchanged", func(t *testing.T) {
		assert.Equal(t, "hello", masker.MaskPayload("hello"))
	})
}

func TestMaskConfigMergeWithDefaultBR(t *testing.T) {
	t.Run("extend BR config with custom fields", func(t *testing.T) {
		customConfig := MaskConfig{
			Fields: map[string]MaskFunc{
				"ssn":  func(s string) string { return "***" },
				"iban": func(s string) string { return strings.Repeat("*", len(s)) },
			},
		}

		merged := DefaultBRConfig().Merge(customConfig)
		masker := NewMasker(merged)

		input := map[string]any{
			"email": "user@example.com",
			"ssn":   "123-45-6789",
			"iban":  "DE89370400",
			"city":  "Berlin",
		}
		result, ok := masker.MaskPayload(input).(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, "u***@example.com", result["email"], "BR field should still be masked")
		assert.Equal(t, "***", result["ssn"], "custom field should be masked")
		assert.Equal(t, "**********", result["iban"], "custom field should be masked")
		assert.Equal(t, "Berlin", result["city"], "unknown field should pass through")
	})
}
