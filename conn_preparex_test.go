package sqlx

import (
	"context"
	"strings"
	"testing"

	"github.com/secondwindoss/sqlx/reflectx"
)

// A Stmt prepared on a Conn inherits the Conn's Mapper and unsafe flag
// (upstream #975).
func TestConnPreparexKeepsMapperAndUnsafe(t *testing.T) {
	RunWithSchema(defaultSchema, t, func(db *DB, t *testing.T, now string) {
		loadDefaultFixture(db, t)
		ctx := context.Background()

		conn, err := db.Unsafe().Connx(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		if !conn.unsafe {
			t.Fatal("precondition: conn from db.Unsafe() should be unsafe")
		}
		stmt, err := conn.PreparexContext(ctx, conn.Rebind("SELECT * FROM person WHERE first_name=?"))
		if err != nil {
			t.Fatal(err)
		}
		defer stmt.Close()
		if stmt.unsafe != conn.unsafe {
			t.Errorf("unsafe: Stmt.unsafe=%v, want %v (conn.unsafe)", stmt.unsafe, conn.unsafe)
		}
		var partial struct {
			FirstName string `db:"first_name"`
		}
		if err := stmt.Get(&partial, "Jason"); err != nil {
			t.Errorf("unsafe: Get into partial struct through conn-prepared Stmt: %v", err)
		}

		conn2, err := db.Connx(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer conn2.Close()
		conn2.Mapper = reflectx.NewMapperFunc("json", strings.ToLower)
		stmt2, err := conn2.PreparexContext(ctx, conn2.Rebind("SELECT first_name, last_name FROM person WHERE first_name=?"))
		if err != nil {
			t.Fatal(err)
		}
		defer stmt2.Close()
		if stmt2.Mapper != conn2.Mapper {
			t.Errorf("mapper: Stmt.Mapper was not taken from Conn.Mapper")
		}
		var p struct {
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
		}
		if err := stmt2.Get(&p, "Jason"); err != nil {
			t.Errorf("mapper: Get with json-tag mapper set on Conn: %v", err)
		}
	})
}
