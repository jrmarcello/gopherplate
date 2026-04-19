package commands

import (
	"strings"
	"testing"
)

func TestValidateServiceName_TC_E2E_12(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"lowercase letters ok", "myservice", false},
		{"with digits ok", "service42", false},
		{"with hyphen ok", "my-service", false},
		{"single letter ok", "a", false},
		{"starts with digit", "2service", true},
		{"starts with hyphen", "-service", true},
		{"uppercase rejected", "MyService", true},
		{"spaces rejected", "my service", true},
		{"special chars rejected", "my_service", true},
		{"dots rejected", "my.service", true},
		{"path traversal rejected", "../escape", true},
		{"empty rejected", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateServiceName(tc.input)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateServiceName(%q) err = %v, wantErr %v", tc.input, err, tc.wantErr)
			}
		})
	}
}

func TestResolveFlavor_TC_UC_03(t *testing.T) {
	t.Run("TC-UC-03 empty input defaults to crud", func(t *testing.T) {
		got, err := resolveFlavor("")
		if err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		if got.ID != "crud" {
			t.Errorf("default flavor = %q, want crud", got.ID)
		}
	})

	t.Run("TC-E2E-07 unknown flavor returns error listing available", func(t *testing.T) {
		_, err := resolveFlavor("foo")
		if err == nil {
			t.Fatalf("expected error for unknown flavor")
		}
		if !strings.Contains(err.Error(), "foo") {
			t.Errorf("error should mention the invalid flavor, got: %v", err)
		}
		if !strings.Contains(err.Error(), "crud") {
			t.Errorf("error should list available flavors (crud), got: %v", err)
		}
	})

	t.Run("explicit crud succeeds", func(t *testing.T) {
		got, err := resolveFlavor("crud")
		if err != nil || got.ID != "crud" {
			t.Errorf("resolveFlavor('crud') = (%v, %v), want (crud, nil)", got, err)
		}
	})
}
