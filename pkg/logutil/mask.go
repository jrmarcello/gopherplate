package logutil

import (
	"strings"
	"unicode/utf8"
)

// MaskFunc is a function that masks a sensitive string value.
type MaskFunc func(string) string

// MaskConfig holds the mapping of field names to their masking functions.
// Field names are matched case-insensitively (stored lowercase).
type MaskConfig struct {
	Fields map[string]MaskFunc
}

// Merge returns a new MaskConfig combining the receiver with other configs.
// Later configs override earlier ones for the same field name.
func (c MaskConfig) Merge(others ...MaskConfig) MaskConfig {
	merged := MaskConfig{
		Fields: make(map[string]MaskFunc, len(c.Fields)),
	}
	for k, v := range c.Fields {
		merged.Fields[strings.ToLower(k)] = v
	}
	for _, other := range others {
		for k, v := range other.Fields {
			merged.Fields[strings.ToLower(k)] = v
		}
	}
	return merged
}

// Masker applies PII masking to payloads based on its MaskConfig.
type Masker struct {
	config MaskConfig
}

// NewMasker creates a Masker with the given config.
func NewMasker(config MaskConfig) *Masker {
	// Normalize all keys to lowercase.
	normalized := MaskConfig{
		Fields: make(map[string]MaskFunc, len(config.Fields)),
	}
	for k, v := range config.Fields {
		normalized.Fields[strings.ToLower(k)] = v
	}
	return &Masker{config: normalized}
}

// MaskPayload recursively masks sensitive fields in the input.
// Returns the input unchanged for non-map types, or a new map with masked values.
func (m *Masker) MaskPayload(input any) any {
	if input == nil {
		return nil
	}

	switch v := input.(type) {
	case map[string]any:
		return m.maskMap(v)
	default:
		return input
	}
}

// maskMap creates a shallow copy of the map with sensitive string fields masked.
// Nested maps and slices are processed recursively.
func (m *Masker) maskMap(mp map[string]any) map[string]any {
	result := make(map[string]any, len(mp))
	for k, v := range mp {
		result[k] = m.maskValue(k, v)
	}
	return result
}

// maskValue applies masking to a single value based on its key.
func (m *Masker) maskValue(key string, value any) any {
	normalizedKey := strings.ToLower(key)

	switch v := value.(type) {
	case string:
		if maskFn, found := m.config.Fields[normalizedKey]; found && v != "" {
			return maskFn(v)
		}
		return v
	case map[string]any:
		return m.maskMap(v)
	case []any:
		result := make([]any, len(v))
		for i, item := range v {
			if nested, ok := item.(map[string]any); ok {
				result[i] = m.maskMap(nested)
			} else {
				result[i] = item
			}
		}
		return result
	default:
		return value
	}
}

// DefaultBRConfig returns a MaskConfig with LGPD/BR-specific field mappings.
// This is the preset that matches the current hardcoded behavior:
//
//	email                                                         -> MaskEmail
//	document, cpf, cnpj                                           -> MaskDocument
//	name, full_name, first_name, last_name, company_name, trade_name -> MaskName
//	phone, telefone                                               -> MaskPhone
func DefaultBRConfig() MaskConfig {
	return MaskConfig{
		Fields: map[string]MaskFunc{
			"email":        MaskEmail,
			"document":     MaskDocument,
			"cpf":          MaskDocument,
			"cnpj":         MaskDocument,
			"name":         MaskName,
			"full_name":    MaskName,
			"first_name":   MaskName,
			"last_name":    MaskName,
			"phone":        MaskPhone,
			"telefone":     MaskPhone,
			"company_name": MaskName,
			"trade_name":   MaskName,
		},
	}
}

// defaultMasker is the package-level masker used by MaskSensitivePayload.
var defaultMasker = NewMasker(DefaultBRConfig())

// MaskSensitivePayload recursively masks known sensitive fields in a map.
// Recognized fields: email, document, cpf, cnpj, name, *_name, phone, telefone.
// Returns the input unchanged for non-map types, or a new map with masked values.
func MaskSensitivePayload(input any) any {
	return defaultMasker.MaskPayload(input)
}

// MaskEmail masks an email address, keeping the first character and domain visible.
// Example: "user@example.com" -> "u***@example.com"
func MaskEmail(email string) string {
	if email == "" {
		return "***"
	}
	atIdx := strings.LastIndex(email, "@")
	if atIdx <= 0 {
		return "***"
	}
	return string(email[0]) + "***" + email[atIdx:]
}

// MaskDocument masks a document number, keeping only the last 4 characters visible.
// Works with any document format (CPF, CNPJ, SSN, NIF, passport, etc.).
// Example: "12345678901" -> "***8901"
func MaskDocument(value string) string {
	if len(value) <= 4 {
		return "***"
	}
	return "***" + value[len(value)-4:]
}

// MaskName masks a full name, keeping only first initials visible.
// Example: "Joao Silva" -> "J*** S***"
func MaskName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "***"
	}

	parts := strings.Fields(name)
	masked := make([]string, len(parts))
	for i, part := range parts {
		r, _ := utf8.DecodeRuneInString(part)
		if r == utf8.RuneError || utf8.RuneCountInString(part) <= 1 {
			masked[i] = part
		} else {
			masked[i] = string(r) + "***"
		}
	}
	return strings.Join(masked, " ")
}

// MaskPhone masks a phone number, keeping the country code prefix and last 4 digits visible.
// Assumes a 2-digit country code (e.g., +55 BR, +44 UK, +33 FR). Single-digit codes
// like +1 (US/Canada) will include one local digit in the visible prefix.
// Numbers with fewer than 7 digits are fully masked to prevent data overlap.
// Example: "+5511999998888" -> "+55***8888"
func MaskPhone(phone string) string {
	if phone == "" {
		return "***"
	}

	digits := make([]byte, 0, len(phone))
	prefix := ""

	for i, ch := range phone {
		if ch == '+' && i == 0 {
			prefix = "+"
			continue
		}
		if ch >= '0' && ch <= '9' {
			digits = append(digits, byte(ch))
		}
	}

	// Need at least 7 digits for prefix (2) + hidden middle (1+) + suffix (4)
	// to avoid overlap between visible parts.
	if len(digits) < 7 {
		return "***"
	}

	return prefix + string(digits[:2]) + "***" + string(digits[len(digits)-4:])
}
