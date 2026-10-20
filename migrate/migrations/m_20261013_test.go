package migrations

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openM20261013TestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:m20261013?mode=memory"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func seedM20261013Prerequisites(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.AutoMigrate(
		&inferenceTaskRowsForM20261013{},
		&nodeTaskRefForM20261013{},
		&nodeModelLegacyForM20261013{},
		&loadedModelLegacyForM20261013{},
		&nodeModelDownloadSelectionLegacyForM20261013{},
		&gpuExecutionCalibrationLegacyKeyForM20261013{},
	); err != nil {
		t.Fatalf("failed to create prerequisite tables: %v", err)
	}

	now := time.Now()
	commitment := "task-1"
	active := true
	seeds := []interface{}{
		&inferenceTaskRowsForM20261013{},
		&nodeTaskRefForM20261013{Status: nodeStatusBusyForM20261013, CurrentTaskIDCommitment: &commitment},
		&nodeModelLegacyForM20261013{NodeAddress: "0x1", ModelID: "base:org/model", CreatedAt: now, UpdatedAt: now},
		&loadedModelLegacyForM20261013{ModelID: "org/model", ModelType: "llm", MinVRAM: 8, CreatedAt: now, UpdatedAt: now},
		&nodeModelDownloadSelectionLegacyForM20261013{ModelID: "base:org/model", NodeAddress: "0x1", Status: "pending", Active: &active, CreatedAt: now, UpdatedAt: now},
		&gpuExecutionCalibrationLegacyKeyForM20261013{GPUName: "gpu", GPUVram: 24, QuantizeBits: 4},
	}
	for _, seed := range seeds {
		if err := db.Create(seed).Error; err != nil {
			t.Fatalf("failed to seed %T: %v", seed, err)
		}
	}
}

func countRows(t *testing.T, db *gorm.DB, table string) int64 {
	t.Helper()
	var count int64
	if err := db.Table(table).Count(&count).Error; err != nil {
		t.Fatalf("failed to count %s: %v", table, err)
	}
	return count
}

func TestM20261013CutsOverToModelFileIdentity(t *testing.T) {
	db := openM20261013TestDB(t)
	seedM20261013Prerequisites(t, db)

	migration := M20261013(db)
	if err := migration.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	for _, table := range []string{"inference_tasks", "node_models", "loaded_models", "node_model_download_selections", "gpu_execution_calibrations"} {
		if n := countRows(t, db, table); n != 0 {
			t.Fatalf("expected %s to be cleared, got %d rows", table, n)
		}
	}

	var node nodeTaskRefForM20261013
	if err := db.First(&node).Error; err != nil {
		t.Fatalf("failed to load node: %v", err)
	}
	if node.CurrentTaskIDCommitment != nil || node.Status != nodeStatusAvailableForM20261013 {
		t.Fatalf("expected node released from the deleted task, got %+v", node)
	}

	migrator := db.Migrator()
	if !migrator.HasTable(&modelFileForM20261013{}) || !migrator.HasTable(&nodeInMemoryModelForM20261013{}) {
		t.Fatal("expected model_files and node_in_memory_models tables")
	}
	for _, column := range inferenceTaskModelFileColumnsForM20261013 {
		if !migrator.HasColumn(&inferenceTaskModelFilesForM20261013{}, column) {
			t.Fatalf("expected inference_tasks column %s", column)
		}
	}
	if migrator.HasColumn(&nodeModelLegacyForM20261013{}, "ModelID") || migrator.HasColumn(&nodeModelLegacyForM20261013{}, "InUse") {
		t.Fatal("expected node_models legacy columns to be removed")
	}
	if !migrator.HasColumn(&nodeModelForM20261013{}, "ModelFileHash") {
		t.Fatal("expected node_models.model_file_hash")
	}
	if !migrator.HasIndex(&nodeModelForM20261013{}, "idx_node_models_node_address_model_file_hash") {
		t.Fatal("expected node_models unique index")
	}
	if !migrator.HasColumn(&loadedModelForM20261013{}, "RequiredModelFilesHash") {
		t.Fatal("expected loaded_models.required_model_files_hash")
	}
	if !migrator.HasColumn(&nodeModelDownloadSelectionForM20261013{}, "ModelFileHash") ||
		migrator.HasColumn(&nodeModelDownloadSelectionLegacyForM20261013{}, "ModelID") {
		t.Fatal("expected node_model_download_selections to use model_file_hash")
	}
	for _, column := range calibrationHashColumnsForM20261013 {
		if !migrator.HasColumn(&gpuExecutionCalibrationHashesForM20261013{}, column) {
			t.Fatalf("expected gpu_execution_calibrations column %s", column)
		}
	}

	hashA := make([]byte, 32)
	hashB := make([]byte, 32)
	hashB[0] = 1
	rows := []nodeModelForM20261013{
		{NodeAddress: "0x1", ModelFileHash: hashA},
		{NodeAddress: "0x1", ModelFileHash: hashB},
		{NodeAddress: "0x2", ModelFileHash: hashA},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("failed to insert distinct node models: %v", err)
	}
	if err := db.Create(&nodeModelForM20261013{NodeAddress: "0x1", ModelFileHash: hashA}).Error; err == nil {
		t.Fatal("expected unique violation on (node_address, model_file_hash)")
	}

	key := gpuExecutionCalibrationKeyForM20261013{GPUName: "gpu", GPUVram: 24, RequiredModelFilesHash: hashA, QuantizationConfigHash: hashA}
	if err := db.Create(&key).Error; err != nil {
		t.Fatalf("failed to insert calibration: %v", err)
	}
	if err := db.Create(&key).Error; err == nil {
		t.Fatal("expected unique violation on the calibration key")
	}
	other := key
	other.QuantizationConfigHash = hashB
	if err := db.Create(&other).Error; err != nil {
		t.Fatalf("expected a different quantization hash to be a distinct calibration: %v", err)
	}

	if err := db.Table("inference_tasks").Create(map[string]interface{}{"id": 1}).Error; err == nil {
		t.Fatal("expected required model file columns to be NOT NULL")
	}
}

// SQLite cannot drop the trailing calibration column after AlterColumn rewrites the table DDL, so the full rollback and re-apply cycle is verified on MySQL.
func TestM20261013RollbackRestoresLegacySchema(t *testing.T) {
	db := openM20261013TestDB(t)
	seedM20261013Prerequisites(t, db)

	migration := M20261013(db)
	if err := migration.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}
	if err := migration.RollbackLast(); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}

	migrator := db.Migrator()
	if migrator.HasTable(&modelFileForM20261013{}) || migrator.HasTable(&nodeInMemoryModelForM20261013{}) {
		t.Fatal("expected new tables to be dropped")
	}
	for _, column := range inferenceTaskModelFileColumnsForM20261013 {
		if migrator.HasColumn(&inferenceTaskModelFilesForM20261013{}, column) {
			t.Fatalf("expected inference_tasks column %s to be dropped", column)
		}
	}
	if !migrator.HasColumn(&nodeModelLegacyForM20261013{}, "ModelID") || migrator.HasColumn(&nodeModelForM20261013{}, "ModelFileHash") {
		t.Fatal("expected legacy node_models schema")
	}
	if !migrator.HasIndex(&loadedModelLegacyForM20261013{}, "idx_loaded_models_model_id_variant") {
		t.Fatal("expected legacy loaded_models unique index")
	}
	if !migrator.HasColumn(&nodeModelDownloadSelectionLegacyForM20261013{}, "ModelID") {
		t.Fatal("expected legacy selection model_id column")
	}
	if migrator.HasColumn(&gpuExecutionCalibrationHashesForM20261013{}, "RequiredModelFilesHash") {
		t.Fatal("expected calibration column RequiredModelFilesHash to be dropped")
	}
	if !migrator.HasIndex(&gpuExecutionCalibrationLegacyKeyForM20261013{}, "idx_gpu_execution_calibration_config") {
		t.Fatal("expected legacy calibration key index")
	}
}
