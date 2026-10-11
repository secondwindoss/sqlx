package sqlx

import (
	"bytes"
	"database/sql/driver"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/secondwindoss/sqlx/reflectx"
)

// Bindvar types supported by Rebind, BindMap and BindStruct.
const (
	UNKNOWN = iota
	QUESTION
	DOLLAR
	NAMED
	AT
)

var defaultBinds = map[int][]string{
	DOLLAR:   {"postgres", "pgx", "pq-timeouts", "cloudsqlpostgres", "ql", "nrpostgres", "cockroach"},
	QUESTION: {"mysql", "sqlite3", "nrmysql", "nrsqlite3"},
	NAMED:    {"oci8", "ora", "goracle", "godror"},
	AT:       {"sqlserver", "azuresql"},
}

var binds sync.Map

func init() {
	for bind, drivers := range defaultBinds {
		for _, driver := range drivers {
			BindDriver(driver, bind)
		}
	}

}

// BindType returns the bindtype for a given database given a drivername.
func BindType(driverName string) int {
	itype, ok := binds.Load(driverName)
	if !ok {
		return UNKNOWN
	}
	return itype.(int)
}

// BindDriver sets the BindType for driverName to bindType.
func BindDriver(driverName string, bindType int) {
	binds.Store(driverName, bindType)
}

// Rebind a query from the default bindtype (QUESTION) to the target bindtype.
// A '?' inside a string literal, a quoted identifier or a comment is left
// alone, and '??' is the escape for a literal '?' with every bind type.
func Rebind(bindType int, query string) string {
	switch bindType {
	case QUESTION, UNKNOWN:
		if !strings.Contains(query, "??") {
			return query
		}
	}

	// Add space enough for 10 params before we have to allocate
	rqb := make([]byte, 0, len(query)+10)

	sc := sqlScanner{q: query, opt: scanOptsFor(bindType)}
	last, j := 0, 0
	for {
		kind, start, end := sc.next()
		if kind == tokEOF {
			break
		}
		switch kind {
		case tokQuestion:
			rqb = append(rqb, query[last:start]...)
			switch bindType {
			case DOLLAR:
				rqb = append(rqb, '$')
			case NAMED:
				rqb = append(rqb, ':', 'a', 'r', 'g')
			case AT:
				rqb = append(rqb, '@', 'p')
			default:
				rqb = append(rqb, '?')
				last = end
				continue
			}
			j++
			rqb = strconv.AppendInt(rqb, int64(j), 10)
			last = end
		case tokQuestionPair:
			rqb = append(rqb, query[last:start]...)
			rqb = append(rqb, '?')
			last = end
		}
	}

	return string(append(rqb, query[last:]...))
}

// scanOptsFor returns the scanner options for a bind type: SQL Server's
// [bracketed] identifiers are only recognised for AT.
func scanOptsFor(bindType int) scanOpts {
	if bindType == AT {
		return optBrackets
	}
	return 0
}

// Experimental implementation of Rebind which uses a bytes.Buffer.  The code is
// much simpler and should be more resistant to odd unicode, but it is twice as
// slow.  Kept here for benchmarking purposes and to possibly replace Rebind if
// problems arise with its somewhat naive handling of unicode.
func rebindBuff(bindType int, query string) string {
	if bindType != DOLLAR {
		return query
	}

	b := make([]byte, 0, len(query))
	rqb := bytes.NewBuffer(b)
	j := 1
	for _, r := range query {
		if r == '?' {
			rqb.WriteRune('$')
			rqb.WriteString(strconv.Itoa(j))
			j++
		} else {
			rqb.WriteRune(r)
		}
	}

	return rqb.String()
}

func asSliceForIn(i interface{}) (v reflect.Value, ok bool) {
	if i == nil {
		return reflect.Value{}, false
	}

	v = reflect.ValueOf(i)
	t := reflectx.Deref(v.Type())

	// Only expand slices
	if t.Kind() != reflect.Slice {
		return reflect.Value{}, false
	}

	// []byte is a driver.Value type so it should not be expanded
	if t == reflect.TypeOf([]byte{}) {
		return reflect.Value{}, false

	}

	return v, true
}

// In expands slice values in args, returning the modified query string
// and a new arg list that can be executed by a database.
//
// The query must use the `?` bindVar (not `$1`, `@p1`, etc.), even when
// targeting PostgreSQL or other drivers that normally use a different
// placeholder style. In always returns a query that still uses `?`; call
// db.Rebind on the result before Query/Exec so the placeholders match the
// driver. A `?` inside a string literal, a quoted identifier or a comment is
// not a bindVar, and `??` is a literal `?`.
//
// An empty or nil slice expands to the SQL literal NULL and binds no
// argument, so `x IN (?)` becomes `x IN (NULL)`, which matches no row on any
// database. Note that `x NOT IN (NULL)` also matches no row (SQL three-valued
// logic): if an empty list must mean "every row", build that branch of the
// query yourself.
func In(query string, args ...interface{}) (string, []interface{}, error) {
	// argMeta stores reflect.Value and length for slices and
	// the value itself for non-slice arguments
	type argMeta struct {
		v      reflect.Value
		i      interface{}
		length int
		empty  bool // a slice of length 0, rewritten to the literal NULL
	}

	var flatArgsCount int
	var anySlices bool

	var stackMeta [32]argMeta

	var meta []argMeta
	if len(args) <= len(stackMeta) {
		meta = stackMeta[:len(args)]
	} else {
		meta = make([]argMeta, len(args))
	}

	for i, arg := range args {
		if a, ok := arg.(driver.Valuer); ok {
			var err error
			arg, err = callValuerValue(a)
			if err != nil {
				return "", nil, err
			}
		}

		if v, ok := asSliceForIn(arg); ok {
			meta[i].length = v.Len()
			meta[i].v = v

			anySlices = true
			flatArgsCount += meta[i].length
			meta[i].empty = meta[i].length == 0
		} else {
			meta[i].i = arg
			flatArgsCount++
		}
	}

	// don't do any parsing if there aren't any slices;  note that this means
	// some errors that we might have caught below will not be returned.
	if !anySlices {
		return query, args, nil
	}

	newArgs := make([]interface{}, 0, flatArgsCount)

	var buf strings.Builder
	buf.Grow(len(query) + len(", ?")*flatArgsCount)

	// walk the query with the scanner so that a '?' inside a string literal
	// or a comment is not taken for a bindvar; '??' and ':name' pass through
	sc := sqlScanner{q: query}
	arg, last := 0, 0
	for {
		kind, start, end := sc.next()
		if kind == tokEOF {
			break
		}
		if kind != tokQuestion {
			continue
		}
		if arg >= len(meta) {
			// if an argument wasn't passed, lets return an error;  this is
			// not actually how database/sql Exec/Query works, but since we are
			// creating an argument list programmatically, we want to be able
			// to catch these programmer errors earlier.
			return "", nil, errors.New("number of bindVars exceeds arguments")
		}

		argMeta := meta[arg]
		arg++

		// an empty slice becomes the literal NULL, which binds nothing
		if argMeta.empty {
			buf.WriteString(query[last:start])
			buf.WriteString("NULL")
			last = end
			continue
		}

		// not a slice: its '?' is copied along with the surrounding text
		if argMeta.length == 0 {
			newArgs = append(newArgs, argMeta.i)
			continue
		}

		// write everything up to and including our ? character
		buf.WriteString(query[last:end])

		for si := 1; si < argMeta.length; si++ {
			buf.WriteString(", ?")
		}

		newArgs = appendReflectSlice(newArgs, argMeta.v, argMeta.length)
		last = end
	}

	buf.WriteString(query[last:])

	if arg < len(meta) {
		return "", nil, errors.New("number of bindVars less than number arguments")
	}

	return buf.String(), newArgs, nil
}

func appendReflectSlice(args []interface{}, v reflect.Value, vlen int) []interface{} {
	switch val := v.Interface().(type) {
	case []interface{}:
		args = append(args, val...)
	case []int:
		for i := range val {
			args = append(args, val[i])
		}
	case []string:
		for i := range val {
			args = append(args, val[i])
		}
	default:
		for si := 0; si < vlen; si++ {
			args = append(args, v.Index(si).Interface())
		}
	}

	return args
}

// callValuerValue returns vr.Value(), with one exception:
// If vr.Value is an auto-generated method on a pointer type and the
// pointer is nil, it would panic at runtime in the panicwrap
// method. Treat it like nil instead.
// Issue 8415.
//
// This is so people can implement driver.Value on value types and
// still use nil pointers to those types to mean nil/NULL, just like
// string/*string.
//
// This function is copied from database/sql/driver package
// and mirrored in the database/sql package.
func callValuerValue(vr driver.Valuer) (v driver.Value, err error) {
	if rv := reflect.ValueOf(vr); rv.Kind() == reflect.Pointer &&
		rv.IsNil() &&
		rv.Type().Elem().Implements(reflect.TypeOf((*driver.Valuer)(nil)).Elem()) {
		return nil, nil
	}
	return vr.Value()
}
