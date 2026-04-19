package flavors

import (
	"strings"
	"testing"
)

func TestRegistry_RegisterAndGet(t *testing.T) {
	reg := NewRegistry()

	crud := Flavor{ID: "crud", Description: "HTTP+gRPC CRUD (default)"}
	if err := reg.Register(crud); err != nil {
		t.Fatalf("register crud: unexpected error %v", err)
	}

	t.Run("TC-UC-01 Get('crud') returns registered flavor", func(t *testing.T) {
		got, getErr := reg.Get("crud")
		if getErr != nil {
			t.Fatalf("unexpected error: %v", getErr)
		}
		if got.ID != "crud" {
			t.Errorf("Get().ID = %q, want crud", got.ID)
		}
	})

	t.Run("TC-UC-02 Get('invalid') returns error listing flavors", func(t *testing.T) {
		_, getErr := reg.Get("invalid")
		if getErr == nil {
			t.Fatalf("expected error for unknown flavor")
		}
		msg := getErr.Error()
		if !strings.Contains(msg, "invalid") {
			t.Errorf("error should mention the invalid flavor id, got: %s", msg)
		}
		if !strings.Contains(msg, "crud") {
			t.Errorf("error should list registered flavors, got: %s", msg)
		}
	})

	t.Run("TC-UC-15 Register duplicate returns error", func(t *testing.T) {
		dup := Flavor{ID: "crud", Description: "duplicate"}
		if err := reg.Register(dup); err == nil {
			t.Errorf("expected error registering duplicate id 'crud'")
		}
	})
}

func TestRegistry_List_IsSortedAndStable(t *testing.T) {
	reg := NewRegistry()

	for _, id := range []string{"data-pipeline", "crud", "event-processor"} {
		if err := reg.Register(Flavor{ID: id}); err != nil {
			t.Fatalf("register %q: %v", id, err)
		}
	}

	names := reg.List()
	want := []string{"crud", "data-pipeline", "event-processor"}

	if len(names) != len(want) {
		t.Fatalf("List() len = %d, want %d", len(names), len(want))
	}
	for i, name := range names {
		if name != want[i] {
			t.Errorf("List()[%d] = %q, want %q (names must be sorted for deterministic --help output)",
				i, name, want[i])
		}
	}
}
