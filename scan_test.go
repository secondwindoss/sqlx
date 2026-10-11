package sqlx

import (
	"strings"
	"testing"
)

type inCase struct {
	name    string
	q       string
	args    []interface{}
	want    string // expected query from the new In; "" when wantErr is set
	wantN   int    // expected len(args)
	wantErr string // substring of the expected error, "" for no error
}

type rebindCase struct {
	name  string
	q     string
	types []int
	want  map[int]string
}

type namedCase struct {
	name    string
	q       string
	types   []int
	want    map[int]string
	names   []string
	wantErr string
}

var inInts = []int{1, 2, 3}
var inTwo = []int{1, 2}

var inCases = []inCase{
	{"plain slice expansion", `SELECT * FROM t WHERE id IN (?)`, []interface{}{inInts}, `SELECT * FROM t WHERE id IN (?, ?, ?)`, 3, ""},
	{"mixed scalar and slice", `SELECT * FROM t WHERE x = ? AND id IN (?) AND y = ?`, []interface{}{"a", inInts, "b"}, `SELECT * FROM t WHERE x = ? AND id IN (?, ?, ?) AND y = ?`, 5, ""},
	{"? in line comment (#961)", "SELECT * -- should we filter by id?\nFROM t WHERE id IN (?)", []interface{}{inInts}, "SELECT * -- should we filter by id?\nFROM t WHERE id IN (?, ?, ?)", 3, ""},
	{"? in block comment", `SELECT /* id=? */ * FROM t WHERE id IN (?)`, []interface{}{inTwo}, `SELECT /* id=? */ * FROM t WHERE id IN (?, ?)`, 2, ""},
	{"? in nested block comment", `SELECT /* a /* b? */ c? */ * FROM t WHERE id IN (?)`, []interface{}{inTwo}, `SELECT /* a /* b? */ c? */ * FROM t WHERE id IN (?, ?)`, 2, ""},
	{"? in single-quoted string (#608)", `INSERT INTO t (a, b, c) VALUES ('?? ?', ?, ?)`, []interface{}{"int", inTwo}, `INSERT INTO t (a, b, c) VALUES ('?? ?', ?, ?, ?)`, 3, ""},
	{"? after doubled quote", `SELECT 'it''s?' FROM t WHERE id IN (?)`, []interface{}{inTwo}, `SELECT 'it''s?' FROM t WHERE id IN (?, ?)`, 2, ""},
	{"? in double-quoted identifier", `SELECT "a?b" FROM t WHERE id IN (?)`, []interface{}{inTwo}, `SELECT "a?b" FROM t WHERE id IN (?, ?)`, 2, ""},
	{"? in backtick identifier", "SELECT `a?b` FROM t WHERE id IN (?)", []interface{}{inTwo}, "SELECT `a?b` FROM t WHERE id IN (?, ?)", 2, ""},
	{"? in dollar-quoted string", `SELECT $$a?b$$ FROM t WHERE id IN (?)`, []interface{}{inTwo}, `SELECT $$a?b$$ FROM t WHERE id IN (?, ?)`, 2, ""},
	{"? in tagged dollar quote", `SELECT $fn$ it's ? $fn$ FROM t WHERE id IN (?)`, []interface{}{inTwo}, `SELECT $fn$ it's ? $fn$ FROM t WHERE id IN (?, ?)`, 2, ""},
	{"regex with (?:) in literal (#487)", `SELECT "IPs" ~ ('(?:^|,)' || ? || '(?:,|$)') FROM r WHERE "RuleID" IN (?)`, []interface{}{"10.0.0.1", inTwo}, `SELECT "IPs" ~ ('(?:^|,)' || ? || '(?:,|$)') FROM r WHERE "RuleID" IN (?, ?)`, 3, ""},
	{"?? escape is not a placeholder (#628)", `SELECT * FROM t WHERE data ?? 'k' AND id IN (?)`, []interface{}{inTwo}, `SELECT * FROM t WHERE data ?? 'k' AND id IN (?, ?)`, 2, ""},
	{"$1 placeholders are left alone", `SELECT * FROM t WHERE a = $1 AND id IN (?)`, []interface{}{inTwo}, `SELECT * FROM t WHERE a = $1 AND id IN (?, ?)`, 2, ""},
	{"E'' string with backslash-quote", `SELECT E'it\'s?' FROM t WHERE id IN (?)`, []interface{}{inTwo}, `SELECT E'it\'s?' FROM t WHERE id IN (?, ?)`, 2, ""},
	{"MySQL backslash-escaped quote", `SELECT 'don\'t?' FROM t WHERE id IN (?)`, []interface{}{inTwo}, `SELECT 'don\'t?' FROM t WHERE id IN (?, ?)`, 2, ""},
	{"LIKE ... ESCAPE '\\' (standard string ending in backslash)", `SELECT * FROM t WHERE name LIKE ? ESCAPE '\' AND id IN (?)`, []interface{}{"a%", inTwo}, `SELECT * FROM t WHERE name LIKE ? ESCAPE '\' AND id IN (?, ?)`, 3, ""},
	{"ARRAY[...] is not an identifier", `SELECT ARRAY[?, ?] FROM t WHERE id IN (?)`, []interface{}{1, 2, inTwo}, `SELECT ARRAY[?, ?] FROM t WHERE id IN (?, ?)`, 4, ""},
	{"trailing line comment without newline", `SELECT * FROM t WHERE id IN (?) -- why?`, []interface{}{inTwo}, `SELECT * FROM t WHERE id IN (?, ?) -- why?`, 2, ""},
	{"unterminated quote degrades to plain text", `SELECT * FROM t WHERE note = 'x AND id IN (?)`, []interface{}{inTwo}, `SELECT * FROM t WHERE note = 'x AND id IN (?, ?)`, 2, ""},
	{"no slices: query returned unparsed", `SELECT '?' FROM t WHERE x = ?`, []interface{}{"a"}, `SELECT '?' FROM t WHERE x = ?`, 1, ""},
	{"more args than bindvars", `SELECT * FROM t WHERE x = ? and y = ?`, []interface{}{"foo", inInts, "bar"}, "", 0, "less than"},
	{"more bindvars than args", `SELECT * FROM t WHERE x = ? AND y in (?)`, []interface{}{inInts}, "", 0, "exceeds"},
	{"empty slice", `SELECT * FROM t WHERE id IN (?)`, []interface{}{[]int{}}, `SELECT * FROM t WHERE id IN (NULL)`, 0, ""},
	{"# is not a comment (documented limitation)", "SELECT * # really?\nFROM t WHERE id IN (?)", []interface{}{inTwo}, "", 0, "exceeds"},
	{"[bracket] identifier not honoured by In (documented limitation)", `SELECT [a?b] FROM t WHERE id IN (?)`, []interface{}{inTwo}, "", 0, "exceeds"},
}

var allBindTypes = []int{QUESTION, DOLLAR, NAMED, AT}

var rebindCases = []rebindCase{
	{"TestRebind q1", `INSERT INTO foo (a, b, c, d, e, f, g, h, i) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, allBindTypes, map[int]string{
		QUESTION: `INSERT INTO foo (a, b, c, d, e, f, g, h, i) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		DOLLAR:   `INSERT INTO foo (a, b, c, d, e, f, g, h, i) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		NAMED:    `INSERT INTO foo (a, b, c, d, e, f, g, h, i) VALUES (:arg1, :arg2, :arg3, :arg4, :arg5, :arg6, :arg7, :arg8, :arg9, :arg10)`,
		AT:       `INSERT INTO foo (a, b, c, d, e, f, g, h, i) VALUES (@p1, @p2, @p3, @p4, @p5, @p6, @p7, @p8, @p9, @p10)`}},
	{"TestRebind q2", `INSERT INTO foo (a, b, c) VALUES (?, ?, "foo"), ("Hi", ?, ?)`, allBindTypes, map[int]string{
		QUESTION: `INSERT INTO foo (a, b, c) VALUES (?, ?, "foo"), ("Hi", ?, ?)`,
		DOLLAR:   `INSERT INTO foo (a, b, c) VALUES ($1, $2, "foo"), ("Hi", $3, $4)`,
		NAMED:    `INSERT INTO foo (a, b, c) VALUES (:arg1, :arg2, "foo"), ("Hi", :arg3, :arg4)`,
		AT:       `INSERT INTO foo (a, b, c) VALUES (@p1, @p2, "foo"), ("Hi", @p3, @p4)`}},
	{"? in single-quoted string", `SELECT '?' AS q, ? FROM t`, allBindTypes, map[int]string{
		QUESTION: `SELECT '?' AS q, ? FROM t`, DOLLAR: `SELECT '?' AS q, $1 FROM t`, NAMED: `SELECT '?' AS q, :arg1 FROM t`, AT: `SELECT '?' AS q, @p1 FROM t`}},
	{"? after doubled quote", `SELECT 'it''s?', ? FROM t`, []int{DOLLAR}, map[int]string{DOLLAR: `SELECT 'it''s?', $1 FROM t`}},
	{"? in double-quoted identifier", `SELECT "what?" FROM t WHERE id = ?`, []int{DOLLAR, AT}, map[int]string{DOLLAR: `SELECT "what?" FROM t WHERE id = $1`, AT: `SELECT "what?" FROM t WHERE id = @p1`}},
	{"? in backtick identifier", "SELECT `what?` FROM t WHERE id = ?", []int{DOLLAR}, map[int]string{DOLLAR: "SELECT `what?` FROM t WHERE id = $1"}},
	{"? in [bracket] identifier (AT only)", `SELECT [what?] FROM t WHERE id = ?`, []int{AT, DOLLAR}, map[int]string{AT: `SELECT [what?] FROM t WHERE id = @p1`, DOLLAR: `SELECT [what$1] FROM t WHERE id = $2`}},
	{"ARRAY[?] subscript (DOLLAR)", `SELECT ARRAY[?, ?], arr[?] FROM t`, []int{DOLLAR}, map[int]string{DOLLAR: `SELECT ARRAY[$1, $2], arr[$3] FROM t`}},
	{"? in line comment", "SELECT ? -- why?\nFROM t", []int{DOLLAR}, map[int]string{DOLLAR: "SELECT $1 -- why?\nFROM t"}},
	{"? in block comment", `SELECT /* ? */ ? FROM t`, []int{DOLLAR}, map[int]string{DOLLAR: `SELECT /* ? */ $1 FROM t`}},
	{"? in nested block comment", `SELECT /* a /* ? */ ? */ ? FROM t`, []int{DOLLAR}, map[int]string{DOLLAR: `SELECT /* a /* ? */ ? */ $1 FROM t`}},
	{"? in dollar quote", `SELECT $$?$$, ? FROM t`, []int{DOLLAR}, map[int]string{DOLLAR: `SELECT $$?$$, $1 FROM t`}},
	{"? in tagged dollar quote", `SELECT $q$it's ?$q$, ? FROM t`, []int{DOLLAR}, map[int]string{DOLLAR: `SELECT $q$it's ?$q$, $1 FROM t`}},
	{"$tag without closer is plain text", `SELECT $a$ , ? FROM t`, []int{DOLLAR}, map[int]string{DOLLAR: `SELECT $a$ , $1 FROM t`}},
	{"identifier containing $ is not a dollar quote", `SELECT a$b$c, ? FROM t WHERE x = ?`, []int{DOLLAR}, map[int]string{DOLLAR: `SELECT a$b$c, $1 FROM t WHERE x = $2`}},
	{"?? escape -> literal ? (jsonb ? operator)", `SELECT * FROM t WHERE data ?? 'k' AND id = ?`, allBindTypes, map[int]string{
		QUESTION: `SELECT * FROM t WHERE data ? 'k' AND id = ?`, DOLLAR: `SELECT * FROM t WHERE data ? 'k' AND id = $1`, NAMED: `SELECT * FROM t WHERE data ? 'k' AND id = :arg1`, AT: `SELECT * FROM t WHERE data ? 'k' AND id = @p1`}},
	{"??| and ??& operators", `SELECT * FROM t WHERE data ??| array['a'] AND data ??& array['b'] AND id = ?`, []int{DOLLAR}, map[int]string{DOLLAR: `SELECT * FROM t WHERE data ?| array['a'] AND data ?& array['b'] AND id = $1`}},
	{"QUESTION without ?? is returned unchanged", `SELECT '?' FROM t WHERE id = ?`, []int{QUESTION, UNKNOWN}, map[int]string{QUESTION: `SELECT '?' FROM t WHERE id = ?`, UNKNOWN: `SELECT '?' FROM t WHERE id = ?`}},
	{"E'' string with backslash-quote", `SELECT E'it\'s?', ? FROM t`, []int{DOLLAR}, map[int]string{DOLLAR: `SELECT E'it\'s?', $1 FROM t`}},
	{"E'' is only a prefix when not part of an identifier", `SELECT SOME'x' , ? FROM t`, []int{DOLLAR}, map[int]string{DOLLAR: `SELECT SOME'x' , $1 FROM t`}},
	{"LIKE ... ESCAPE '\\'", `SELECT * FROM t WHERE name LIKE ? ESCAPE '\' AND id = ?`, []int{DOLLAR}, map[int]string{DOLLAR: `SELECT * FROM t WHERE name LIKE $1 ESCAPE '\' AND id = $2`}},
	{"MySQL backslash-escaped quote (heuristic)", `SELECT 'don\'t?', ? FROM t`, []int{DOLLAR}, map[int]string{DOLLAR: `SELECT 'don\'t?', $1 FROM t`}},
	{"Windows path literal", `SELECT * FROM f WHERE dir = 'C:\' AND id = ?`, []int{DOLLAR}, map[int]string{DOLLAR: `SELECT * FROM f WHERE dir = 'C:\' AND id = $1`}},
	{"unterminated quote degrades to plain text", `SELECT 'x FROM t WHERE id = ?`, []int{DOLLAR}, map[int]string{DOLLAR: `SELECT 'x FROM t WHERE id = $1`}},
	{"# is not a comment (documented limitation)", "SELECT * # what?\nFROM t WHERE id = ?", []int{DOLLAR}, map[int]string{DOLLAR: "SELECT * # what$1\nFROM t WHERE id = $2"}},
	{"jsonb #> operator is untouched", `SELECT data #> '{a}' FROM t WHERE id = ?`, []int{DOLLAR}, map[int]string{DOLLAR: `SELECT data #> '{a}' FROM t WHERE id = $1`}},
	{":name text is untouched by Rebind", `SELECT :name, ? FROM t`, []int{DOLLAR}, map[int]string{DOLLAR: `SELECT :name, $1 FROM t`}},
}

var namedCases = []namedCase{
	{"TestCompileQuery row 1", `INSERT INTO foo (a,b,c,d) VALUES (:name, :age, :first, :last)`, allBindTypes, map[int]string{
		QUESTION: `INSERT INTO foo (a,b,c,d) VALUES (?, ?, ?, ?)`, DOLLAR: `INSERT INTO foo (a,b,c,d) VALUES ($1, $2, $3, $4)`,
		AT: `INSERT INTO foo (a,b,c,d) VALUES (@p1, @p2, @p3, @p4)`, NAMED: `INSERT INTO foo (a,b,c,d) VALUES (:name, :age, :first, :last)`},
		[]string{"name", "age", "first", "last"}, ""},
	{"TestCompileQuery row 2 (name at end, digits)", `SELECT * FROM a WHERE first_name=:name1 AND last_name=:name2`, allBindTypes, map[int]string{
		QUESTION: `SELECT * FROM a WHERE first_name=? AND last_name=?`, DOLLAR: `SELECT * FROM a WHERE first_name=$1 AND last_name=$2`,
		AT: `SELECT * FROM a WHERE first_name=@p1 AND last_name=@p2`, NAMED: `SELECT * FROM a WHERE first_name=:name1 AND last_name=:name2`},
		[]string{"name1", "name2"}, ""},
	{"TestCompileQuery row 3 (CHANGED: \"::foo\" is verbatim)", `SELECT "::foo" FROM a WHERE first_name=:name1 AND last_name=:name2`, []int{QUESTION, DOLLAR}, map[int]string{
		QUESTION: `SELECT "::foo" FROM a WHERE first_name=? AND last_name=?`, DOLLAR: `SELECT "::foo" FROM a WHERE first_name=$1 AND last_name=$2`},
		[]string{"name1", "name2"}, ""},
	{"TestCompileQuery row 4 (CHANGED: string contents verbatim)", `SELECT 'a::b::c' || first_name, '::::ABC::_::' FROM person WHERE first_name=:first_name AND last_name=:last_name`, []int{QUESTION, DOLLAR}, map[int]string{
		QUESTION: `SELECT 'a::b::c' || first_name, '::::ABC::_::' FROM person WHERE first_name=? AND last_name=?`,
		DOLLAR:   `SELECT 'a::b::c' || first_name, '::::ABC::_::' FROM person WHERE first_name=$1 AND last_name=$2`},
		[]string{"first_name", "last_name"}, ""},
	{"TestCompileQuery row 5 (:= assignment)", `SELECT @name := "name", :age, :first, :last`, allBindTypes, map[int]string{
		QUESTION: `SELECT @name := "name", ?, ?, ?`, DOLLAR: `SELECT @name := "name", $1, $2, $3`,
		NAMED: `SELECT @name := "name", :age, :first, :last`, AT: `SELECT @name := "name", @p1, @p2, @p3`},
		[]string{"age", "first", "last"}, ""},
	{"unicode names (the commented-out TestCompileQuery row)", `INSERT INTO foo (a,b,c,d) VALUES (:あ, :b, :キコ, :名前)`, []int{QUESTION, DOLLAR, NAMED}, map[int]string{
		QUESTION: `INSERT INTO foo (a,b,c,d) VALUES (?, ?, ?, ?)`, DOLLAR: `INSERT INTO foo (a,b,c,d) VALUES ($1, $2, $3, $4)`, NAMED: `INSERT INTO foo (a,b,c,d) VALUES (:あ, :b, :キコ, :名前)`},
		[]string{"あ", "b", "キコ", "名前"}, ""},
	{"nested struct name :place.id", `SELECT * FROM pp WHERE place.id=:place.id`, []int{QUESTION}, map[int]string{QUESTION: `SELECT * FROM pp WHERE place.id=?`}, []string{"place.id"}, ""},
	{"trailing dot is not part of the name", `SELECT :a.`, []int{QUESTION}, map[int]string{QUESTION: `SELECT ?.`}, []string{"a"}, ""},
	{"cast attached to a parameter (#983, PR #1002)", `SELECT :boundary::jsonb AS b, '2026-05-01'::timestamptz AS s`, allBindTypes, map[int]string{
		QUESTION: `SELECT ?::jsonb AS b, '2026-05-01'::timestamptz AS s`, DOLLAR: `SELECT $1::jsonb AS b, '2026-05-01'::timestamptz AS s`,
		NAMED: `SELECT :boundary::jsonb AS b, '2026-05-01'::timestamptz AS s`, AT: `SELECT @p1::jsonb AS b, '2026-05-01'::timestamptz AS s`},
		[]string{"boundary"}, ""},
	{"cast elsewhere (#428 #650 #956)", `SELECT (d.categories->>0)::int AS c, now()::date FROM t WHERE id = :id`, []int{QUESTION, DOLLAR}, map[int]string{
		QUESTION: `SELECT (d.categories->>0)::int AS c, now()::date FROM t WHERE id = ?`, DOLLAR: `SELECT (d.categories->>0)::int AS c, now()::date FROM t WHERE id = $1`},
		[]string{"id"}, ""},
	{"CHANGED: :::: is no longer an escape", `SELECT :x ::::jsonb, (1, 'John')::::my_table`, []int{QUESTION}, map[int]string{QUESTION: `SELECT ? ::::jsonb, (1, 'John')::::my_table`}, []string{"x"}, ""},
	{"SQL Server static method geography::Point", `SELECT geography::Point(:lat, :lng, 4326)`, []int{AT}, map[int]string{AT: `SELECT geography::Point(@p1, @p2, 4326)`}, []string{"lat", "lng"}, ""},
	{"time literal in string (#302 #368)", `SELECT * FROM t WHERE date="1970-01-01 00:00:00" AND v=:v`, []int{QUESTION}, map[int]string{QUESTION: `SELECT * FROM t WHERE date="1970-01-01 00:00:00" AND v=?`}, []string{"v"}, ""},
	{"timezone offset in string (#253 #443)", `SELECT CONVERT_TZ(FROM_UNIXTIME(ts), @@session.time_zone, '+00:00') AS s FROM t WHERE id=:id`, []int{QUESTION}, map[int]string{QUESTION: `SELECT CONVERT_TZ(FROM_UNIXTIME(ts), @@session.time_zone, '+00:00') AS s FROM t WHERE id=?`}, []string{"id"}, ""},
	{"IPv6 literal in string (#872)", `INSERT INTO users (name, ipv6) VALUES (:name, '2001:0db8:85a3::8a2e:0370:7334')`, []int{QUESTION}, map[int]string{QUESTION: `INSERT INTO users (name, ipv6) VALUES (?, '2001:0db8:85a3::8a2e:0370:7334')`}, []string{"name"}, ""},
	{"URL in string (#947)", `SELECT id, 'http://example.com' AS link FROM t WHERE id = (:input_id)`, []int{QUESTION}, map[int]string{QUESTION: `SELECT id, 'http://example.com' AS link FROM t WHERE id = (?)`}, []string{"input_id"}, ""},
	{"colon followed by space in string (#206)", `SELECT '1: profiles' AS p, :a FROM profiles`, []int{QUESTION}, map[int]string{QUESTION: `SELECT '1: profiles' AS p, ? FROM profiles`}, []string{"a"}, ""},
	{"colon in line comment (#961 #206 #894)", "SELECT * FROM t WHERE col = :param -- Note: this breaks", []int{QUESTION}, map[int]string{QUESTION: "SELECT * FROM t WHERE col = ? -- Note: this breaks"}, []string{"param"}, ""},
	{"colon in block comment (#920)", `SELECT * FROM person WHERE id=:id /*TODO: handle :colon*/`, []int{QUESTION}, map[int]string{QUESTION: `SELECT * FROM person WHERE id=? /*TODO: handle :colon*/`}, []string{"id"}, ""},
	{"license header with URL (#894)", "-- http://www.apache.org/licenses/LICENSE-2.0\nSELECT * FROM t WHERE id = :id", []int{QUESTION}, map[int]string{QUESTION: "-- http://www.apache.org/licenses/LICENSE-2.0\nSELECT * FROM t WHERE id = ?"}, []string{"id"}, ""},
	{"# comment with colon+space (#394) works via the name-start rule", "# author(chen): some explanation\nINSERT INTO t(foo, bar) VALUE (:foo, :bar)", []int{QUESTION}, map[int]string{QUESTION: "# author(chen): some explanation\nINSERT INTO t(foo, bar) VALUE (?, ?)"}, []string{"foo", "bar"}, ""},
	{"# comment with :word is still misread (documented limitation)", "# see :ticket\nSELECT :a", []int{QUESTION}, map[int]string{QUESTION: "# see ?\nSELECT ?"}, []string{"ticket", "a"}, ""},
	{"trailing underscore at end of query (#441)", `UPDATE t SET name = :name WHERE id_ = :id_`, []int{QUESTION}, map[int]string{QUESTION: `UPDATE t SET name = ? WHERE id_ = ?`}, []string{"name", "id_"}, ""},
	{"lone colon at end of query", `SELECT :`, []int{QUESTION}, map[int]string{QUESTION: `SELECT :`}, []string{}, ""},
	{"colon before ) and ,", `SELECT f(:), g(:, 1)`, []int{QUESTION}, map[int]string{QUESTION: `SELECT f(:), g(:, 1)`}, []string{}, ""},
	{"array slice arr[1:2] is not a parameter", `SELECT arr[1:2], arr[:n] FROM t WHERE id = :id`, []int{QUESTION}, map[int]string{QUESTION: `SELECT arr[1:2], arr[?] FROM t WHERE id = ?`}, []string{"n", "id"}, ""},
	{"CHANGED: digit-start :1 is not a parameter", `SELECT :1, :a`, []int{QUESTION}, map[int]string{QUESTION: `SELECT :1, ?`}, []string{"a"}, ""},
	{"PL/pgSQL assignment", `DO $$ BEGIN x := :v; END $$; SELECT :w`, []int{QUESTION}, map[int]string{QUESTION: `DO $$ BEGIN x := :v; END $$; SELECT ?`}, []string{"w"}, ""},
	{"named param inside dollar quote is text", `SELECT $$:x$$, :a`, []int{QUESTION}, map[int]string{QUESTION: `SELECT $$:x$$, ?`}, []string{"a"}, ""},
	{"E'' string with colon", `SELECT E'a\'b:c', :a`, []int{QUESTION}, map[int]string{QUESTION: `SELECT E'a\'b:c', ?`}, []string{"a"}, ""},
	{"MySQL backslash string with colon", `SELECT 'don\'t:x', :a`, []int{QUESTION}, map[int]string{QUESTION: `SELECT 'don\'t:x', ?`}, []string{"a"}, ""},
	{"TestEscapedColons input (currently skipped)", `SELECT * FROM testtable WHERE timeposted BETWEEN (now() AT TIME ZONE 'utc') AND
	(now() AT TIME ZONE 'utc') - interval '01:30:00') AND name = '\'this is a test\'' and id = :id`, []int{DOLLAR}, map[int]string{DOLLAR: `SELECT * FROM testtable WHERE timeposted BETWEEN (now() AT TIME ZONE 'utc') AND
	(now() AT TIME ZONE 'utc') - interval '01:30:00') AND name = '\'this is a test\'' and id = $1`}, []string{"id"}, ""},
	{"? placeholders are left alone by Named", `SELECT '?' , ?, :a`, []int{QUESTION}, map[int]string{QUESTION: `SELECT '?' , ?, ?`}, []string{"a"}, ""},
	{"[bracket] identifier with colon (AT only)", `SELECT [a:b] FROM t WHERE id = :id`, []int{AT}, map[int]string{AT: `SELECT [a:b] FROM t WHERE id = @p1`}, []string{"id"}, ""},
	{":first:name still errors", `SELECT * FROM person WHERE first_name=:first:name`, []int{QUESTION}, nil, nil, "unexpected `:` while reading named param at 44"},
	{"param immediately before cast then another param", `SELECT :a::text, :b`, []int{DOLLAR}, map[int]string{DOLLAR: `SELECT $1::text, $2`}, []string{"a", "b"}, ""},
	{"same name twice yields inTwo bindvars", `SELECT :a, :a`, []int{DOLLAR}, map[int]string{DOLLAR: `SELECT $1, $2`}, []string{"a", "a"}, ""},
	{"param followed by ) at end", `INSERT INTO t (a) VALUES (:a)`, []int{QUESTION}, map[int]string{QUESTION: `INSERT INTO t (a) VALUES (?)`}, []string{"a"}, ""},
}

func bindTypeName(bt int) string {
	return map[int]string{UNKNOWN: "UNKNOWN", QUESTION: "QUESTION", DOLLAR: "DOLLAR", NAMED: "NAMED", AT: "AT"}[bt]
}
func TestInMatrix(t *testing.T) {
	for _, c := range inCases {
		t.Run(c.name, func(t *testing.T) {
			q, a, err := In(c.q, c.args...)
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("want error containing %q, got %v (query %q)", c.wantErr, err, q)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if q != c.want {
				t.Errorf("query:\n got: %q\nwant: %q", q, c.want)
			}
			if len(a) != c.wantN {
				t.Errorf("args: got %d want %d (%v)", len(a), c.wantN, a)
			}
		})
	}
}

func TestRebindMatrix(t *testing.T) {
	for _, c := range rebindCases {
		for _, bt := range c.types {
			t.Run(c.name+"/"+bindTypeName(bt), func(t *testing.T) {
				got := Rebind(bt, c.q)
				if got != c.want[bt] {
					t.Errorf("\n got: %q\nwant: %q", got, c.want[bt])
				}
			})
		}
	}
}

func TestNamedMatrix(t *testing.T) {
	for _, c := range namedCases {
		for _, bt := range c.types {
			t.Run(c.name+"/"+bindTypeName(bt), func(t *testing.T) {
				q, names, err := compileNamedQuery(c.q, bt)
				if c.wantErr != "" {
					if err == nil || !strings.Contains(err.Error(), c.wantErr) {
						t.Fatalf("want error containing %q, got %v (query %q)", c.wantErr, err, q)
					}
					return
				}
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if q != c.want[bt] {
					t.Errorf("query:\n got: %q\nwant: %q", q, c.want[bt])
				}
				if strings.Join(names, ",") != strings.Join(c.names, ",") {
					t.Errorf("names: got %q want %q", names, c.names)
				}
			})
		}
	}
}
