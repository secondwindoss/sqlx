package dbtest

import (
	"testing"

	. "github.com/secondwindoss/sqlx"
)

func TestInEmptySliceAgainstDatabase(t *testing.T) {
	RunWithSchema(defaultSchema, t, func(db *DB, t *testing.T, now string) {
		loadDefaultFixture(db, t)
		for _, tt := range []struct {
			query string
			args  []interface{}
		}{
			{"SELECT * FROM place WHERE telcode IN (?)", []interface{}{[]int{}}},
			{"SELECT * FROM place WHERE country = ? AND telcode IN (?)", []interface{}{"Hong Kong", []int{}}},
			{"SELECT * FROM place WHERE telcode IN (?) AND country = ?", []interface{}{[]int{}, "Hong Kong"}},
			// NOT IN (NULL) matches no row either; this documents the caveat.
			{"SELECT * FROM place WHERE telcode NOT IN (?)", []interface{}{[]int{}}},
		} {
			q, args, err := In(tt.query, tt.args...)
			if err != nil {
				t.Fatalf("%s: %v", tt.query, err)
			}
			var places []Place
			if err := db.Select(&places, db.Rebind(q), args...); err != nil {
				t.Fatalf("%s: %v", tt.query, err)
			}
			if len(places) != 0 {
				t.Errorf("%s: got %d rows, want none", tt.query, len(places))
			}
		}
	})
}
