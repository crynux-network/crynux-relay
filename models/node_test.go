package models

import (
	"bytes"
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openNodeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&NodeModel{}, &NodeInMemoryModel{}); err != nil {
		t.Fatalf("failed to migrate node model tables: %v", err)
	}
	return db
}

func TestNodeModelUniquePerNodeAndModelFile(t *testing.T) {
	db := openNodeTestDB(t)
	ctx := context.Background()
	hashA := Hash32{1}
	hashB := Hash32{2}

	first := NewNodeModel("0x1", hashA)
	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("failed to create node model: %v", err)
	}
	duplicate := NewNodeModel("0x1", hashA)
	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatal("expected unique violation for the same node and model file hash")
	}
	if err := db.Create(&[]NodeModel{NewNodeModel("0x1", hashB), NewNodeModel("0x2", hashA)}).Error; err != nil {
		t.Fatalf("failed to create distinct node models: %v", err)
	}

	counts, err := CountNodesByModelFileHash(ctx, db)
	if err != nil {
		t.Fatalf("failed to count nodes: %v", err)
	}
	if counts[hashA] != 2 || counts[hashB] != 1 {
		t.Fatalf("unexpected counts: %v", counts)
	}

	stored, err := GetNodeModel(ctx, db, "0x1", hashB)
	if err != nil {
		t.Fatalf("failed to get node model: %v", err)
	}
	if !bytes.Equal(stored.ModelFileHash, hashB.Bytes()) {
		t.Fatalf("unexpected stored hash: %x", stored.ModelFileHash)
	}
}

func TestNodeInMemoryModelSaveReplaceAndClear(t *testing.T) {
	db := openNodeTestDB(t)
	ctx := context.Background()

	empty, err := GetNodeInMemoryModel(ctx, db, "0x1")
	if err != nil || empty != nil {
		t.Fatalf("expected no row for empty memory, got %v, %v", empty, err)
	}

	files := []Hash32{{9}, {3}}
	row := NewNodeInMemoryModel("0x1", Hash32{7}, files, []byte(`{"dtype":"auto"}`), "bfloat16")
	if err := SaveNodeInMemoryModel(ctx, db, &row); err != nil {
		t.Fatalf("failed to save in-memory model: %v", err)
	}
	replacement := NewNodeInMemoryModel("0x1", Hash32{8}, files[:1], []byte(`{"dtype":"float16"}`), "float16")
	if err := SaveNodeInMemoryModel(ctx, db, &replacement); err != nil {
		t.Fatalf("failed to replace in-memory model: %v", err)
	}

	stored, err := GetNodeInMemoryModel(ctx, db, "0x1")
	if err != nil || stored == nil {
		t.Fatalf("failed to load in-memory model: %v, %v", stored, err)
	}
	if !bytes.Equal(stored.CurrentModelCacheHash, (Hash32{8}).Bytes()) || stored.ExecutionDType != "float16" {
		t.Fatalf("unexpected stored in-memory model: %+v", stored)
	}
	storedFiles, err := stored.FileHashes()
	if err != nil || len(storedFiles) != 1 || storedFiles[0] != (Hash32{9}) {
		t.Fatalf("unexpected stored file hashes: %v, %v", storedFiles, err)
	}

	if err := ClearNodeInMemoryModel(ctx, db, "0x1"); err != nil {
		t.Fatalf("failed to clear in-memory model: %v", err)
	}
	cleared, err := GetNodeInMemoryModel(ctx, db, "0x1")
	if err != nil || cleared != nil {
		t.Fatalf("expected no row after clear, got %v, %v", cleared, err)
	}
}
