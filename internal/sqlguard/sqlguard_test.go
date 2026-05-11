package sqlguard

import "testing"

func TestCheckReadOnlyAppendsLimit(t *testing.T) {
	checked, err := CheckReadOnly("select * from users", 50)
	if err != nil {
		t.Fatal(err)
	}
	if checked.SQL != "select * from users LIMIT 50" {
		t.Fatalf("unexpected sql: %s", checked.SQL)
	}
}

func TestCheckReadOnlyRejectsWrites(t *testing.T) {
	for _, sql := range []string{
		"update users set name = 'x'",
		"delete from users",
		"drop table users",
		"select 1; drop table users",
	} {
		if _, err := CheckReadOnly(sql, 50); err == nil {
			t.Fatalf("expected rejection for %q", sql)
		}
	}
}

func TestCheckExplainableRejectsNonSelect(t *testing.T) {
	if _, err := CheckExplainable("show tables"); err == nil {
		t.Fatal("expected non-select explain rejection")
	}
}
