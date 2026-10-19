# Changelog

Notable changes to this fork, newest first. An entry that imports an upstream
pull request names its number and author. Releases up to v1.4.0 are releases of
upstream [jmoiron/sqlx](https://github.com/jmoiron/sqlx).

## v1.4.3 (2026-10-19)

No API change. Batch inserts report an unclosed `VALUES (` group instead of
producing SQL the database rejects, and expand correctly when the group holds a
bracket inside a string literal or comment.

### Fixes

- A batch insert (a slice or array passed to `NamedExec`, `BindNamed` and the
  like) whose `VALUES (` group never closes now returns the error
  `missing closing bracket in VALUES`, whatever the number of rows. Before, the
  query came back unexpanded with every row's arguments, and the mistake only
  surfaced when the database rejected the argument count. Single-struct and map
  arguments are unchanged (#36).
- The batch insert expansion skips brackets inside string literals, quoted
  identifiers and comments. `VALUES (:a, ')')` or `VALUES (:a, /* ) */ :b)`
  with several rows used to be cut at the bracket inside the literal or comment
  and expanded into invalid SQL; `VALUES (:a, '(')` was not expanded at all
  (#36).

## v1.4.2 (2026-10-19)

No API change. sqlx's `go.mod` now requires no other module, so importing sqlx
no longer changes which database driver versions a program builds with, and the
upstream tags that came with the fork's history are retracted.

### Module and toolchain

- sqlx's `go.mod` requires no other module. The tests that run against
  Postgres, MySQL and SQLite move to the nested module `dbtest/`, which holds
  the three drivers, so `go-sql-driver/mysql`, `lib/pq` and `go-sqlite3` no
  longer enter the module graph of a program that imports sqlx. Before, Go's
  minimum version selection raised a consumer's own MySQL driver to v1.10.1
  even though sqlx only used it in tests (upstream #642, #569, #849). The
  checks in three tests that read the unexported `unsafe` field now test its
  effect instead: scanning into a struct that lacks some selected columns
  (#34).
- v1.2.0 to v1.4.0 are retracted under this module path. They are upstream's
  tags, pushed with the history; their `go.mod` declares
  `github.com/jmoiron/sqlx`, so `go get github.com/secondwindoss/sqlx@v1.4.0`
  failed with a module path error. `go list -m -versions` now lists only the
  fork's own releases (#35).

### Project

- Release tags are signed; GitHub shows them as verified.
- An OpenSSF Scorecard workflow publishes the repository's score, shown as a
  README badge; CI actions are pinned by commit SHA; Go fuzz tests cover the
  SQL scanner, `Rebind`, `compileNamedQuery` and `In`, and their seed corpus
  runs with the unit tests (#32, #33). CodeQL scans every pull request, and
  merging into `master` requires the CI checks to pass.
- The README is rewritten around the fork, with a migration section that
  explains why a `replace` directive cannot stand in for the import rewrite
  (#28, #29, #30).

## v1.4.1 (2026-10-10)

The first fork release. It keeps every exported identifier of v1.4.0 (CI runs
`apidiff` against the v1.4.0 tag on every change), requires Go 1.24, adds one
method and fixes the bugs that sat in upstream pull requests. The documented
behaviours that change are marked **changed**.

### Module and toolchain

- The module path is `github.com/secondwindoss/sqlx`. The package name stays
  `sqlx`; only the import path changes (#2).
- **Changed:** the `go` directive is 1.24.0, up from 1.10, as upstream #1001
  (AshSgDe29071999) proposed. The test drivers move to mysql 1.10.1, pq 1.12.3
  and sqlite3 1.14.52, and the deprecated `io/ioutil` and `reflect.PtrTo`
  calls are replaced (#11). `filippo.io/edwards25519` is no longer pinned at
  the version that module scanners flag (#9, #11; upstream #994).

### Fixes

- Named queries, `In` and `Rebind` scan SQL instead of counting bytes (#23).
  A `?` or `:` inside a string literal, a quoted identifier, a `--` or nested
  `/* */` comment or a Postgres `$tag$` string is no longer a placeholder or a
  parameter. Fixes upstream #487, #608 and #961 (`?` in literals and comments,
  as #984 by Charlie Tonneslan and #768 by Daniel Morell attempted), #206,
  #253, #302, #368, #394, #441, #443, #872, #894 and #947 (`:` in literals and
  comments, as #151 by Mark Beznos, #474 by detaoin and #775 by David Sharnoff
  attempted) and #983 (a cast attached to a parameter, `:id::uuid`, as #1002
  by Burak Keskin and #985 by Charlie Tonneslan attempted). Parameter names
  may contain non-ASCII letters.
- **Changed:** `::` is a cast and passes through untouched, which fixes
  `now()::date` (upstream #428, #650, #956). The old escape that turned `::`
  into `:` is gone, so a query that wrote `::::` to get a cast now writes `::`.
- **Changed:** `??` is a literal `?` in `In` and `Rebind`, the pgJDBC and
  squirrel convention, so the Postgres JSONB operators can be written `??`,
  `??|` and `??&` (upstream #628). `Rebind(QUESTION, q)` rewrites `??` to `?`.
- **Changed:** a parameter name can no longer start with a digit; `:1` is
  text.
- **Changed:** `In` expands an empty or nil slice to `NULL` instead of
  returning an error, so `x IN (?)` becomes `x IN (NULL)` and matches no row.
  `x NOT IN (NULL)` also matches no row. Callers that used the error as an
  empty-list guard must check the length themselves (#22; upstream #945 by
  Markus, markus-wa).
- `In` no longer panics on a nil pointer whose type implements
  `driver.Valuer` (#12; upstream #953 by Alexandre Thomas, issue #952).
- Batch inserts expand a `VALUES` list that has no column list before it,
  sits in a `FROM` or `JOIN`, or starts on its own line (#13; upstream #962 by
  Caleb Spare, issues #746, #753, #772, #898; cases from #754 by MishaNiki,
  #811 by xyz1hang and #948 by stephennancekivell). The group repeated is the
  first one that contains a bind variable, so a table named `values` or a
  MySQL `VALUES(col)` is not mistaken for it.
- Named queries accept a pointer to a slice, to a map and to a slice of maps
  (#20; upstream #805 by Xudong Cai, issues #577, #729).
- A statement prepared on a `Conn` keeps the `Conn`'s `Mapper` and unsafe
  flag (#15; upstream #993 by itxaiohanglover, issue #975).
- `Tx.Stmtx` and `Tx.StmtxContext` keep the transaction's unsafe flag and the
  wrapped statement's own (#18; upstream #979 by Gianmarco Fantinuoli).
- `Rows.NextResultSet` resets the field cache, so a second result set with a
  different column layout scans instead of panicking (#19; upstream #858 by
  Kamil Akhmetov, issues #380, #857; test from #747 by Peter Magnusson).
- `ConnectContext` closes the DB and returns `nil` when the ping fails, as
  `Connect` does (#16; upstream #561 by Timo Reitz, issue #910).
- A colon followed by nothing usable is text, not an empty parameter. The
  clearer error for that case from upstream #920 by Lee Fernandes went in
  first (#17); the scanner then made the case unreachable.

### Additions

- `Tx.NamedQueryContext` (#21; upstream #724 by Dave Worth, issue #447). The
  prepared-statement queryer also gains `QueryRow` and `QueryRowContext`, from
  #941 by Jon Senchyna.
- The README and the `In` doc comment say that `In` always takes and returns
  `?` placeholders (#14; upstream #1003 by TastyHeadphones, issue #959).

### Project

- CI runs on GitHub Actions: `go vet`, `staticcheck`, `goimports`,
  `govulncheck` and `apidiff`, plus the race tests on Go 1.24 to stable
  against Postgres, MySQL and SQLite (#1, #11, #24).
- README fork notice and migration section, LICENSE line, CONTRIBUTING.md,
  SECURITY.md, a bug report template and Dependabot (#3, #4, #5, #10).

## v1.4.0 (2024-04-15, upstream)

The last upstream release; see its
[release notes](https://github.com/jmoiron/sqlx/releases/tag/v1.4.0).
