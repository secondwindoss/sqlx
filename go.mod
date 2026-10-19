module github.com/secondwindoss/sqlx

go 1.24.0

// Upstream jmoiron/sqlx tags that came with the fork's history. Their go.mod
// declares github.com/jmoiron/sqlx, so they cannot be used under this path.
retract [v1.2.0, v1.4.0]
