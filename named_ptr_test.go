package sqlx

import "testing"

// Named queries accept a pointer to a slice, to a map and to a slice of maps
// (upstream #577, #729).
func TestNamedPtrArgs(t *testing.T) {
	RunWithSchema(defaultSchema, t, func(db *DB, t *testing.T, now string) {
		const insert = "INSERT INTO person (first_name, last_name, email) VALUES (:first_name, :last_name, :email)"
		try := func(label string, arg interface{}, want int64) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s: panicked: %v", label, r)
				}
			}()
			res, err := db.NamedExec(insert, arg)
			if err != nil {
				t.Errorf("%s: %v", label, err)
				return
			}
			if n, err := res.RowsAffected(); err == nil && n != want {
				t.Errorf("%s: %d rows affected, want %d", label, n, want)
			}
		}
		people := []Person{{FirstName: "A", LastName: "B", Email: "a@b"}, {FirstName: "C", LastName: "D", Email: "c@d"}}
		try("*[]struct", &people, 2)
		m := map[string]interface{}{"first_name": "E", "last_name": "F", "email": "e@f"}
		try("*map", &m, 1)
		ms := []map[string]interface{}{{"first_name": "G", "last_name": "H", "email": "g@h"}, {"first_name": "I", "last_name": "J", "email": "i@j"}}
		try("*[]map", &ms, 2)

		var count int
		if err := db.Get(&count, "SELECT count(*) FROM person"); err != nil {
			t.Fatal(err)
		}
		if count != 5 {
			t.Errorf("expected 5 people inserted, got %d", count)
		}
	})
}
