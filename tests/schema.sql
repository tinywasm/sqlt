-- dialect: sqlite
--
-- Fixture para TestExportDDL_FullSchema en sqlt/export_test.go
--
-- Cubre TODOS los tipos DB del framework y las 3 estrategias de PK:
--
--   PK auto-generado (DB):  id int64 `db:"pk,autoinc"` → INTEGER PRIMARY KEY AUTOINCREMENT
--   PK provisto por app:    id string `db:"pk"`         → TEXT PRIMARY KEY  (ej. UUID)
--   PK compuesto:           (user_id, role_id) int64    → PRIMARY KEY (user_id, role_id)
--
-- Tipos DB soportados por el framework (FieldStruct/FieldIntSlice/FieldStructSlice no son columnas DB):
--
--   FieldText  string sin max         → TEXT
--   FieldText  string con input:max=N → VARCHAR(N)
--   FieldRaw   RawJSON (string alias) → TEXT  (JSON pre-serializado, sin encoding adicional)
--   FieldInt   int64                  → INTEGER
--   FieldFloat float64                → REAL
--   FieldBool  bool                   → INTEGER  (0=false, 1=true; SQLite sin BOOLEAN nativo)
--   FieldBlob  []byte                 → BLOB
--
-- Estructuras fuente (Go):
--
--   users: id int64 `db:"pk,autoinc"`,
--          username string `db:"not_null" input:"required,max=50"` UNIQUE,
--          email string `db:"not_null"` UNIQUE,
--          score float64, active bool, avatar []byte
--
--   roles: id int64 `db:"pk,autoinc"`,
--          name string `db:"not_null" input:"required,max=100"` UNIQUE
--
--   sessions: id string `db:"pk"`,                          ← PK provisto por app (UUID)
--             user_id int64 `db:"ref=users"`,               ← ON DELETE CASCADE por defecto
--             metadata RawJSON                              ← FieldRaw → TEXT
--
--   user_roles: user_id int64 `db:"pk,ref=users"`,          ← PK compuesto
--               role_id int64 `db:"pk,ref=roles"`
--
-- Orden garantizado por TopologicalSort: users → roles → sessions → user_roles

CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY AUTOINCREMENT, username VARCHAR(50) NOT NULL UNIQUE, email TEXT NOT NULL UNIQUE, score REAL, active INTEGER, avatar BLOB);

CREATE TABLE IF NOT EXISTS roles (id INTEGER PRIMARY KEY AUTOINCREMENT, name VARCHAR(100) NOT NULL UNIQUE);

CREATE TABLE IF NOT EXISTS sessions (id TEXT PRIMARY KEY, user_id INTEGER, metadata TEXT, CONSTRAINT fk_sessions_user_id FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE);

CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);

CREATE TABLE IF NOT EXISTS user_roles (user_id INTEGER NOT NULL, role_id INTEGER NOT NULL, PRIMARY KEY (user_id, role_id), CONSTRAINT fk_user_roles_user_id FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE, CONSTRAINT fk_user_roles_role_id FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE);

CREATE INDEX IF NOT EXISTS idx_user_roles_user_id ON user_roles(user_id);

CREATE INDEX IF NOT EXISTS idx_user_roles_role_id ON user_roles(role_id);

