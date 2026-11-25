package migrations

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openM20261125TestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:m20261125?mode=memory"), &gorm.Config{})
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

func seedM20261125Prerequisites(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.AutoMigrate(
		&loadedModelForM20261125Test{},
		&nodeInMemoryModelForM20261125Test{},
		&gpuExecutionCalibrationForM20261125Test{},
		&inferenceTaskForM20261125Test{},
	); err != nil {
		t.Fatalf("create prerequisite tables: %v", err)
	}

	hashA := make([]byte, 32)
	hashB := make([]byte, 32)
	hashB[0] = 1
	if err := db.Create(&loadedModelForM20261125Test{
		RequiredModelFilesHash: hashA, ModelCacheHash: hashA, ExecutionDType: "float16",
		RequiredModelFileHashes: hashA, QuantizationConfig: "null", ModelID: "org/model",
		ModelCacheConfig: `{"dtype":"float16","task_kind":"llm"}`, ModelType: "llm", MinVRAM: 24,
	}).Error; err != nil {
		t.Fatalf("seed concrete loaded model: %v", err)
	}
	if err := db.Create(&loadedModelForM20261125Test{
		RequiredModelFilesHash: hashA, ModelCacheHash: hashA, ExecutionDType: "bfloat16",
		RequiredModelFileHashes: hashA, QuantizationConfig: "null", ModelID: "org/model",
		ModelCacheConfig: `{"dtype":"float16","task_kind":"llm"}`, ModelType: "llm", MinVRAM: 16,
	}).Error; err != nil {
		t.Fatalf("seed duplicate loaded model: %v", err)
	}
	if err := db.Create(&loadedModelForM20261125Test{
		RequiredModelFilesHash: hashB, ModelCacheHash: hashB, ExecutionDType: "auto",
		RequiredModelFileHashes: hashB, QuantizationConfig: "null", ModelID: "org/auto",
		ModelCacheConfig: `{"dtype":"auto","task_kind":"llm"}`, ModelType: "llm", MinVRAM: 8,
	}).Error; err != nil {
		t.Fatalf("seed auto loaded model: %v", err)
	}

	if err := db.Create(&nodeInMemoryModelForM20261125Test{
		NodeAddress: "0x1", CurrentModelCacheHash: hashA, CurrentModelFileHashes: hashA,
		CurrentModelCacheConfig: `{"dtype":"float16"}`, ExecutionDType: "float16",
	}).Error; err != nil {
		t.Fatalf("seed in-memory model: %v", err)
	}

	zeroHash := make([]byte, 32)
	if err := db.Create(&gpuExecutionCalibrationForM20261125Test{
		TaskType: 1, GPUName: "A100", GPUVram: 40, ModelName: "org/model",
		ExecutionDType: "float16", RequiredModelFilesHash: zeroHash, QuantizationConfigHash: zeroHash,
	}).Error; err != nil {
		t.Fatalf("seed concrete calibration: %v", err)
	}
	if err := db.Create(&gpuExecutionCalibrationForM20261125Test{
		TaskType: 1, GPUName: "A100", GPUVram: 40, ModelName: "org/auto",
		ExecutionDType: "auto", RequiredModelFilesHash: zeroHash, QuantizationConfigHash: zeroHash,
	}).Error; err != nil {
		t.Fatalf("seed auto calibration: %v", err)
	}

	if err := db.Create(&inferenceTaskForM20261125Test{
		RequestedDType: "bfloat16", ExecutionDType: "bfloat16",
	}).Error; err != nil {
		t.Fatalf("seed inference task: %v", err)
	}
}

func TestM20261125ConvergesDTypeColumns(t *testing.T) {
	db := openM20261125TestDB(t)
	seedM20261125Prerequisites(t, db)

	if err := M20261125(db).Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if n := countRows(t, db, "loaded_models"); n != 1 {
		t.Fatalf("expected 1 loaded_models row after prune, got %d", n)
	}
	var loadedMinVRAM uint64
	var loadedDType string
	if err := db.Table("loaded_models").Select("min_vram", "dtype").Limit(1).Row().Scan(&loadedMinVRAM, &loadedDType); err != nil {
		t.Fatalf("read loaded model: %v", err)
	}
	if loadedMinVRAM != 16 || loadedDType != "float16" {
		t.Fatalf("unexpected loaded model min_vram=%d dtype=%q", loadedMinVRAM, loadedDType)
	}
	if db.Migrator().HasColumn(&nodeInMemoryModelForM20261125Test{}, "ExecutionDType") {
		t.Fatal("expected node_in_memory_models.execution_dtype to be dropped")
	}
	if n := countRows(t, db, "gpu_execution_calibrations"); n != 1 {
		t.Fatalf("expected 1 calibration row after prune, got %d", n)
	}
	var calibrationDType string
	if err := db.Table("gpu_execution_calibrations").Select("dtype").Limit(1).Scan(&calibrationDType).Error; err != nil {
		t.Fatalf("read calibration dtype: %v", err)
	}
	if calibrationDType != "float16" {
		t.Fatalf("unexpected calibration dtype %q", calibrationDType)
	}
	if db.Migrator().HasColumn(&inferenceTaskExecutionDTypeForM20261125{}, "ExecutionDType") {
		t.Fatal("expected inference_tasks.execution_dtype to be dropped")
	}
	var taskDType string
	if err := db.Table("inference_tasks").Select("dtype").Limit(1).Scan(&taskDType).Error; err != nil {
		t.Fatalf("read inference task dtype: %v", err)
	}
	if taskDType != "bfloat16" {
		t.Fatalf("unexpected inference task dtype %q", taskDType)
	}
}

func TestM20261125RollbackRestoresOldColumns(t *testing.T) {
	db := openM20261125TestDB(t)
	seedM20261125Prerequisites(t, db)

	migration := M20261125(db)
	if err := migration.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := migration.RollbackLast(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	if !db.Migrator().HasColumn(&loadedModelForM20261125Test{}, "ExecutionDType") {
		t.Fatal("expected loaded_models.execution_dtype after rollback")
	}
	if !db.Migrator().HasColumn(&nodeInMemoryModelForM20261125Test{}, "ExecutionDType") {
		t.Fatal("expected node_in_memory_models.execution_dtype after rollback")
	}
	if !db.Migrator().HasColumn(&gpuExecutionCalibrationForM20261125Test{}, "ExecutionDType") {
		t.Fatal("expected gpu_execution_calibrations.execution_dtype after rollback")
	}
	if !db.Migrator().HasColumn(&inferenceTaskForM20261125Test{}, "RequestedDType") {
		t.Fatal("expected inference_tasks.requested_dtype after rollback")
	}
	if !db.Migrator().HasColumn(&inferenceTaskForM20261125Test{}, "ExecutionDType") {
		t.Fatal("expected inference_tasks.execution_dtype after rollback")
	}
}
