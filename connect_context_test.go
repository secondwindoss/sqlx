package sqlx

import (
	"context"
	"testing"
)

// ConnectContext closes the DB and returns nil when the ping fails
// (upstream #910).
func TestConnectContextClosesOnPingFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	db, err := ConnectContext(ctx, "sqlite3", ":memory:")
	if err == nil {
		t.Fatal("expected a ping error with a cancelled context")
	}
	if db != nil {
		t.Errorf("ConnectContext returned a non-nil *DB together with the error: %v", err)
		db.Close()
	}
}
