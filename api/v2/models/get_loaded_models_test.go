package models

import (
	dbmodels "crynux_relay/models"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGetLoadedModels(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	originalGetDB := getDB
	getDB = func() *gorm.DB {
		return db
	}
	defer func() {
		getDB = originalGetDB
	}()

	if err := db.AutoMigrate(&dbmodels.LoadedModel{}); err != nil {
		t.Fatalf("failed to migrate loaded models: %v", err)
	}
	loadedModels := []dbmodels.LoadedModel{
		{ModelID: "z/model", ModelType: dbmodels.LoadedModelTypeSD, MinVRAM: 24},
		{ModelID: "a/model", ModelType: dbmodels.LoadedModelTypeLLM, MinVRAM: 16},
	}
	if err := db.Create(&loadedModels).Error; err != nil {
		t.Fatalf("failed to create loaded models: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v2/loaded-models", nil)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	resp, err := GetLoadedModels(c)
	if err != nil {
		t.Fatalf("GetLoadedModels failed: %v", err)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 loaded models, got %d", len(resp.Data))
	}
	if resp.Data[0].ModelID != "a/model" || resp.Data[0].ModelType != dbmodels.LoadedModelTypeLLM || resp.Data[0].MinVRAM != 16 {
		t.Fatalf("unexpected first loaded model: %+v", resp.Data[0])
	}
	if resp.Data[1].ModelID != "z/model" || resp.Data[1].ModelType != dbmodels.LoadedModelTypeSD || resp.Data[1].MinVRAM != 24 {
		t.Fatalf("unexpected second loaded model: %+v", resp.Data[1])
	}
}
