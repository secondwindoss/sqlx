package dbtest

import (
	"context"
	"testing"

	. "github.com/secondwindoss/sqlx"
)

// Tx.Stmtx and Tx.StmtxContext keep the transaction's unsafe flag, and keep
// the wrapped statement's own unsafe flag (upstream #979). The flag is checked
// through its effect: an unsafe Stmt scans into a struct that lacks some of the
// selected columns, a safe one returns a missing destination error.
func TestTxStmtxKeepsUnsafe(t *testing.T) {
	RunWithSchema(defaultSchema, t, func(db *DB, t *testing.T, now string) {
		loadDefaultFixture(db, t)
		ctx := context.Background()
		stmt, err := db.Preparex(db.Rebind("SELECT * FROM person WHERE first_name=?"))
		if err != nil {
			t.Fatal(err)
		}
		defer stmt.Close()

		var partial struct {
			FirstName string `db:"first_name"`
		}
		if err := stmt.Get(&partial, "Jason"); err == nil {
			t.Fatal("precondition: Get into partial struct through a safe Stmt succeeded")
		}

		tx, err := db.Unsafe().Beginx()
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		ts := tx.Stmtx(stmt)
		if err := ts.Get(&partial, "Jason"); err != nil {
			t.Errorf("Stmtx: Get into partial struct: %v", err)
		}
		tsc := tx.StmtxContext(ctx, stmt)
		if err := tsc.GetContext(ctx, &partial, "Jason"); err != nil {
			t.Errorf("StmtxContext: Get into partial struct: %v", err)
		}

		// An unsafe statement wrapped in a safe transaction stays unsafe.
		safeTx, err := db.Beginx()
		if err != nil {
			t.Fatal(err)
		}
		defer safeTx.Rollback()
		us := safeTx.Stmtx(stmt.Unsafe())
		if err := us.Get(&partial, "Jason"); err != nil {
			t.Errorf("Stmtx: Get through unsafe Stmt in safe Tx: %v", err)
		}
		if err := safeTx.StmtxContext(ctx, stmt).GetContext(ctx, &partial, "Jason"); err == nil {
			t.Error("StmtxContext: safe Stmt in safe Tx became unsafe (Get into partial struct succeeded)")
		}
	})
}
