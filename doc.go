// Package sqlx provides general purpose extensions to database/sql.
//
// This is the secondwindoss fork of github.com/jmoiron/sqlx. It keeps the API
// of upstream v1.4.0 and only the import path differs; the README has the
// migration steps and CHANGELOG.md lists what each release fixes.
//
// It is intended to seamlessly wrap database/sql and provide convenience
// methods which are useful in the development of database driven applications.
// None of the underlying database/sql methods are changed.  Instead all extended
// behavior is implemented through new methods defined on wrapper types.
//
// Additions include scanning into structs, named query support, rebinding
// queries for different drivers, convenient shorthands for common error handling
// and more.
package sqlx
