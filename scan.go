package sqlx

// The SQL scanner shared by Rebind, In and compileNamedQuery. It reports only
// the tokens those callers care about (a '?' placeholder, the '??' escape and
// a ':name' parameter) and treats everything else as opaque text, so that a
// '?' or ':' inside a string literal, a quoted identifier, a comment or a
// Postgres dollar-quoted string is never taken for one.

import "strings"

type tokKind uint8

const (
	tokEOF          tokKind = iota
	tokQuestion             // a single '?' placeholder; end-start == 1
	tokQuestionPair         // '??', an escaped literal '?'; end-start == 2
	tokName                 // ':' followed by a name; the name is q[start+1:end]
)

type scanOpts uint8

const (
	// optBrackets makes [...] a delimited identifier (SQL Server). Off by
	// default because Postgres uses [ ] for ARRAY[...] and subscripts.
	optBrackets scanOpts = 1 << iota
)

// sqlScanner walks a query from left to right. It holds no heap state.
type sqlScanner struct {
	q   string
	pos int
	opt scanOpts
}

// stopByte marks the bytes the scanner has to look at; every other byte is
// skipped in a tight loop.
var stopByte = [256]bool{
	'?': true, ':': true, '\'': true, '"': true, '`': true,
	'-': true, '/': true, '$': true, '[': true,
}

func isLetter(b byte) bool { return 'a' <= b|0x20 && b|0x20 <= 'z' }
func isDigit(b byte) bool  { return '0' <= b && b <= '9' }

// isIdentByte: a byte that can be part of an unquoted identifier (letters,
// digits, '_', '$', and any non-ASCII byte).
func isIdentByte(b byte) bool { return isLetter(b) || isDigit(b) || b == '_' || b == '$' || b >= 0x80 }

// name rules for ':name': must start with a letter, '_' or a non-ASCII byte;
// may continue with letters, digits, '_' and non-ASCII bytes; a '.' is part
// of the name only when followed by another name byte (":place.id").
func isNameStart(b byte) bool { return isLetter(b) || b == '_' || b >= 0x80 }
func isNameByte(b byte) bool  { return isLetter(b) || isDigit(b) || b == '_' || b >= 0x80 }

// next returns the next significant token. kind==tokEOF when the input is
// exhausted. start and end are byte offsets into the query.
func (s *sqlScanner) next() (kind tokKind, start, end int) {
	q := s.q
	n := len(q)
	i := s.pos
	for i < n {
		b := q[i]
		if !stopByte[b] {
			i++
			continue
		}
		switch b {
		case '?':
			if i+1 < n && q[i+1] == '?' {
				s.pos = i + 2
				return tokQuestionPair, i, i + 2
			}
			s.pos = i + 1
			return tokQuestion, i, i + 1
		case ':':
			if i+1 < n && q[i+1] == ':' { // '::' cast (or SQL Server geography::Point)
				i += 2
				continue
			}
			if i+1 < n && isNameStart(q[i+1]) {
				j := i + 2
				for j < n {
					c := q[j]
					if isNameByte(c) || (c == '.' && j+1 < n && isNameByte(q[j+1])) {
						j++
						continue
					}
					break
				}
				s.pos = j
				return tokName, i, j
			}
			i++ // ':=' , ' : ', ':1', ':)' ... are plain text
		case '\'':
			// E'...' (Postgres escape string): backslash escapes are certain.
			esc := i > 0 && (q[i-1] == 'E' || q[i-1] == 'e') && (i < 2 || !isIdentByte(q[i-2]))
			if j, ok := scanQuoted(q, i+1, '\'', esc); ok {
				i = j
			} else {
				i++ // unterminated: not a string after all, treat the quote as text
			}
		case '"':
			if j, ok := scanQuoted(q, i+1, '"', false); ok {
				i = j
			} else {
				i++
			}
		case '`':
			if j, ok := scanBacktick(q, i+1); ok {
				i = j
			} else {
				i++
			}
		case '[':
			if s.opt&optBrackets != 0 {
				if j, ok := scanBracket(q, i+1); ok {
					i = j
					continue
				}
			}
			i++
		case '-':
			if i+1 < n && q[i+1] == '-' {
				if j := strings.IndexByte(q[i:], '\n'); j >= 0 {
					i += j + 1
				} else {
					i = n
				}
			} else {
				i++
			}
		case '/':
			if i+1 < n && q[i+1] == '*' {
				i = skipBlockComment(q, i+2)
			} else {
				i++
			}
		case '$':
			if i == 0 || !isIdentByte(q[i-1]) {
				if j, ok := scanDollarQuote(q, i); ok {
					i = j
					continue
				}
			}
			i++
		}
	}
	s.pos = n
	return tokEOF, n, n
}

// scanQuoted scans a '...' or "..." literal whose opening quote is at i-1.
// A doubled quote is always an escaped quote. A backslash is an escape only
// (a) in Postgres E'...' strings (escapeStrings), or (b) when it is followed by
// the quote character and the number of quote characters remaining in the
// query after that pair is odd (the MySQL \' heuristic; see the design doc).
// It returns the offset just past the closing quote, or ok=false when the
// literal is unterminated.
func scanQuoted(q string, i int, quote byte, escapeStrings bool) (end int, ok bool) {
	n := len(q)
	for i < n {
		switch q[i] {
		case quote:
			if i+1 < n && q[i+1] == quote {
				i += 2
				continue
			}
			return i + 1, true
		case '\\':
			if i+1 >= n {
				return 0, false
			}
			switch {
			case escapeStrings:
				i += 2 // E'..\x..': the next byte is always escaped
			case q[i+1] == '\\':
				i += 2 // '\\' reads the same under both rules
			case q[i+1] == quote:
				if strings.Count(q[i+2:], string(quote))%2 == 1 {
					i += 2 // MySQL-style escaped quote: 'don\'t'
				} else {
					i++ // standard: the backslash is text and the quote closes: ESCAPE '\'
				}
			default:
				i++
			}
		default:
			i++
		}
	}
	return 0, false
}

// scanBacktick: MySQL/SQLite `identifier`; a doubled backtick is an escaped
// backtick, there are no backslash escapes.
func scanBacktick(q string, i int) (end int, ok bool) {
	n := len(q)
	for i < n {
		if q[i] == '`' {
			if i+1 < n && q[i+1] == '`' {
				i += 2
				continue
			}
			return i + 1, true
		}
		i++
	}
	return 0, false
}

// scanBracket: SQL Server [identifier]; ]] is an escaped ].
func scanBracket(q string, i int) (end int, ok bool) {
	n := len(q)
	for i < n {
		if q[i] == ']' {
			if i+1 < n && q[i+1] == ']' {
				i += 2
				continue
			}
			return i + 1, true
		}
		i++
	}
	return 0, false
}

// skipBlockComment skips a /* */ comment whose opener ends at i. Comments
// nest (SQL standard, Postgres, SQL Server). An unterminated comment runs to
// the end of the input (SQLite rule; Postgres/MySQL reject the query anyway).
func skipBlockComment(q string, i int) int {
	n := len(q)
	depth := 1
	for i+1 < n {
		if q[i] == '/' && q[i+1] == '*' {
			depth++
			i += 2
			continue
		}
		if q[i] == '*' && q[i+1] == '/' {
			depth--
			i += 2
			if depth == 0 {
				return i
			}
			continue
		}
		i++
	}
	return n
}

// scanDollarQuote scans a Postgres $tag$...$tag$ or $$...$$ string starting
// at q[i]=='$'. The tag follows unquoted-identifier rules (so "$1" is never a
// tag). An opener without a matching closer is not a dollar quote.
func scanDollarQuote(q string, i int) (end int, ok bool) {
	n := len(q)
	j := i + 1
	if j < n && isDigit(q[j]) {
		return 0, false
	}
	for j < n && (isLetter(q[j]) || isDigit(q[j]) || q[j] == '_' || q[j] >= 0x80) {
		j++
	}
	if j >= n || q[j] != '$' {
		return 0, false
	}
	tag := q[i : j+1]
	k := strings.Index(q[j+1:], tag)
	if k < 0 {
		return 0, false
	}
	return j + 1 + k + len(tag), true
}
