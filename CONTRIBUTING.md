# Contributing

Bug reports, fixes, tests and documentation are all welcome.

## Reporting a bug

Open an issue with the bug report template. The driver and database
version, the SQL, the struct or map you bind or scan, and what you expected
are what make a report reproducible. A failing test in the style of
`sqlx_test.go` is the best report of all. Questions about how to use sqlx go
to [Discussions](https://github.com/secondwindoss/sqlx/discussions).

## Making a change

1. Branch from `master`.
2. Add or update a test. The suite always runs against SQLite and also runs
   against Postgres and MySQL when their DSN variables are set:

       docker run -d --name sqlx-pg -e POSTGRES_PASSWORD=sqlx -p 5432:5432 postgres:16
       docker run -d --name sqlx-mysql -e MYSQL_ROOT_PASSWORD=sqlx -e MYSQL_DATABASE=sqlx -p 3306:3306 mysql:8
       export SQLX_POSTGRES_DSN='postgres://postgres:sqlx@localhost:5432/postgres?sslmode=disable'
       export SQLX_MYSQL_DSN='root:sqlx@tcp(127.0.0.1:3306)/sqlx'
       export SQLX_SQLITE_DSN=/tmp/sqlx.db
       make test-race

   The test binary prints `Disabling Postgres tests` or `Disabling MySQL
   tests` when it cannot connect; CI fails on either line, so make sure both
   databases are reachable before you rely on a local run.
3. Run `make tooling` once, then `make lint fmt vuln-check` before pushing.
   If the change alters what `Named`, `In` or `Rebind` emit, run
   `go test -run TestGolden -update`, review the diff of
   `testdata/golden.txt`, and mention the changed lines in the pull request.
4. Open a pull request against `master`. CI runs the lint job and the race
   tests on Go 1.23 to stable with all three databases and must be green
   before the merge.

Keep the exported API unchanged within v1 unless the change is purely
additive. The `db` tag rules, the default `NameMapper`, the bind constants and
the output of `Named`, `Rebind` and `In` for existing queries are a
compatibility contract.

## Importing an upstream pull request

Pull requests that were opened against upstream `jmoiron/sqlx` are welcome
here. Fetch the pull request head and merge it, so the original commits keep
their authors, then add a line to `CHANGELOG.md` naming the upstream pull
request number and author.

## Commit messages

One sentence that says what the change does.
