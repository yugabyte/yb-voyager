// anonymizer_test.go
package anonymizer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func setupAnonymizer(t *testing.T) *SqlAnonymizer {
	dir := t.TempDir()
	metadir := filepath.Join(dir, "metainfo")
	if err := os.MkdirAll(metadir, 0o755); err != nil {
		t.Fatalf("failed to mkdir %s: %v", metadir, err)
	}
	a, err := NewSqlAnonymizer(dir)
	if err != nil {
		t.Fatalf("NewSqlAnonymizer: %v", err)
	}
	return a
}

// hasToken returns true if s contains a substring starting with "anon_"
func hasToken(s string) bool {
	return strings.Contains(s, "anon_")
}

func TestAnonTableName(t *testing.T) {
	in := "CREATE TABLE foo (id int);"
	a := setupAnonymizer(t)

	out, err := a.Anonymize(in)
	if err != nil {
		t.Fatalf("Anonymize error: %v", err)
	}
	t.Logf("\nBEFORE: %s\nAFTER : %s", in, out)

	if strings.Contains(out, "foo") {
		t.Errorf("expected 'foo' to be removed, got: %s", out)
	}
	if !hasToken(out) {
		t.Errorf("expected an anonymized token for table, got: %s", out)
	}
}

func TestAnonColumnDef(t *testing.T) {
	in := "CREATE TABLE tbl (secret_col text);"
	a := setupAnonymizer(t)

	out, err := a.Anonymize(in)
	if err != nil {
		t.Fatalf("Anonymize error: %v", err)
	}
	t.Logf("\nBEFORE: %s\nAFTER : %s", in, out)

	if strings.Contains(out, "secret_col") {
		t.Errorf("expected 'secret_col' to be removed, got: %s", out)
	}
	if !hasToken(out) {
		t.Errorf("expected an anonymized token for column, got: %s", out)
	}
}

func TestAnonColumnRef(t *testing.T) {
	in := "SELECT password FROM users;"
	a := setupAnonymizer(t)

	out, err := a.Anonymize(in)
	if err != nil {
		t.Fatalf("Anonymize error: %v", err)
	}
	t.Logf("\nBEFORE: %s\nAFTER : %s", in, out)

	if strings.Contains(out, "password") {
		t.Errorf("expected 'password' to be removed, got: %s", out)
	}
	if !hasToken(out) {
		t.Errorf("expected an anonymized token for column ref, got: %s", out)
	}
}

func TestAnonResTarget(t *testing.T) {
	in := "SELECT email AS user_email FROM accounts;"
	a := setupAnonymizer(t)

	out, err := a.Anonymize(in)
	if err != nil {
		t.Fatalf("Anonymize error: %v", err)
	}
	t.Logf("\nBEFORE: %s\nAFTER : %s", in, out)

	if strings.Contains(out, "user_email") {
		t.Errorf("expected 'user_email' to be removed, got: %s", out)
	}
	if !hasToken(out) {
		t.Errorf("expected an anonymized token for alias, got: %s", out)
	}
}

func TestAnonIndexStmt(t *testing.T) {
	in := "CREATE INDEX idx_foo ON mytable(bar);"
	a := setupAnonymizer(t)

	out, err := a.Anonymize(in)
	if err != nil {
		t.Fatalf("Anonymize error: %v", err)
	}
	t.Logf("\nBEFORE: %s\nAFTER : %s", in, out)

	if strings.Contains(out, "idx_foo") || strings.Contains(out, "mytable") {
		t.Errorf("expected 'idx_foo' and 'mytable' removed, got: %s", out)
	}
	if !hasToken(out) {
		t.Errorf("expected anonymized tokens for index and table, got: %s", out)
	}
}

func TestAnonConstraint(t *testing.T) {
	in := "CREATE TABLE t (id int CONSTRAINT pk_t PRIMARY KEY);"
	a := setupAnonymizer(t)

	out, err := a.Anonymize(in)
	if err != nil {
		t.Fatalf("Anonymize error: %v", err)
	}
	t.Logf("\nBEFORE: %s\nAFTER : %s", in, out)

	if strings.Contains(out, "pk_t") {
		t.Errorf("expected 'pk_t' removed, got: %s", out)
	}
	if !hasToken(out) {
		t.Errorf("expected an anonymized token for constraint, got: %s", out)
	}
}

func TestAnonAliasNode(t *testing.T) {
	in := "SELECT * FROM orders o WHERE o.amount > 100;"
	a := setupAnonymizer(t)

	out, err := a.Anonymize(in)
	if err != nil {
		t.Fatalf("Anonymize error: %v", err)
	}
	t.Logf("\nBEFORE: %s\nAFTER : %s", in, out)

	if strings.Contains(out, " orders ") || strings.Contains(out, " o ") {
		t.Errorf("expected 'orders' and alias 'o' removed, got: %s", out)
	}
	if !hasToken(out) {
		t.Errorf("expected anonymized tokens for table and alias, got: %s", out)
	}
}

func TestAnonCombined(t *testing.T) {
	in := `
CREATE TABLE users (
  id int PRIMARY KEY,
  email varchar(255) CONSTRAINT uq_email UNIQUE
);
`
	a := setupAnonymizer(t)

	out, err := a.Anonymize(in)
	if err != nil {
		t.Fatalf("Anonymize error: %v", err)
	}
	t.Logf("BEFORE:\n%s\nAFTER:\n%s", in, out)

	// check each original is gone
	for _, orig := range []string{"users", "id", "email", "uq_email", "idx_users_email", "user_id", "user_email", "u"} {
		if strings.Contains(out, orig) {
			t.Errorf("expected %q removed, but found in %s", orig, out)
		}
	}
	if !hasToken(out) {
		t.Errorf("expected at least one anonymized token, got: %s", out)
	}
}
