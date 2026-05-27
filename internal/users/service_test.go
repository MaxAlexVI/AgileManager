package users

import (
	"testing"

	"agile-manager/internal/shared"
)

func TestManagerCanCreateUser(t *testing.T) {
	store := shared.NewMemoryStore(shared.SeedData())
	service := New(store)
	manager, ok := shared.FindUser(store.State().Users, "USR-001")
	if !ok {
		t.Fatal("seed manager USR-001 not found")
	}

	user, err := service.Create(manager, shared.UserInput{
		Name:   "Новый участник",
		RoleID: shared.RoleDeveloper,
		Email:  "new@example.local",
	})
	if err != nil {
		t.Fatalf("Create user: %v", err)
	}
	if user.ID == "" {
		t.Fatal("created user has empty ID")
	}
	if user.RoleID != shared.RoleDeveloper {
		t.Fatalf("role = %q, want %q", user.RoleID, shared.RoleDeveloper)
	}
	if user.Password != "" {
		t.Fatal("created user response exposes password")
	}
}
