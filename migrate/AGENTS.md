## Migration Authoring Rules

### Versioning Contract

Migrations are versioned and executed once by the migration framework.

Migration code in this directory MUST NOT add defensive re-execution guards such as:

- `HasTable` checks before `CreateTable` or `DropTable`
- `HasColumn` checks before `AddColumn` or `DropColumn`
- `HasIndex` checks before `CreateIndex` or `DropIndex`

Write migration steps as direct, deterministic schema transitions for the target version.

### GORM and Migration Library Versions

Use these versions when writing migrations:
- `go-gormigrate` `v2.1.0`
- `gorm` `v1.25.2`

Before editing a migration, check the official documentation for these exact versions to confirm the correct syntax and APIs. Do not hand-write SQL unless there is no supported GORM-based approach.

### Target DB

The online system is using MySQL v8.1.0 as the DB server. Make sure the migrations are compatible with it.
