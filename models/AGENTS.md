# Model Requirements

Model structs MUST NOT use GORM soft delete by default. Do not embed `gorm.Model` or add `gorm.DeletedAt` unless there is a specific, confirmed requirement to preserve deleted rows as hidden application state.

When a model needs ID and timestamps, define `ID`, `CreatedAt`, and `UpdatedAt` explicitly instead of relying on `gorm.Model`.
