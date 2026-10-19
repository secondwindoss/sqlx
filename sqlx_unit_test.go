// Unit tests that need no database. The tests that run against Postgres,
// MySQL and SQLite live in the dbtest module, so the drivers stay out of
// this module's go.mod.
package sqlx

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/secondwindoss/sqlx/reflectx"
)

/* compile time checks that Db, Tx, Stmt (qStmt) implement expected interfaces */
var _, _ Ext = &DB{}, &Tx{}
var _, _ ColScanner = &Row{}, &Rows{}
var _ Queryer = &qStmt{}
var _ Execer = &qStmt{}

// tests that sqlx will not panic when the wrong driver is passed because
// of an automatic nil dereference in sqlx.Open(), which was fixed.
func TestDoNotPanicOnConnect(t *testing.T) {
	db, err := Connect("bogus", "hehe")
	if err == nil {
		t.Errorf("Should return error when using bogus driverName")
	}
	if db != nil {
		t.Errorf("Should not return the db on a connect failure")
	}
}

func TestRebind(t *testing.T) {
	q1 := `INSERT INTO foo (a, b, c, d, e, f, g, h, i) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	q2 := `INSERT INTO foo (a, b, c) VALUES (?, ?, "foo"), ("Hi", ?, ?)`

	s1 := Rebind(DOLLAR, q1)
	s2 := Rebind(DOLLAR, q2)

	if s1 != `INSERT INTO foo (a, b, c, d, e, f, g, h, i) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)` {
		t.Errorf("q1 failed")
	}

	if s2 != `INSERT INTO foo (a, b, c) VALUES ($1, $2, "foo"), ("Hi", $3, $4)` {
		t.Errorf("q2 failed")
	}

	s1 = Rebind(AT, q1)
	s2 = Rebind(AT, q2)

	if s1 != `INSERT INTO foo (a, b, c, d, e, f, g, h, i) VALUES (@p1, @p2, @p3, @p4, @p5, @p6, @p7, @p8, @p9, @p10)` {
		t.Errorf("q1 failed")
	}

	if s2 != `INSERT INTO foo (a, b, c) VALUES (@p1, @p2, "foo"), ("Hi", @p3, @p4)` {
		t.Errorf("q2 failed")
	}

	s1 = Rebind(NAMED, q1)
	s2 = Rebind(NAMED, q2)

	ex1 := `INSERT INTO foo (a, b, c, d, e, f, g, h, i) VALUES ` +
		`(:arg1, :arg2, :arg3, :arg4, :arg5, :arg6, :arg7, :arg8, :arg9, :arg10)`
	if s1 != ex1 {
		t.Error("q1 failed on Named params")
	}

	ex2 := `INSERT INTO foo (a, b, c) VALUES (:arg1, :arg2, "foo"), ("Hi", :arg3, :arg4)`
	if s2 != ex2 {
		t.Error("q2 failed on Named params")
	}
}

func TestBindMap(t *testing.T) {
	// Test that it works..
	q1 := `INSERT INTO foo (a, b, c, d) VALUES (:name, :age, :first, :last)`
	am := map[string]interface{}{
		"name":  "Jason Moiron",
		"age":   30,
		"first": "Jason",
		"last":  "Moiron",
	}

	bq, args, _ := bindMap(QUESTION, q1, am)
	expect := `INSERT INTO foo (a, b, c, d) VALUES (?, ?, ?, ?)`
	if bq != expect {
		t.Errorf("Interpolation of query failed: got `%v`, expected `%v`\n", bq, expect)
	}

	if args[0].(string) != "Jason Moiron" {
		t.Errorf("Expected `Jason Moiron`, got %v\n", args[0])
	}

	if args[1].(int) != 30 {
		t.Errorf("Expected 30, got %v\n", args[1])
	}

	if args[2].(string) != "Jason" {
		t.Errorf("Expected Jason, got %v\n", args[2])
	}

	if args[3].(string) != "Moiron" {
		t.Errorf("Expected Moiron, got %v\n", args[3])
	}
}

func TestBindStruct(t *testing.T) {
	var err error

	q1 := `INSERT INTO foo (a, b, c, d) VALUES (:name, :age, :first, :last)`

	type tt struct {
		Name  string
		Age   int
		First string
		Last  string
	}

	type tt2 struct {
		Field1 string `db:"field_1"`
		Field2 string `db:"field_2"`
	}

	type tt3 struct {
		tt2
		Name string
	}

	am := tt{"Jason Moiron", 30, "Jason", "Moiron"}

	bq, args, _ := bindStruct(QUESTION, q1, am, mapper())
	expect := `INSERT INTO foo (a, b, c, d) VALUES (?, ?, ?, ?)`
	if bq != expect {
		t.Errorf("Interpolation of query failed: got `%v`, expected `%v`\n", bq, expect)
	}

	if args[0].(string) != "Jason Moiron" {
		t.Errorf("Expected `Jason Moiron`, got %v\n", args[0])
	}

	if args[1].(int) != 30 {
		t.Errorf("Expected 30, got %v\n", args[1])
	}

	if args[2].(string) != "Jason" {
		t.Errorf("Expected Jason, got %v\n", args[2])
	}

	if args[3].(string) != "Moiron" {
		t.Errorf("Expected Moiron, got %v\n", args[3])
	}

	am2 := tt2{"Hello", "World"}
	bq, args, _ = bindStruct(QUESTION, "INSERT INTO foo (a, b) VALUES (:field_2, :field_1)", am2, mapper())
	expect = `INSERT INTO foo (a, b) VALUES (?, ?)`
	if bq != expect {
		t.Errorf("Interpolation of query failed: got `%v`, expected `%v`\n", bq, expect)
	}

	if args[0].(string) != "World" {
		t.Errorf("Expected 'World', got %s\n", args[0].(string))
	}
	if args[1].(string) != "Hello" {
		t.Errorf("Expected 'Hello', got %s\n", args[1].(string))
	}

	am3 := tt3{Name: "Hello!"}
	am3.Field1 = "Hello"
	am3.Field2 = "World"

	bq, args, err = bindStruct(QUESTION, "INSERT INTO foo (a, b, c) VALUES (:name, :field_1, :field_2)", am3, mapper())

	if err != nil {
		t.Fatal(err)
	}

	expect = `INSERT INTO foo (a, b, c) VALUES (?, ?, ?)`
	if bq != expect {
		t.Errorf("Interpolation of query failed: got `%v`, expected `%v`\n", bq, expect)
	}

	if args[0].(string) != "Hello!" {
		t.Errorf("Expected 'Hello!', got %s\n", args[0].(string))
	}
	if args[1].(string) != "Hello" {
		t.Errorf("Expected 'Hello', got %s\n", args[1].(string))
	}
	if args[2].(string) != "World" {
		t.Errorf("Expected 'World', got %s\n", args[0].(string))
	}
}

func BenchmarkBindStruct(b *testing.B) {
	b.StopTimer()
	q1 := `INSERT INTO foo (a, b, c, d) VALUES (:name, :age, :first, :last)`
	type t struct {
		Name  string
		Age   int
		First string
		Last  string
	}
	am := t{"Jason Moiron", 30, "Jason", "Moiron"}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		bindStruct(DOLLAR, q1, am, mapper())
	}
}

func TestBindNamedMapper(t *testing.T) {
	type A map[string]interface{}
	m := reflectx.NewMapperFunc("db", NameMapper)
	query, args, err := bindNamedMapper(DOLLAR, `select :x`, A{
		"x": "X!",
	}, m)
	if err != nil {
		t.Fatal(err)
	}

	got := fmt.Sprintf("%s %s", query, args)
	want := `select $1 [X!]`
	if got != want {
		t.Errorf("\ngot:  %q\nwant: %q", got, want)
	}

	_, _, err = bindNamedMapper(DOLLAR, `select :x`, map[string]string{
		"x": "X!",
	}, m)
	if err == nil {
		t.Fatal("err is nil")
	}
	if !strings.Contains(err.Error(), "unsupported map type") {
		t.Errorf("wrong error: %s", err)
	}
}

func BenchmarkBindMap(b *testing.B) {
	b.StopTimer()
	q1 := `INSERT INTO foo (a, b, c, d) VALUES (:name, :age, :first, :last)`
	am := map[string]interface{}{
		"name":  "Jason Moiron",
		"age":   30,
		"first": "Jason",
		"last":  "Moiron",
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		bindMap(DOLLAR, q1, am)
	}
}

func BenchmarkIn(b *testing.B) {
	q := `SELECT * FROM foo WHERE x = ? AND v in (?) AND y = ?`

	for i := 0; i < b.N; i++ {
		_, _, _ = In(q, []interface{}{"foo", []int{0, 5, 7, 2, 9}, "bar"}...)
	}
}

func BenchmarkIn1k(b *testing.B) {
	q := `SELECT * FROM foo WHERE x = ? AND v in (?) AND y = ?`

	var vals [1000]interface{}

	for i := 0; i < b.N; i++ {
		_, _, _ = In(q, []interface{}{"foo", vals[:], "bar"}...)
	}
}

func BenchmarkIn1kInt(b *testing.B) {
	q := `SELECT * FROM foo WHERE x = ? AND v in (?) AND y = ?`

	var vals [1000]int

	for i := 0; i < b.N; i++ {
		_, _, _ = In(q, []interface{}{"foo", vals[:], "bar"}...)
	}
}

func BenchmarkIn1kString(b *testing.B) {
	q := `SELECT * FROM foo WHERE x = ? AND v in (?) AND y = ?`

	var vals [1000]string

	for i := 0; i < b.N; i++ {
		_, _, _ = In(q, []interface{}{"foo", vals[:], "bar"}...)
	}
}

func BenchmarkRebind(b *testing.B) {
	b.StopTimer()
	q1 := `INSERT INTO foo (a, b, c, d, e, f, g, h, i) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	q2 := `INSERT INTO foo (a, b, c) VALUES (?, ?, "foo"), ("Hi", ?, ?)`
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		Rebind(DOLLAR, q1)
		Rebind(DOLLAR, q2)
	}
}

func BenchmarkRebindBuffer(b *testing.B) {
	b.StopTimer()
	q1 := `INSERT INTO foo (a, b, c, d, e, f, g, h, i) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	q2 := `INSERT INTO foo (a, b, c) VALUES (?, ?, "foo"), ("Hi", ?, ?)`
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		rebindBuff(DOLLAR, q1)
		rebindBuff(DOLLAR, q2)
	}
}

func TestIn130Regression(t *testing.T) {
	t.Run("[]interface{}{}", func(t *testing.T) {
		q, args, err := In("SELECT * FROM people WHERE name IN (?)", []interface{}{[]string{"gopher"}}...)
		if err != nil {
			t.Fatal(err)
		}
		if q != "SELECT * FROM people WHERE name IN (?)" {
			t.Errorf("got=%v", q)
		}
		t.Log(args)
		for _, a := range args {
			switch a := a.(type) {
			case string:
				t.Log("ok: string", a)
			case *string:
				t.Error("ng: string pointer", a, *a)
			}
		}
	})

	t.Run("[]string{}", func(t *testing.T) {
		q, args, err := In("SELECT * FROM people WHERE name IN (?)", []string{"gopher"})
		if err != nil {
			t.Fatal(err)
		}
		if q != "SELECT * FROM people WHERE name IN (?)" {
			t.Errorf("got=%v", q)
		}
		t.Log(args)
		for _, a := range args {
			switch a := a.(type) {
			case string:
				t.Log("ok: string", a)
			case *string:
				t.Error("ng: string pointer", a, *a)
			}
		}
	})
}

func TestDoNotPanicOnConnectContext(t *testing.T) {
	_, err := ConnectContext(context.Background(), "bogus", "hehe")
	if err == nil {
		t.Errorf("Should return error when using bogus driverName")
	}
}
