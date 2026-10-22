package migrations

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type inferenceTaskForM20261022Test struct {
	ID           uint   `gorm:"primaryKey"`
	ModelVariant string `gorm:"size:191;not null;default:''"`
}

func (inferenceTaskForM20261022Test) TableName() string { return "inference_tasks" }

func openM20261022TestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:m20261022?mode=memory"), &gorm.Config{})
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

func seedM20261022Prerequisites(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.AutoMigrate(
		&inferenceTaskForM20261022Test{},
		&loadedModelWithVariantForM20261022{},
		&gpuExecutionCalibrationKeyWithVariantForM20261022{},
	); err != nil {
		t.Fatalf("create prerequisite tables: %v", err)
	}

	hash := make([]byte, 32)
	seeds := []interface{}{
		&inferenceTaskForM20261022Test{ModelVariant: "fp16"},
		&loadedModelWithVariantForM20261022{
			RequiredModelFilesHash: hash, ModelCacheHash: hash, ExecutionDType: "float16",
			RequiredModelFileHashes: hash, QuantizationConfig: "null", ModelID: "org/model",
			Variant: "fp16", ModelType: "sd", MinVRAM: 8,
		},
		&gpuExecutionCalibrationKeyWithVariantForM20261022{
			GPUName: "gpu", GPUVram: 24, ModelName: "org/model", ModelVariant: "fp16",
			ExecutionDType: "float16", RequiredModelFilesHash: hash, QuantizationConfigHash: hash,
		},
	}
	for _, seed := range seeds {
		if err := db.Create(seed).Error; err != nil {
			t.Fatalf("seed %T: %v", seed, err)
		}
	}
}

func TestM20261022ReplacesVariantWithModelCacheConfig(t *testing.T) {
	db := openM20261022TestDB(t)
	seedM20261022Prerequisites(t, db)

	if err := M20261022(db).Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	for _, table := range []string{"loaded_models", "gpu_execution_calibrations"} {
		if n := countRows(t, db, table); n != 0 {
			t.Fatalf("expected %s to be empty, got %d rows", table, n)
		}
	}
	if n := countRows(t, db, "inference_tasks"); n != 1 {
		t.Fatalf("expected inference_tasks rows to be kept, got %d", n)
	}

	migrator := db.Migrator()
	if !migrator.HasColumn(&loadedModelForM20261022{}, "ModelCacheConfig") || migrator.HasColumn(&loadedModelWithVariantForM20261022{}, "Variant") {
		t.Fatal("expected loaded_models to use model_cache_config without variant")
	}
	if !migrator.HasIndex(&loadedModelForM20261022{}, "idx_loaded_models_identity") {
		t.Fatal("expected loaded_models identity index")
	}
	if migrator.HasColumn(&inferenceTaskModelVariantForM20261022{}, "ModelVariant") {
		t.Fatal("expected inference_tasks.model_variant to be dropped")
	}
	if migrator.HasColumn(&gpuExecutionCalibrationModelVariantForM20261022{}, "ModelVariant") {
		t.Fatal("expected gpu_execution_calibrations.model_variant to be dropped")
	}
	if !migrator.HasIndex(&gpuExecutionCalibrationKeyForM20261022{}, "idx_gpu_execution_calibration_config") {
		t.Fatal("expected calibration key index")
	}

	if err := db.Table("loaded_models").Create(map[string]interface{}{
		"required_model_files_hash": make([]byte, 32), "model_cache_hash": make([]byte, 32),
		"execution_dtype": "float16", "required_model_file_hashes": make([]byte, 32),
		"quantization_config": "null", "model_id": "org/model", "model_type": "llm", "min_vram": 8,
	}).Error; err == nil {
		t.Fatal("expected model_cache_config to be NOT NULL")
	}

	hash := make([]byte, 32)
	key := gpuExecutionCalibrationKeyForM20261022{
		GPUName: "gpu", GPUVram: 24, ModelName: "org/model", ExecutionDType: "float16",
		RequiredModelFilesHash: hash, QuantizationConfigHash: hash,
	}
	if err := db.Create(&key).Error; err != nil {
		t.Fatalf("insert calibration: %v", err)
	}
	if err := db.Create(&key).Error; err == nil {
		t.Fatal("expected unique violation on the calibration key")
	}
}

func TestM20261022RollbackRestoresVariantSchema(t *testing.T) {
	db := openM20261022TestDB(t)
	seedM20261022Prerequisites(t, db)

	migration := M20261022(db)
	if err := migration.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := migration.RollbackLast(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	migrator := db.Migrator()
	if !migrator.HasColumn(&loadedModelWithVariantForM20261022{}, "Variant") || migrator.HasColumn(&loadedModelForM20261022{}, "ModelCacheConfig") {
		t.Fatal("expected loaded_models with variant and without model_cache_config")
	}
	if !migrator.HasColumn(&inferenceTaskModelVariantForM20261022{}, "ModelVariant") {
		t.Fatal("expected inference_tasks.model_variant")
	}
	if !migrator.HasColumn(&gpuExecutionCalibrationModelVariantForM20261022{}, "ModelVariant") {
		t.Fatal("expected gpu_execution_calibrations.model_variant")
	}
	if !migrator.HasIndex(&gpuExecutionCalibrationKeyWithVariantForM20261022{}, "idx_gpu_execution_calibration_config") {
		t.Fatal("expected calibration key index with variant")
	}
	for _, table := range []string{"loaded_models", "gpu_execution_calibrations"} {
		if n := countRows(t, db, table); n != 0 {
			t.Fatalf("expected %s to be empty, got %d rows", table, n)
		}
	}
}
