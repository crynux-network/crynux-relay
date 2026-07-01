package nodes

import (
	"context"
	"crynux_relay/api/v2/response"
	"crynux_relay/models"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupDelegatedNodesTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.Node{}, &models.DelegatedStakingNodeListSnapshot{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func TestParseDelegatedNodeListFiltersValidatesInputs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	newContext := func(rawURL string) *gin.Context {
		req := httptest.NewRequest(http.MethodGet, rawURL, nil)
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = req
		return c
	}

	filters, err := parseDelegatedNodeListFilters(newContext("/nodes?status=running&status[]=stopped&gpu_vram=24,48&gpu_name=RTX%2B4090&version=1.2.3"), "gpu_vram")
	if err != nil {
		t.Fatalf("parse filters: %v", err)
	}
	if filters.SortBy != "gpu_vram" {
		t.Fatalf("unexpected sort key %s", filters.SortBy)
	}
	if !reflect.DeepEqual(filters.StatusGroups, []string{"running", "stopped"}) {
		t.Fatalf("unexpected statuses %v", filters.StatusGroups)
	}
	if !reflect.DeepEqual(filters.GPUVrams, []uint64{24, 48}) {
		t.Fatalf("unexpected GPU VRAM filters %v", filters.GPUVrams)
	}

	if _, err := parseDelegatedNodeListFilters(newContext("/nodes?status=paused"), "gpu_vram"); err == nil {
		t.Fatal("expected invalid status error")
	} else if validationErr, ok := err.(*response.ValidationErrorResponse); !ok || validationErr.GetFieldName() != "status" {
		t.Fatalf("expected status validation error, got %v", err)
	}

	if _, err := parseDelegatedNodeListFilters(newContext("/nodes"), "unknown"); err == nil {
		t.Fatal("expected invalid sort error")
	} else if validationErr, ok := err.(*response.ValidationErrorResponse); !ok || validationErr.GetFieldName() != "sort_by" {
		t.Fatalf("expected sort validation error, got %v", err)
	}
}

func TestGetDelegatedNodesSortsRunningBeforeStoppedThenMetricDesc(t *testing.T) {
	db := setupDelegatedNodesTestDB(t)
	nodes := []models.Node{
		{Address: "0xaaa", Network: "base", DelegatorShare: 10},
		{Address: "0xbbb", Network: "base", DelegatorShare: 10},
		{Address: "0xccc", Network: "base", DelegatorShare: 10},
	}
	if err := db.Create(&nodes).Error; err != nil {
		t.Fatalf("create nodes: %v", err)
	}
	snapshots := []models.DelegatedStakingNodeListSnapshot{
		{NodeAddress: "0xaaa", StatusGroup: "running", StatusRank: 0, GPUName: "RTX 4090", GPUVram: 24, Version: "1.0.0", OperatorEmission4w: models.BigInt{Int: *big.NewInt(10)}},
		{NodeAddress: "0xbbb", StatusGroup: "running", StatusRank: 0, GPUName: "RTX 5090", GPUVram: 32, Version: "1.0.1", OperatorEmission4w: models.BigInt{Int: *big.NewInt(30)}},
		{NodeAddress: "0xccc", StatusGroup: "stopped", StatusRank: 1, GPUName: "RTX 6000", GPUVram: 48, Version: "1.0.2", OperatorEmission4w: models.BigInt{Int: *big.NewInt(100)}},
	}
	if err := db.Create(&snapshots).Error; err != nil {
		t.Fatalf("create snapshots: %v", err)
	}

	res, total, err := getDelegatedNodes(context.Background(), db, &delegatedNodeListFilters{SortBy: defaultDelegatedNodeSortBy}, 0, 10)
	if err != nil {
		t.Fatalf("get delegated nodes: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected total 3, got %d", total)
	}
	addresses := []string{res[0].Address, res[1].Address, res[2].Address}
	if !reflect.DeepEqual(addresses, []string{"0xbbb", "0xaaa", "0xccc"}) {
		t.Fatalf("unexpected order %v", addresses)
	}
}

func TestGetDelegatedNodeFilterOptionsUseStakeableSnapshotsOnly(t *testing.T) {
	db := setupDelegatedNodesTestDB(t)
	snapshots := []models.DelegatedStakingNodeListSnapshot{
		{NodeAddress: "0xaaa", StatusGroup: "running", StatusRank: 0, GPUName: "RTX 4090", GPUVram: 24, Version: "1.0.0"},
		{NodeAddress: "0xbbb", StatusGroup: "stopped", StatusRank: 1, GPUName: "RTX 5090", GPUVram: 32, Version: "1.0.1"},
	}
	if err := db.Create(&snapshots).Error; err != nil {
		t.Fatalf("create snapshots: %v", err)
	}

	options, err := getDelegatedNodeFilterOptions(context.Background(), db)
	if err != nil {
		t.Fatalf("get options: %v", err)
	}
	if !reflect.DeepEqual(options.Statuses, []string{"running", "stopped"}) {
		t.Fatalf("unexpected statuses %v", options.Statuses)
	}
	if !reflect.DeepEqual(options.GPUVrams, []uint64{24, 32}) {
		t.Fatalf("unexpected GPU VRAMs %v", options.GPUVrams)
	}
	if !reflect.DeepEqual(options.GPUNames, []string{"RTX 4090", "RTX 5090"}) {
		t.Fatalf("unexpected GPU names %v", options.GPUNames)
	}
	if !reflect.DeepEqual(options.Versions, []string{"1.0.0", "1.0.1"}) {
		t.Fatalf("unexpected versions %v", options.Versions)
	}
}

func TestNodeResponseIncludesEmissionEstimateFields(t *testing.T) {
	payload, err := json.Marshal(Node{
		EstimatedUpcomingOperatorEmission:  models.BigInt{Int: *big.NewInt(1)},
		EstimatedUpcomingDelegatorEmission: models.BigInt{Int: *big.NewInt(2)},
		EmissionWeekStart:                  1767830400,
		EmissionWeekEnd:                    1768435200,
		EstimateUpdatedAt:                  1768000000,
	})
	if err != nil {
		t.Fatalf("marshal node: %v", err)
	}

	var fields map[string]interface{}
	if err := json.Unmarshal(payload, &fields); err != nil {
		t.Fatalf("unmarshal node: %v", err)
	}
	for _, field := range []string{
		"estimated_upcoming_operator_emission",
		"estimated_upcoming_delegator_emission",
		"emission_week_start",
		"emission_week_end",
		"estimate_updated_at",
	} {
		if _, ok := fields[field]; !ok {
			t.Fatalf("missing field %s in %s", field, payload)
		}
	}
}
