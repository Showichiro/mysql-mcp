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

func TestCheckAllowsWritesWhenConfigured(t *testing.T) {
	for _, sql := range []string{
		"insert into users (name) values ('x')",
		"update users set name = 'x' where id = 1",
		"delete from users where id = 1",
		"replace into users (id, name) values (1, 'x')",
	} {
		checked, err := Check(sql, 50, true)
		if err != nil {
			t.Fatalf("expected write to be allowed for %q: %v", sql, err)
		}
		if checked.Kind != KindWrite {
			t.Fatalf("expected write kind for %q, got %s", sql, checked.Kind)
		}
	}
}

func TestCheckRejectsAdministrativeSQLWhenWritesAllowed(t *testing.T) {
	for _, sql := range []string{
		"create table users (id int)",
		"drop table users",
		"truncate table users",
		"grant select on *.* to 'u'@'%'",
		"start transaction",
	} {
		if _, err := Check(sql, 50, true); err == nil {
			t.Fatalf("expected administrative SQL rejection for %q", sql)
		}
	}
}

func TestCheckExplainableRejectsNonSelect(t *testing.T) {
	if _, err := CheckExplainable("show tables"); err == nil {
		t.Fatal("expected non-select explain rejection")
	}
}
