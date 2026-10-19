package sqlx

import (
	"strconv"
	"strings"
	"testing"
)

// fuzzSeeds covers each construct the scanner treats specially: placeholders,
// the '??' escape, named parameters, casts, literals, quoted identifiers,
// comments, dollar quotes and the backslash-quote cases from the audit.
var fuzzSeeds = []string{
	"SELECT * FROM t WHERE a = ? AND b = ?",
	"SELECT ?? FROM t WHERE a = ?",
	"SELECT * FROM t WHERE id IN (?) AND x = ?",
	"INSERT INTO t (a, b) VALUES (:a, :b)",
	"SELECT :data::jsonb AS boundary",
	"SELECT :first:name",
	"SELECT * FROM t WHERE name = ':not' AND id = :id",
	`SELECT "a?b", ` + "`c:d`" + `, [e?] FROM t WHERE x = ?`,
	"SELECT 1 -- a ? comment\nWHERE a = ?",
	"SELECT /* outer /* inner ? */ :x */ :y",
	"SELECT $$ ? :x $$, $tag$ ? $tag$, ?",
	"SELECT E'it\\'s ?' , ?",
	`SELECT * FROM t WHERE p LIKE ? ESCAPE '\' AND q = ? -- don't`,
	`SELECT * FROM t WHERE path = 'C:\' AND name = ? /* it's */`,
	`SELECT * FROM t WHERE s = 'don\'t' AND c = ? -- it's`,
	"SELECT :place.id, :ünï, :a_",
	"'unterminated ? :x",
	"",
}

var fuzzBindTypes = []int{UNKNOWN, QUESTION, DOLLAR, NAMED, AT}

type fuzzToken struct {
	kind       tokKind
	start, end int
}

// scanTokens returns every token in q, failing the test if the scanner does not
// make progress or reports a token that breaks its documented shape.
func scanTokens(t *testing.T, q string, opt scanOpts) []fuzzToken {
	t.Helper()
	sc := sqlScanner{q: q, opt: opt}
	var toks []fuzzToken
	prevEnd := 0
	for i := 0; ; i++ {
		if i > len(q)+1 {
			t.Fatalf("scanner did not reach EOF after %d tokens in %q", i, q)
		}
		kind, start, end := sc.next()
		if kind == tokEOF {
			return toks
		}
		if start < prevEnd || end <= start || end > len(q) {
			t.Fatalf("token %d [%d,%d) out of order or bounds (prev end %d, len %d) in %q", i, start, end, prevEnd, len(q), q)
		}
		switch kind {
		case tokQuestion:
			if q[start:end] != "?" {
				t.Fatalf("tokQuestion covers %q in %q", q[start:end], q)
			}
		case tokQuestionPair:
			if q[start:end] != "??" {
				t.Fatalf("tokQuestionPair covers %q in %q", q[start:end], q)
			}
		case tokName:
			if q[start] != ':' || end-start < 2 || !isNameStart(q[start+1]) {
				t.Fatalf("tokName covers %q in %q", q[start:end], q)
			}
		default:
			t.Fatalf("unknown token kind %d in %q", kind, q)
		}
		toks = append(toks, fuzzToken{kind, start, end})
		prevEnd = end
	}
}

func countKind(toks []fuzzToken, kind tokKind) int {
	n := 0
	for _, tok := range toks {
		if tok.kind == kind {
			n++
		}
	}
	return n
}

func FuzzScanner(f *testing.F) {
	for _, q := range fuzzSeeds {
		f.Add(q, false)
		f.Add(q, true)
	}
	f.Fuzz(func(t *testing.T, q string, brackets bool) {
		var opt scanOpts
		if brackets {
			opt = optBrackets
		}
		scanTokens(t, q, opt)
	})
}

func FuzzRebind(f *testing.F) {
	for _, q := range fuzzSeeds {
		f.Add(q)
	}
	f.Fuzz(func(t *testing.T, q string) {
		for _, bt := range fuzzBindTypes {
			out := Rebind(bt, q)
			toks := scanTokens(t, q, scanOptsFor(bt))
			n := countKind(toks, tokQuestion)
			pairs := countKind(toks, tokQuestionPair)
			switch bt {
			case QUESTION, UNKNOWN:
				if len(out) != len(q)-pairs {
					t.Fatalf("Rebind(%d, %q) = %q: want %d bytes, one per '??' fewer", bt, q, out, len(q)-pairs)
				}
			case DOLLAR:
				if n > 0 && !strings.Contains(out, "$"+strconv.Itoa(n)) {
					t.Fatalf("Rebind(DOLLAR, %q) = %q: missing $%d", q, out, n)
				}
			case AT:
				if n > 0 && !strings.Contains(out, "@p"+strconv.Itoa(n)) {
					t.Fatalf("Rebind(AT, %q) = %q: missing @p%d", q, out, n)
				}
			}
		}
	})
}

func FuzzCompileNamedQuery(f *testing.F) {
	for _, q := range fuzzSeeds {
		f.Add(q)
	}
	f.Fuzz(func(t *testing.T, q string) {
		for _, bt := range fuzzBindTypes {
			out, names, err := compileNamedQuery(q, bt)
			if err != nil {
				continue
			}
			for _, name := range names {
				if name == "" || !isNameStart(name[0]) {
					t.Fatalf("compileNamedQuery(%q, %d) returned name %q", q, bt, name)
				}
			}
			switch bt {
			case NAMED:
				if out != q {
					t.Fatalf("compileNamedQuery(%q, NAMED) = %q: want the query unchanged", q, out)
				}
			case DOLLAR:
				if len(names) > 0 && !strings.Contains(out, "$"+strconv.Itoa(len(names))) {
					t.Fatalf("compileNamedQuery(%q, DOLLAR) = %q: missing $%d", q, out, len(names))
				}
			}
		}
	})
}

func FuzzIn(f *testing.F) {
	for _, q := range fuzzSeeds {
		f.Add(q, uint8(3))
	}
	f.Fuzz(func(t *testing.T, q string, width uint8) {
		// One argument per '?': non-empty slices of 1 to 4 ints alternating
		// with scalars, so the expansion is never the NULL literal.
		n := countKind(scanTokens(t, q, 0), tokQuestion)
		args := make([]interface{}, n)
		want := 0
		for i := range args {
			if i%2 == 0 {
				s := make([]int, int(width)%4+1)
				args[i] = s
				want += len(s)
			} else {
				args[i] = i
				want++
			}
		}
		out, newArgs, err := In(q, args...)
		if err != nil {
			t.Fatalf("In(%q) with %d arguments: %v", q, n, err)
		}
		if n == 0 {
			return
		}
		if len(newArgs) != want {
			t.Fatalf("In(%q) returned %d arguments, want %d", q, len(newArgs), want)
		}
		if got := countKind(scanTokens(t, out, 0), tokQuestion); got != want {
			t.Fatalf("In(%q) = %q has %d placeholders for %d arguments", q, out, got, want)
		}
	})
}
