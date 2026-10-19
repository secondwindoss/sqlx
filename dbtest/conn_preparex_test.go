package dbtest

import (
	"context"
	"strings"
	"testing"

	. "github.com/secondwindoss/sqlx"

	"github.com/secondwindoss/sqlx/reflectx"
)

// A Stmt prepared on a Conn inherits the Conn's Mapper and unsafe flag
// (upstream #975). The unsafe flag is checked through its effect: scanning
// into a struct that lacks some of the selected columns.
func TestConnPreparexKeepsMapperAndUnsafe(t *testing.T) {
	RunWithSchema(defaultSchema, t, func(db *DB, t *testing.T, now string) {
		loadDefaultFixture(db, t)
		ctx := context.Background()

		conn, err := db.Unsafe().Connx(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		stmt, err := conn.PreparexContext(ctx, conn.Rebind("SELECT * FROM person WHERE first_name=?"))
		if err != nil {
			t.Fatal(err)
		}
		defer stmt.Close()
		var partial struct {
			FirstName string `db:"first_name"`
		}
		if err := stmt.Get(&partial, "Jason"); err != nil {
			t.Errorf("unsafe: Get into partial struct through conn-prepared Stmt: %v", err)
		}

		// Control: the same statement on a safe Conn rejects the partial struct.
		safeConn, err := db.Connx(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer safeConn.Close()
		safeStmt, err := safeConn.PreparexContext(ctx, safeConn.Rebind("SELECT * FROM person WHERE first_name=?"))
		if err != nil {
			t.Fatal(err)
		}
		defer safeStmt.Close()
		if err := safeStmt.Get(&partial, "Jason"); err == nil {
			t.Error("safe: Get into partial struct through a safe Conn's Stmt succeeded, want missing destination error")
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
