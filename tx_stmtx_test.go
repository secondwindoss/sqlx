package sqlx

import (
	"context"
	"testing"
)

// Tx.Stmtx and Tx.StmtxContext keep the transaction's unsafe flag, and keep
// the wrapped statement's own unsafe flag (upstream #979).
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

		tx, err := db.Unsafe().Beginx()
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		if !tx.unsafe {
			t.Fatal("precondition: tx from db.Unsafe() should be unsafe")
		}
		ts := tx.Stmtx(stmt)
		if !ts.unsafe {
			t.Error("Stmtx: unsafe Tx returned a safe Stmt")
		}
		if err := ts.Get(&partial, "Jason"); err != nil {
			t.Errorf("Stmtx: Get into partial struct: %v", err)
		}
		tsc := tx.StmtxContext(ctx, stmt)
		if !tsc.unsafe {
			t.Error("StmtxContext: unsafe Tx returned a safe Stmt")
		}
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
		if !us.unsafe {
			t.Error("Stmtx: unsafe Stmt wrapped in a safe Tx became safe")
		}
		if err := us.Get(&partial, "Jason"); err != nil {
			t.Errorf("Stmtx: Get through unsafe Stmt in safe Tx: %v", err)
		}
		if ss := safeTx.StmtxContext(ctx, stmt); ss.unsafe {
			t.Error("StmtxContext: safe Stmt in safe Tx became unsafe")
		}
	})
}
