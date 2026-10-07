package store

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestBoundedMigrationErrorMessage(t *testing.T) {
	t.Run("short diagnostic remains unchanged", func(t *testing.T) {
		const message = "migration failed: relation already exists"
		if got := BoundedMigrationErrorMessage("  " + message + "  "); got != message {
			t.Fatalf("bounded short diagnostic=%q want=%q", got, message)
		}
	})

	t.Run("long unicode diagnostic retains phase and root cause", func(t *testing.T) {
		message := "execute migration statement: " + strings.Repeat("表", 1800) + ": pq: relation already exists"
		got := BoundedMigrationErrorMessage(message)
		if utf8.RuneCountInString(got) != migrationErrorMessageMaxCharacters {
			t.Fatalf("bounded diagnostic characters=%d want=%d", utf8.RuneCountInString(got), migrationErrorMessageMaxCharacters)
		}
		if !strings.HasPrefix(got, "execute migration statement: ") {
			t.Fatalf("bounded diagnostic lost phase prefix: %q", got[:64])
		}
		if !strings.Contains(got, migrationErrorMessageOmission) {
			t.Fatal("bounded diagnostic lacks an explicit truncation marker")
		}
		if !strings.HasSuffix(got, ": pq: relation already exists") {
			t.Fatalf("bounded diagnostic lost database root cause: %q", got[len(got)-64:])
		}
	})
}
