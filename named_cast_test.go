package sqlx

import "testing"

// A Postgres cast attached to a named parameter binds the parameter and keeps
// the cast (upstream #983); a cast elsewhere in the query is left alone
// (upstream #428, #650, #956). Other databases have no :: operator.
func TestNamedPostgresTypeCast(t *testing.T) {
	RunWithSchema(defaultSchema, t, func(db *DB, t *testing.T, now string) {
		if db.DriverName() != "postgres" {
			return // the :: cast is Postgres syntax
		}
		loadDefaultFixture(db, t)

		var got struct {
			Code  string `db:"code"`
			Day   string `db:"day"`
			Count int    `db:"count"`
		}
		rows, err := db.NamedQuery(`
			SELECT :tel::text AS code, (now() AT TIME ZONE 'utc')::date::text AS day, count(*)::int AS count
			FROM place WHERE telcode = :tel::int`,
			map[string]interface{}{"tel": 852})
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		if !rows.Next() {
			t.Fatal("expected one row")
		}
		if err := rows.StructScan(&got); err != nil {
			t.Fatal(err)
		}
		if got.Code != "852" || got.Count != 1 || len(got.Day) != 10 {
			t.Errorf("got %+v", got)
		}
	})
}
