package migrations

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openM20261023TestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:m20261023?mode=memory"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func seedM20261023Prerequisites(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.AutoMigrate(&loadedModelForM20261023Test{}, &nodeInMemoryModelForM20261023Test{}); err != nil {
		t.Fatalf("create prerequisite tables: %v", err)
	}
	hash := make([]byte, 32)
	if err := db.Create(&loadedModelForM20261023Test{
		RequiredModelFilesHash: hash, ModelCacheHash: hash, ExecutionDType: "float16",
		RequiredModelFileHashes: hash, QuantizationConfig: "null", ModelID: "org/model",
		ModelCacheConfig: `{"task_kind":"llm"}`, ModelType: "llm", MinVRAM: 8,
	}).Error; err != nil {
		t.Fatalf("seed loaded model: %v", err)
	}
	if err := db.Create(&nodeInMemoryModelForM20261023Test{
		NodeAddress: "0x1", CurrentModelCacheHash: hash, CurrentModelFileHashes: hash,
		CurrentModelCacheConfig: `{"dtype":"auto"}`, ExecutionDType: "bfloat16",
	}).Error; err != nil {
		t.Fatalf("seed in-memory model: %v", err)
	}
}

func TestM20261023StoresCacheConfigAsLongtext(t *testing.T) {
	db := openM20261023TestDB(t)
	seedM20261023Prerequisites(t, db)

	if err := M20261023(db).Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if n := countRows(t, db, "loaded_models"); n != 0 {
		t.Fatalf("expected loaded_models to be empty, got %d rows", n)
	}
	if n := countRows(t, db, "node_in_memory_models"); n != 1 {
		t.Fatalf("expected node_in_memory_models rows to be kept, got %d", n)
	}

	hash := make([]byte, 32)
	if err := db.Table("loaded_models").Create(map[string]interface{}{
		"required_model_files_hash": hash, "model_cache_hash": hash,
		"execution_dtype": "float16", "required_model_file_hashes": hash,
		"quantization_config": "null", "model_id": "org/model",
		"model_cache_config": `{"task_kind":"llm"}`, "model_type": "llm", "min_vram": 8,
	}).Error; err != nil {
		t.Fatalf("insert loaded model after migrate: %v", err)
	}

	var loadedConfig string
	if err := db.Table("loaded_models").Select("model_cache_config").Limit(1).Scan(&loadedConfig).Error; err != nil {
		t.Fatalf("read loaded model config: %v", err)
	}
	if loadedConfig != `{"task_kind":"llm"}` {
		t.Fatalf("unexpected loaded model config: %q", loadedConfig)
	}

	var memoryConfig string
	if err := db.Table("node_in_memory_models").Select("current_model_cache_config").Limit(1).Scan(&memoryConfig).Error; err != nil {
		t.Fatalf("read in-memory model config: %v", err)
	}
	if memoryConfig != `{"dtype":"auto"}` {
		t.Fatalf("unexpected in-memory model config: %q", memoryConfig)
	}
}

func TestM20261023RollbackRestoresJSONConfig(t *testing.T) {
	db := openM20261023TestDB(t)
	seedM20261023Prerequisites(t, db)

	migration := M20261023(db)
	if err := migration.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := migration.RollbackLast(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	if n := countRows(t, db, "loaded_models"); n != 0 {
		t.Fatalf("expected loaded_models empty after rollback, got %d", n)
	}
	if n := countRows(t, db, "node_in_memory_models"); n != 1 {
		t.Fatalf("expected node_in_memory_models rows kept after rollback, got %d", n)
	}

	hash := make([]byte, 32)
	if err := db.Table("loaded_models").Create(map[string]interface{}{
		"required_model_files_hash": hash, "model_cache_hash": hash,
		"execution_dtype": "float16", "required_model_file_hashes": hash,
		"quantization_config": "null", "model_id": "org/model",
		"model_cache_config": `{"task_kind":"llm"}`, "model_type": "llm", "min_vram": 8,
	}).Error; err != nil {
		t.Fatalf("insert loaded model after rollback: %v", err)
	}
}
