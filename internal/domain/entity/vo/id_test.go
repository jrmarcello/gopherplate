package vo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewID(t *testing.T) {
	id := NewID()

	// ID deve ser ULID válido (26 caracteres)
	assert.Len(t, id.String(), 26)
	assert.NotEmpty(t, id.String())

	// Cada chamada gera ID único
	id2 := NewID()
	assert.NotEqual(t, id, id2)
}

func TestParseID_Valid(t *testing.T) {
	// Gera um ID válido para testar
	original := NewID()

	parsed, err := ParseID(original.String())

	require.NoError(t, err)
	assert.Equal(t, original, parsed)
}

func TestParseID_Invalid(t *testing.T) {
	testCases := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"too short", "01ARZ3NDEKT"},
		{"random string", "invalid-id-format"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseID(tc.input)
			assert.Error(t, err)
		})
	}
}

func TestID_ScanValue(t *testing.T) {
	original := NewID()

	// Test Value
	value, err := original.Value()
	require.NoError(t, err)
	assert.Equal(t, original.String(), value)

	// Test Scan from string
	var scanned ID
	err = scanned.Scan(original.String())
	require.NoError(t, err)
	assert.Equal(t, original, scanned)

	// Test Scan from []byte
	var scannedBytes ID
	err = scannedBytes.Scan([]byte(original.String()))
	require.NoError(t, err)
	assert.Equal(t, original, scannedBytes)
}

func TestID_Scan_Error(t *testing.T) {
	var id ID

	err := id.Scan(nil)
	assert.Error(t, err)

	err = id.Scan(123)
	assert.Error(t, err)
}
