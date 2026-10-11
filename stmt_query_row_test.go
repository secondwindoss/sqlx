package sqlx

import (
	"context"
	"testing"
)

// *Stmt promotes (*sql.Stmt).QueryRowContext(ctx, args...): the first
// variadic argument is a bind argument, not a query. Guards against a method
// on *Stmt shadowing it (the mistake in upstream #941).
func TestStmtQueryRowContextArgs(t *testing.T) {
	RunWithSchema(defaultSchema, t, func(db *DB, t *testing.T, now string) {
		loadDefaultFixture(db, t)
		ctx := context.Background()
		stmt, err := db.PreparexContext(ctx, db.Rebind("SELECT last_name FROM person WHERE first_name=?"))
		if err != nil {
			t.Fatal(err)
		}
		defer stmt.Close()
		var last string
		if err := stmt.QueryRowContext(ctx, "Jason").Scan(&last); err != nil || last != "Moiron" {
			t.Errorf("stmt.QueryRowContext(ctx, \"Jason\").Scan: err=%v last=%q, want nil and Moiron", err, last)
		}
		if err := stmt.QueryRow("Jason").Scan(&last); err != nil || last != "Moiron" {
			t.Errorf("stmt.QueryRow(\"Jason\").Scan: err=%v last=%q, want nil and Moiron", err, last)
		}
	})
}
