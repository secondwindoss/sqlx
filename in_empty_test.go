package sqlx

import (
	"database/sql/driver"
	"reflect"
	"strings"
	"testing"
)

type emptySliceValuer struct{}

func (emptySliceValuer) Value() (driver.Value, error) { return []int{}, nil }

// An empty slice expands to NULL and binds nothing, after IN and NOT IN alike
// (upstream #945).
func TestInEmptySlice(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		args     []interface{}
		wantQ    string
		wantArgs []interface{}
		wantErr  string
	}{
		{"empty", "SELECT * FROM t WHERE x IN (?)", []interface{}{[]int{}}, "SELECT * FROM t WHERE x IN (NULL)", []interface{}{}, ""},
		{"nil slice", "SELECT * FROM t WHERE x IN (?)", []interface{}{[]int(nil)}, "SELECT * FROM t WHERE x IN (NULL)", []interface{}{}, ""},
		{"scalar before", "SELECT * FROM t WHERE a = ? AND x IN (?)", []interface{}{1, []int{}}, "SELECT * FROM t WHERE a = ? AND x IN (NULL)", []interface{}{1}, ""},
		{"scalar after", "SELECT * FROM t WHERE x IN (?) AND a = ?", []interface{}{[]int{}, 1}, "SELECT * FROM t WHERE x IN (NULL) AND a = ?", []interface{}{1}, ""},
		{"empty then full", "SELECT * FROM t WHERE x IN (?) AND y IN (?)", []interface{}{[]int{}, []int{1, 2}}, "SELECT * FROM t WHERE x IN (NULL) AND y IN (?, ?)", []interface{}{1, 2}, ""},
		{"full then empty", "SELECT * FROM t WHERE x IN (?) AND y IN (?)", []interface{}{[]int{1, 2}, []int{}}, "SELECT * FROM t WHERE x IN (?, ?) AND y IN (NULL)", []interface{}{1, 2}, ""},
		{"two empty", "SELECT * FROM t WHERE x IN (?) AND y IN (?)", []interface{}{[]int{}, []string{}}, "SELECT * FROM t WHERE x IN (NULL) AND y IN (NULL)", []interface{}{}, ""},
		{"no space", "SELECT * FROM t WHERE x IN(?)", []interface{}{[]int{}}, "SELECT * FROM t WHERE x IN(NULL)", []interface{}{}, ""},
		{"inner spaces", "SELECT * FROM t WHERE x IN ( ? )", []interface{}{[]int{}}, "SELECT * FROM t WHERE x IN ( NULL )", []interface{}{}, ""},
		{"empty interface slice", "SELECT * FROM t WHERE x IN (?)", []interface{}{[]interface{}{}}, "SELECT * FROM t WHERE x IN (NULL)", []interface{}{}, ""},
		{"empty bytes are a value", "SELECT * FROM t WHERE x IN (?)", []interface{}{[]byte{}}, "SELECT * FROM t WHERE x IN (?)", []interface{}{[]byte{}}, ""},
		{"valuer returning an empty slice", "SELECT * FROM t WHERE x IN (?)", []interface{}{emptySliceValuer{}}, "SELECT * FROM t WHERE x IN (NULL)", []interface{}{}, ""},
		{"too few args", "SELECT * FROM t WHERE x IN (?) AND y = ?", []interface{}{[]int{}}, "", nil, "number of bindVars exceeds arguments"},
		{"too many args", "SELECT * FROM t WHERE x IN (?)", []interface{}{[]int{}, 1}, "", nil, "number of bindVars less than number arguments"},
		{"not in", "SELECT * FROM t WHERE x NOT IN (?)", []interface{}{[]string{}}, "SELECT * FROM t WHERE x NOT IN (NULL)", []interface{}{}, ""},
		{"not in with a scalar", "DELETE FROM server_config WHERE server_id = ? AND config_id not in(?)", []interface{}{7, []int{}}, "DELETE FROM server_config WHERE server_id = ? AND config_id not in(NULL)", []interface{}{7}, ""},
		{"not in with a full slice", "SELECT * FROM t WHERE x NOT IN (?)", []interface{}{[]int{1}}, "SELECT * FROM t WHERE x NOT IN (?)", []interface{}{1}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, args, err := In(tt.query, tt.args...)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want one containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if q != tt.wantQ {
				t.Errorf("query = %q, want %q", q, tt.wantQ)
			}
			if !reflect.DeepEqual(args, tt.wantArgs) {
				t.Errorf("args = %#v, want %#v", args, tt.wantArgs)
			}
		})
	}
}

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
