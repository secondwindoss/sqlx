package sqlx_test

// The golden corpus: realistic queries run through BindNamed, In and Rebind.
// goldenRender writes one line per case and bind type; testdata/golden.txt
// holds the reviewed output and TestGolden compares against it. The same
// file, with the import path changed, renders the corpus against any other
// sqlx version for a diff.

import (
	"database/sql/driver"
	"fmt"
	"sort"
	"strings"

	"github.com/secondwindoss/sqlx"
)

type goldenPerson struct {
	FirstName string `db:"first_name"`
	LastName  string `db:"last_name"`
	Email     string `db:"email"`
}

type goldenPlace struct {
	Country string `db:"country"`
	City    string `db:"city"`
}

type goldenBooking struct {
	Person goldenPerson `db:"person"`
	Place  goldenPlace  `db:"place"`
	Nights int          `db:"nights"`
}

type goldenValuer struct{ v string }

func (g goldenValuer) Value() (driver.Value, error) { return g.v, nil }

type goldenSliceValuer struct{}

func (goldenSliceValuer) Value() (driver.Value, error) { return []int{7, 8}, nil }

var goldenBindTypes = []struct {
	name string
	bt   int
}{
	{"QUESTION", sqlx.QUESTION},
	{"DOLLAR", sqlx.DOLLAR},
	{"NAMED", sqlx.NAMED},
	{"AT", sqlx.AT},
}

type goldenNamed struct {
	name  string
	query string
	arg   interface{}
}

type goldenIn struct {
	name  string
	query string
	args  []interface{}
}

type goldenRebind struct {
	name  string
	query string
}

var person = goldenPerson{FirstName: "Jason", LastName: "Moiron", Email: "jmoiron@jmoiron.net"}
var people = []goldenPerson{person, {FirstName: "John", LastName: "Doe", Email: "johndoeDNE@gmail.net"}}
var personMap = map[string]interface{}{"first_name": "Jane", "last_name": "Citizen", "email": "jane.citzen@example.com"}
var personMaps = []map[string]interface{}{personMap, {"first_name": "Bin", "last_name": "Smuth", "email": "bensmith@allblacks.nz"}}
var idMap = map[string]interface{}{"id": 42}
var abMap = map[string]interface{}{"a": 1, "b": 2}
var abMaps = []map[string]interface{}{abMap, {"a": 3, "b": 4}}

var goldenNamedCases = []goldenNamed{
	{"insert struct", `INSERT INTO person (first_name, last_name, email) VALUES (:first_name, :last_name, :email)`, person},
	{"select map", `SELECT * FROM person WHERE first_name=:first_name AND last_name=:last_name`, personMap},
	{"select struct trailing param", `SELECT * FROM person WHERE email=:email`, person},
	{"params without spaces", `INSERT INTO t (a,b) VALUES(:a,:b)`, abMap},
	{"batch insert structs", `INSERT INTO person (first_name, last_name, email) VALUES (:first_name, :last_name, :email)`, people},
	{"batch insert maps", `INSERT INTO person (first_name, last_name, email) VALUES (:first_name, :last_name, :email)`, personMaps},
	{"batch insert without column list", `insert into t values (:a, :b)`, abMaps},
	{"batch insert multiline", "INSERT INTO t (a, b)\nVALUES\n  (:a, :b)", abMaps},
	{"batch values in from", `UPDATE t SET b = u.b FROM (VALUES (:a, :b)) AS u(a, b) WHERE t.a = u.a`, abMaps},
	{"batch on duplicate key", `INSERT INTO t (a, b) VALUES (:a, :b) ON DUPLICATE KEY UPDATE b = VALUES(b)`, abMaps},
	{"batch table named values", `INSERT INTO "values" (a, b) VALUES (:a, :b)`, abMaps},
	{"nested struct dotted names", `INSERT INTO booking (first, country, city, nights) VALUES (:person.first_name, :place.country, :place.city, :nights)`, goldenBooking{Person: person, Place: goldenPlace{Country: "NZ", City: "Wellington"}, Nights: 3}},
	{"cast attached to param", `SELECT * FROM t WHERE id = :id::uuid`, idMap},
	{"cast attached to param text", `SELECT :id::text AS id`, idMap},
	{"cast elsewhere", `SELECT (now() AT TIME ZONE 'utc')::date FROM t WHERE id = :id`, idMap},
	{"double cast", `SELECT :id::text::int FROM t`, idMap},
	{"old four-colon escape", `SELECT :id::::uuid`, idMap},
	{"colon in string literal time", `SELECT * FROM t WHERE at > '12:30' AND id = :id`, idMap},
	{"colon in string literal words", `SELECT 'note: see :id' AS note, id FROM t WHERE id = :id`, idMap},
	{"colon in line comment", "SELECT * FROM t -- filter: by id\nWHERE id = :id", idMap},
	{"colon in block comment", `SELECT /* owner: ops */ * FROM t WHERE id = :id`, idMap},
	{"nested block comment", `SELECT /* a /* b: */ c */ * FROM t WHERE id = :id`, idMap},
	{"mysql assignment", `SELECT @x := :id, @x + 1`, idMap},
	{"array slice", `SELECT arr[1:2] FROM t WHERE id = :id`, idMap},
	{"unicode name", `INSERT INTO t (name) VALUES (:名前)`, map[string]interface{}{"名前": "x"}},
	{"dollar quoted string", `SELECT $$it's :not a param$$ AS s, id FROM t WHERE id = :id`, idMap},
	{"dollar quoted with tag", `SELECT $q$:a::b$q$ AS s FROM t WHERE id = :id`, idMap},
	{"double quoted identifier", `SELECT "col:on" FROM t WHERE id = :id`, idMap},
	{"backtick identifier", "SELECT `col:on` FROM t WHERE id = :id", idMap},
	{"postgres escape string", `SELECT E'it\'s' AS s FROM t WHERE id = :id`, idMap},
	{"doubled quote in literal", `SELECT 'it''s :fine' AS s FROM t WHERE id = :id`, idMap},
	{"json path with colon", `SELECT data->>'k:v' FROM t WHERE id = :id`, idMap},
	{"question mark literal", `SELECT '?' AS q FROM t WHERE id = :id`, idMap},
	{"jsonb operator", `SELECT * FROM t WHERE data ? 'k' AND id = :id`, idMap},
	{"two params adjacent", `SELECT :a:b`, abMap},
	{"name starting with digit", `SELECT :1 AS one, :a`, abMap},
	{"param then colon then space", `SELECT :a, b: c`, abMap},
	{"missing argument", `SELECT :a, :missing`, abMap},
	{"no params at all", `SELECT 1`, abMap},
	{"empty map", `SELECT 1`, map[string]interface{}{}},
}

var goldenInCases = []goldenIn{
	{"single slice", `SELECT * FROM t WHERE id IN (?)`, []interface{}{[]int{1, 2, 3}}},
	{"one element", `SELECT * FROM t WHERE id IN (?)`, []interface{}{[]int{1}}},
	{"two slices", `SELECT * FROM t WHERE a IN (?) AND b IN (?)`, []interface{}{[]int{1, 2}, []string{"x", "y"}}},
	{"scalar and slice", `SELECT * FROM t WHERE x = ? AND id IN (?) AND y = ?`, []interface{}{"a", []int{1, 2}, "b"}},
	{"interface slice", `SELECT * FROM t WHERE id IN (?)`, []interface{}{[]interface{}{1, "two", 3.0}}},
	{"bytes are a value", `SELECT * FROM t WHERE blob = ?`, []interface{}{[]byte("x")}},
	{"valuer scalar", `SELECT * FROM t WHERE x = ? AND id IN (?)`, []interface{}{goldenValuer{"v"}, []int{1}}},
	{"valuer slice", `SELECT * FROM t WHERE id IN (?)`, []interface{}{goldenSliceValuer{}}},
	{"empty slice", `SELECT * FROM t WHERE id IN (?)`, []interface{}{[]int{}}},
	{"nil slice", `SELECT * FROM t WHERE id IN (?)`, []interface{}{[]int(nil)}},
	{"empty slice not in", `SELECT * FROM t WHERE id NOT IN (?)`, []interface{}{[]int{}}},
	{"empty and full", `SELECT * FROM t WHERE a IN (?) AND b IN (?)`, []interface{}{[]int{}, []int{1, 2}}},
	{"question in line comment", "SELECT * -- which ids?\nFROM t WHERE id IN (?)", []interface{}{[]int{1, 2}}},
	{"question in block comment", `SELECT /* why? */ * FROM t WHERE id IN (?)`, []interface{}{[]int{1, 2}}},
	{"question in literal", `SELECT '?' AS q FROM t WHERE id IN (?)`, []interface{}{[]int{1, 2}}},
	{"question in identifier", `SELECT "what?" FROM t WHERE id IN (?)`, []interface{}{[]int{1, 2}}},
	{"double question escape", `SELECT * FROM t WHERE data ?? 'k' AND id IN (?)`, []interface{}{[]int{1, 2}}},
	{"mysql escaped quote", `SELECT * FROM t WHERE note = 'don\'t' AND id IN (?)`, []interface{}{[]int{1, 2}}},
	{"dollar quoted question", `SELECT $$?$$ AS s FROM t WHERE id IN (?)`, []interface{}{[]int{1, 2}}},
	{"too few args", `SELECT * FROM t WHERE a IN (?) AND b = ?`, []interface{}{[]int{1}}},
	{"too many args", `SELECT * FROM t WHERE a IN (?)`, []interface{}{[]int{1}, 2}},
	{"no slices passthrough", `SELECT * FROM t WHERE a = ? AND b = ?`, []interface{}{1, 2}},
	{"colon param left alone", `SELECT * FROM t WHERE a = :a AND id IN (?)`, []interface{}{[]int{1, 2}}},
}

var goldenRebindCases = []goldenRebind{
	{"three placeholders", `INSERT INTO t (a, b, c) VALUES (?, ?, ?)`},
	{"no placeholders", `SELECT 1`},
	{"question in literal", `SELECT '?' AS q FROM t WHERE a = ?`},
	{"question in line comment", "SELECT * -- a?\nFROM t WHERE a = ?"},
	{"question in block comment", `SELECT /* a? */ * FROM t WHERE a = ?`},
	{"double question escape", `SELECT * FROM t WHERE data ?? 'k' AND a = ?`},
	{"jsonb operators escaped", `SELECT * FROM t WHERE data ??| array['a'] AND data ??& array['b'] AND a = ?`},
	{"mysql escaped quote", `SELECT * FROM t WHERE note = 'don\'t' AND a = ?`},
	{"like escape clause", `SELECT * FROM t WHERE a LIKE ? ESCAPE '\' AND b = ?`},
	{"bracket identifier", `SELECT [what?] FROM t WHERE a = ?`},
	{"dollar quoted", `SELECT $$?$$ FROM t WHERE a = ?`},
	{"backtick identifier", "SELECT `a?` FROM t WHERE a = ?"},
	{"postgres escape string", `SELECT E'\'?' FROM t WHERE a = ?`},
	{"already dollar", `SELECT * FROM t WHERE a = $1 AND b = ?`},
}

func goldenRender() string {
	var b strings.Builder
	line := func(format string, args ...interface{}) {
		fmt.Fprintf(&b, format, args...)
		b.WriteByte('\n')
	}
	for _, c := range goldenNamedCases {
		for _, bt := range goldenBindTypes {
			q, args, err := sqlx.BindNamed(bt.bt, c.query, c.arg)
			line("named/%s/%s: %q args=%s err=%v", c.name, bt.name, q, goldenArgs(args), err)
		}
	}
	for _, c := range goldenInCases {
		q, args, err := sqlx.In(c.query, c.args...)
		line("in/%s: %q args=%s err=%v", c.name, q, goldenArgs(args), err)
	}
	for _, c := range goldenRebindCases {
		for _, bt := range goldenBindTypes {
			line("rebind/%s/%s: %q", c.name, bt.name, sqlx.Rebind(bt.bt, c.query))
		}
	}
	return b.String()
}

func goldenArgs(args []interface{}) string {
	if args == nil {
		return "nil"
	}
	parts := make([]string, len(args))
	for i, a := range args {
		switch v := a.(type) {
		case map[string]interface{}:
			keys := make([]string, 0, len(v))
			for k := range v {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			parts[i] = fmt.Sprintf("map%v", keys)
		default:
			parts[i] = fmt.Sprintf("%#v", a)
		}
	}
	return "[" + strings.Join(parts, " ") + "]"
}
