# webtyp/sqlt
<img src="docs/img/badges.svg">

SQLite SQL compiler. Implements `storage.Compiler` (DML) and `ddl.Compiler` (DDL) —
translates `storage.Query`/`ddl.Stmt` into SQLite SQL. `ExportDDL` builds full-schema
SQL (ordered via `ddl.TopologicalSort`) for build-time schema generation. Pure compiler:
no connections, no execution — see `webtyp/sqlite` for the driver that wraps a real
`*sql.DB` in a `storage.Conn`.
