# Changelog

Notable changes to this fork, newest first. An entry that imports an upstream
pull request names its number and author. Releases up to v1.4.0 are releases of
upstream [jmoiron/sqlx](https://github.com/jmoiron/sqlx).

## Unreleased

- The module path is now `github.com/secondwindoss/sqlx`. The package name
  stays `sqlx` and the exported API is unchanged (#2).
- CI runs on GitHub Actions: `go vet`, `staticcheck`, `goimports` and
  `govulncheck`, plus the race tests on Go 1.23 to stable against Postgres,
  MySQL and SQLite (#1).

## v1.4.0 (2024-04-15, upstream)

The last upstream release; see its
[release notes](https://github.com/jmoiron/sqlx/releases/tag/v1.4.0).
