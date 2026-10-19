package dbtest

import (
	"testing"

	. "github.com/secondwindoss/sqlx"
)

// Scanning a second result set with a different column layout must not reuse
// the first set's field cache (upstream #380, #857; test from upstream #747).
func TestMultiResultSet(t *testing.T) {
	RunWithSchema(defaultSchema, t, func(db *DB, t *testing.T, now string) {
		var sp, sql string

		loadDefaultFixture(db, t)

		switch db.DriverName() {
		case "mysql":
			sp = `
				CREATE PROCEDURE multiQuery()
				BEGIN
					SELECT first_name, last_name, email FROM person LIMIT 1;
					SELECT country, telcode FROM place LIMIT 1;
				END
			`
			sql = "CALL multiQuery();"
		case "postgres":
			sql = `
				SELECT first_name, last_name, email FROM person LIMIT 1;
				SELECT country, telcode FROM place LIMIT 1;
			`
		default:
			// sqlite3 does not support multiple result sets.
			return
		}

		type Person struct {
			FirstName string `db:"first_name"`
			LastName  string `db:"last_name"`
			Email     string `db:"email"`
		}
		type Place struct {
			Country string `db:"country"`
			TelCode string `db:"telcode"`
		}

		if sp != "" {
			if _, err := db.Exec("DROP PROCEDURE IF EXISTS multiQuery"); err != nil {
				t.Fatal(err)
			}

			if _, err := db.Exec(sp); err != nil {
				t.Fatal(err)
			}
		}

		rows, err := db.Queryx(sql)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()

		var (
			person Person
			place  Place
		)

		var i int
		for rows.Next() {
			i++
			if err = rows.StructScan(&person); err != nil {
				t.Fatal(err)
			}
		}
		if i < 1 {
			t.Errorf("[%s] Expected at least one record, got none", db.DriverName())
		}

		if !rows.NextResultSet() {
			t.Fatalf("[%s] Expected a second recordset, got one", db.DriverName())
		}

		i = 0
		for rows.Next() {
			i++
			if err = rows.StructScan(&place); err != nil {
				t.Fatal(err)
			}
		}
		if i < 1 {
			t.Errorf("Expected at least one records, got none")
		}

		// nest no next
		rows2, err := db.Queryx("SELECT first_name, last_name, email FROM person LIMIT 2;")
		if err != nil {
			t.Fatal(err)
		}
		defer rows2.Close()

		i = 0
		for rows2.Next() {
			i++
			if err = rows2.StructScan(&person); err != nil {
				t.Fatal(err)
			}
		}
		if i < 2 {
			t.Errorf("[%s] Expected at least 2 records, got %d", db.DriverName(), i)
		}
		if rows.NextResultSet() {
			t.Fatalf("[%s] Did not expected a second recordset", db.DriverName())
		}
	})
}
