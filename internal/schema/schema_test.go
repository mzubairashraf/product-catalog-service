package schema

import (
	"os"
	"strings"
	"testing"
)

func TestStatementsFromTheMigrationFile(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/001_initial_schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	got := Statements(string(raw))

	wantPrefixes := []string{
		"CREATE TABLE products", "CREATE TABLE outbox_events",
		"CREATE INDEX idx_outbox_status", "CREATE INDEX idx_products_category", "CREATE INDEX idx_products_status",
	}
	if len(got) != len(wantPrefixes) {
		t.Fatalf("statements = %d, want %d:\n%s", len(got), len(wantPrefixes), strings.Join(got, "\n---\n"))
	}
	for i, prefix := range wantPrefixes {
		if !strings.HasPrefix(got[i], prefix) {
			t.Errorf("statement %d = %q, want prefix %q", i, got[i], prefix)
		}
		if strings.Contains(got[i], "--") || strings.HasSuffix(got[i], ";") {
			t.Errorf("statement %d still has a comment or terminator: %q", i, got[i])
		}
	}
}
