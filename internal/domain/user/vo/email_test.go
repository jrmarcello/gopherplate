package vo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEmail_Valid(t *testing.T) {
	testCases := []struct {
		input string
	}{
		{"user@example.com"},
		{"user.name@example.com"},
		{"user+tag@example.com"},
		{"user@subdomain.example.com"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			email, err := NewEmail(tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.input, email.String())
		})
	}
}

func TestNewEmail_Invalid(t *testing.T) {
	testCases := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"no @", "userexample.com"},
		{"no domain", "user@"},
		{"no local", "@example.com"},
		{"spaces", "user @example.com"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewEmail(tc.input)
			assert.ErrorIs(t, err, ErrInvalidEmail)
		})
	}
}

func TestEmail_ScanValue(t *testing.T) {
	email, _ := NewEmail("test@example.com")

	// Test Value
	value, err := email.Value()
	require.NoError(t, err)
	assert.Equal(t, "test@example.com", value)

	// Test Scan from string
	var scanned Email
	err = scanned.Scan("scanned@example.com")
	require.NoError(t, err)
	assert.Equal(t, "scanned@example.com", scanned.String())

	// Test Scan from []byte
	var scannedBytes Email
	err = scannedBytes.Scan([]byte("bytes@example.com"))
	require.NoError(t, err)
	assert.Equal(t, "bytes@example.com", scannedBytes.String())
}

func TestEmail_Scan_Error(t *testing.T) {
	var email Email

	err := email.Scan(nil)
	assert.Error(t, err)

	err = email.Scan(123)
	assert.Error(t, err)
}

func TestParseEmail(t *testing.T) {
	t.Run("valid email string", func(t *testing.T) {
		email := ParseEmail("user@example.com")
		assert.Equal(t, "user@example.com", email.String())
	})

	t.Run("empty string still creates Email", func(t *testing.T) {
		email := ParseEmail("")
		assert.Equal(t, "", email.String())
	})

	t.Run("String returns the value passed", func(t *testing.T) {
		raw := "any-value-without-validation"
		email := ParseEmail(raw)
		assert.Equal(t, raw, email.String())
	})
}
