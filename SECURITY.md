# Security policy

## Supported versions

The latest fork release receives security fixes. Versions up to v1.4.0 are
upstream `jmoiron/sqlx` releases and are not patched here.

## Reporting a vulnerability

Report it privately through GitHub at
<https://github.com/secondwindoss/sqlx/security/advisories/new>. Please do not
open a public issue. You will hear back from a maintainer, and the fix and the
advisory are published together once a release is out.

sqlx passes the query strings you give it to the driver as they are; it does
not validate or escape SQL. A query built from untrusted input is an injection
in the calling code, not in sqlx, and is out of scope.
