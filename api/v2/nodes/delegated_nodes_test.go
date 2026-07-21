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
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var delegatedNodesTestSnapshotTime = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)

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
		{NodeAddress: "0xaaa", StatusGroup: "running", StatusRank: 0, GPUName: "RTX 4090", GPUVram: 24, Version: "1.0.0", OperatorEmission4w: models.BigInt{Int: *big.NewInt(10)}, DelegationAprUpdatedAt: delegatedNodesTestSnapshotTime},
		{NodeAddress: "0xbbb", StatusGroup: "running", StatusRank: 0, GPUName: "RTX 5090", GPUVram: 32, Version: "1.0.1", OperatorEmission4w: models.BigInt{Int: *big.NewInt(30)}, DelegationAprUpdatedAt: delegatedNodesTestSnapshotTime},
		{NodeAddress: "0xccc", StatusGroup: "stopped", StatusRank: 1, GPUName: "RTX 6000", GPUVram: 48, Version: "1.0.2", OperatorEmission4w: models.BigInt{Int: *big.NewInt(100)}, DelegationAprUpdatedAt: delegatedNodesTestSnapshotTime},
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
	addresses := []string{res[0].Node.Address, res[1].Node.Address, res[2].Node.Address}
	if !reflect.DeepEqual(addresses, []string{"0xbbb", "0xaaa", "0xccc"}) {
		t.Fatalf("unexpected order %v", addresses)
	}
}

func TestGetDelegatedNodesSortsByDelegatorEmission4wFromSnapshot(t *testing.T) {
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
		{NodeAddress: "0xaaa", StatusGroup: "running", StatusRank: 0, GPUName: "RTX 4090", GPUVram: 24, Version: "1.0.0", DelegatorEmission4w: models.BigInt{Int: *big.NewInt(10)}, DelegationAprUpdatedAt: delegatedNodesTestSnapshotTime},
		{NodeAddress: "0xbbb", StatusGroup: "running", StatusRank: 0, GPUName: "RTX 5090", GPUVram: 32, Version: "1.0.1", DelegatorEmission4w: models.BigInt{Int: *big.NewInt(30)}, DelegationAprUpdatedAt: delegatedNodesTestSnapshotTime},
		{NodeAddress: "0xccc", StatusGroup: "stopped", StatusRank: 1, GPUName: "RTX 6000", GPUVram: 48, Version: "1.0.2", DelegatorEmission4w: models.BigInt{Int: *big.NewInt(100)}, DelegationAprUpdatedAt: delegatedNodesTestSnapshotTime},
	}
	if err := db.Create(&snapshots).Error; err != nil {
		t.Fatalf("create snapshots: %v", err)
	}

	res, total, err := getDelegatedNodes(context.Background(), db, &delegatedNodeListFilters{SortBy: "delegator_emission_4w"}, 0, 10)
	if err != nil {
		t.Fatalf("get delegated nodes: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected total 3, got %d", total)
	}
	addresses := []string{res[0].Node.Address, res[1].Node.Address, res[2].Node.Address}
	if !reflect.DeepEqual(addresses, []string{"0xbbb", "0xaaa", "0xccc"}) {
		t.Fatalf("unexpected order %v", addresses)
	}
}

func TestGetDelegatedNodesSortsByDelegationAPRFromSnapshot(t *testing.T) {
	db := setupDelegatedNodesTestDB(t)
	nodes := []models.Node{
		{Address: "0xaaa", Network: "base", DelegatorShare: 10},
		{Address: "0xbbb", Network: "base", DelegatorShare: 10},
	}
	if err := db.Create(&nodes).Error; err != nil {
		t.Fatalf("create nodes: %v", err)
	}
	snapshots := []models.DelegatedStakingNodeListSnapshot{
		{NodeAddress: "0xaaa", StatusGroup: "running", StatusRank: 0, GPUName: "RTX 4090", GPUVram: 24, Version: "1.0.0", DelegationApr12m: 0.25, DelegationAprUpdatedAt: delegatedNodesTestSnapshotTime},
		{NodeAddress: "0xbbb", StatusGroup: "running", StatusRank: 0, GPUName: "RTX 5090", GPUVram: 32, Version: "1.0.1", DelegationApr12m: 0.75, AprObservationDays: 12, DelegationAprUpdatedAt: delegatedNodesTestSnapshotTime},
	}
	if err := db.Create(&snapshots).Error; err != nil {
		t.Fatalf("create snapshots: %v", err)
	}

	res, total, err := getDelegatedNodes(context.Background(), db, &delegatedNodeListFilters{SortBy: "delegation_apr_12m"}, 0, 10)
	if err != nil {
		t.Fatalf("get delegated nodes: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}
	if res[0].Node.Address != "0xbbb" {
		t.Fatalf("unexpected first node %s", res[0].Node.Address)
	}
	if res[0].Snapshot.DelegationApr12m != 0.75 || res[0].Snapshot.AprObservationDays != 12 {
		t.Fatalf("unexpected APR snapshot %+v", res[0].Snapshot)
	}
}

func TestGetDelegatedNodesSortsByEstimatedNextDelegationAPRFromSnapshot(t *testing.T) {
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
		{NodeAddress: "0xaaa", StatusGroup: "running", StatusRank: 0, GPUName: "RTX 4090", GPUVram: 24, Version: "1.0.0", EstimatedNext10kDelegationApr: 0.20, EstimatedNext100kDelegationApr: 0.80, EstimatedNext1mDelegationApr: 0.10, DelegationAprUpdatedAt: delegatedNodesTestSnapshotTime},
		{NodeAddress: "0xbbb", StatusGroup: "running", StatusRank: 0, GPUName: "RTX 5090", GPUVram: 32, Version: "1.0.1", EstimatedNext10kDelegationApr: 0.90, EstimatedNext100kDelegationApr: 0.30, EstimatedNext1mDelegationApr: 0.70, DelegationAprUpdatedAt: delegatedNodesTestSnapshotTime},
		{NodeAddress: "0xccc", StatusGroup: "stopped", StatusRank: 1, GPUName: "RTX 6000", GPUVram: 48, Version: "1.0.2", EstimatedNext10kDelegationApr: 1.00, EstimatedNext100kDelegationApr: 1.00, EstimatedNext1mDelegationApr: 1.00, DelegationAprUpdatedAt: delegatedNodesTestSnapshotTime},
	}
	if err := db.Create(&snapshots).Error; err != nil {
		t.Fatalf("create snapshots: %v", err)
	}

	cases := []struct {
		sortBy        string
		firstAddress  string
		expectedValue float64
		value         func(snapshot models.DelegatedStakingNodeListSnapshot) float64
	}{
		{
			sortBy:        "estimated_next_10k_delegation_apr",
			firstAddress:  "0xbbb",
			expectedValue: 0.90,
			value: func(snapshot models.DelegatedStakingNodeListSnapshot) float64 {
				return snapshot.EstimatedNext10kDelegationApr
			},
		},
		{
			sortBy:        "estimated_next_100k_delegation_apr",
			firstAddress:  "0xaaa",
			expectedValue: 0.80,
			value: func(snapshot models.DelegatedStakingNodeListSnapshot) float64 {
				return snapshot.EstimatedNext100kDelegationApr
			},
		},
		{
			sortBy:        "estimated_next_1m_delegation_apr",
			firstAddress:  "0xbbb",
			expectedValue: 0.70,
			value: func(snapshot models.DelegatedStakingNodeListSnapshot) float64 {
				return snapshot.EstimatedNext1mDelegationApr
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.sortBy, func(t *testing.T) {
			res, total, err := getDelegatedNodes(context.Background(), db, &delegatedNodeListFilters{SortBy: tc.sortBy}, 0, 10)
			if err != nil {
				t.Fatalf("get delegated nodes: %v", err)
			}
			if total != 3 {
				t.Fatalf("expected total 3, got %d", total)
			}
			if res[0].Node.Address != tc.firstAddress {
				t.Fatalf("unexpected first node %s", res[0].Node.Address)
			}
			if got := tc.value(res[0].Snapshot); got != tc.expectedValue {
				t.Fatalf("expected snapshot APR %f, got %f", tc.expectedValue, got)
			}
		})
	}
}

func TestGetDelegatedNodesSortsByEstimatedUpcomingDelegatorEmissionFromSnapshot(t *testing.T) {
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
		{NodeAddress: "0xaaa", StatusGroup: "running", StatusRank: 0, GPUName: "RTX 4090", GPUVram: 24, Version: "1.0.0", EstimatedUpcomingDelegatorEmission: models.BigInt{Int: *big.NewInt(20)}, DelegationAprUpdatedAt: delegatedNodesTestSnapshotTime},
		{NodeAddress: "0xbbb", StatusGroup: "running", StatusRank: 0, GPUName: "RTX 5090", GPUVram: 32, Version: "1.0.1", EstimatedUpcomingDelegatorEmission: models.BigInt{Int: *big.NewInt(80)}, DelegationAprUpdatedAt: delegatedNodesTestSnapshotTime},
		{NodeAddress: "0xccc", StatusGroup: "stopped", StatusRank: 1, GPUName: "RTX 6000", GPUVram: 48, Version: "1.0.2", EstimatedUpcomingDelegatorEmission: models.BigInt{Int: *big.NewInt(100)}, DelegationAprUpdatedAt: delegatedNodesTestSnapshotTime},
	}
	if err := db.Create(&snapshots).Error; err != nil {
		t.Fatalf("create snapshots: %v", err)
	}

	res, total, err := getDelegatedNodes(context.Background(), db, &delegatedNodeListFilters{SortBy: "estimated_upcoming_delegator_emission"}, 0, 10)
	if err != nil {
		t.Fatalf("get delegated nodes: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected total 3, got %d", total)
	}
	addresses := []string{res[0].Node.Address, res[1].Node.Address, res[2].Node.Address}
	if !reflect.DeepEqual(addresses, []string{"0xbbb", "0xaaa", "0xccc"}) {
		t.Fatalf("unexpected order %v", addresses)
	}
}

func TestGetDelegatedNodesSortsByEstimatedUpcomingOperatorEmissionFromSnapshot(t *testing.T) {
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
		{NodeAddress: "0xaaa", StatusGroup: "running", StatusRank: 0, GPUName: "RTX 4090", GPUVram: 24, Version: "1.0.0", EstimatedUpcomingOperatorEmission: models.BigInt{Int: *big.NewInt(70)}, DelegationAprUpdatedAt: delegatedNodesTestSnapshotTime},
		{NodeAddress: "0xbbb", StatusGroup: "running", StatusRank: 0, GPUName: "RTX 5090", GPUVram: 32, Version: "1.0.1", EstimatedUpcomingOperatorEmission: models.BigInt{Int: *big.NewInt(90)}, DelegationAprUpdatedAt: delegatedNodesTestSnapshotTime},
		{NodeAddress: "0xccc", StatusGroup: "stopped", StatusRank: 1, GPUName: "RTX 6000", GPUVram: 48, Version: "1.0.2", EstimatedUpcomingOperatorEmission: models.BigInt{Int: *big.NewInt(100)}, DelegationAprUpdatedAt: delegatedNodesTestSnapshotTime},
	}
	if err := db.Create(&snapshots).Error; err != nil {
		t.Fatalf("create snapshots: %v", err)
	}

	res, total, err := getDelegatedNodes(context.Background(), db, &delegatedNodeListFilters{SortBy: "estimated_upcoming_operator_emission"}, 0, 10)
	if err != nil {
		t.Fatalf("get delegated nodes: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected total 3, got %d", total)
	}
	addresses := []string{res[0].Node.Address, res[1].Node.Address, res[2].Node.Address}
	if !reflect.DeepEqual(addresses, []string{"0xbbb", "0xaaa", "0xccc"}) {
		t.Fatalf("unexpected order %v", addresses)
	}
}

func TestGetDelegatedNodeFilterOptionsUseStakeableSnapshotsOnly(t *testing.T) {
	db := setupDelegatedNodesTestDB(t)
	snapshots := []models.DelegatedStakingNodeListSnapshot{
		{NodeAddress: "0xaaa", StatusGroup: "running", StatusRank: 0, GPUName: "RTX 4090", GPUVram: 24, Version: "1.0.0", DelegationAprUpdatedAt: delegatedNodesTestSnapshotTime},
		{NodeAddress: "0xbbb", StatusGroup: "stopped", StatusRank: 1, GPUName: "RTX 5090", GPUVram: 32, Version: "1.0.1", DelegationAprUpdatedAt: delegatedNodesTestSnapshotTime},
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
		DelegationApr12m:                   0.25,
		EstimatedNext10kDelegationApr:      0.20,
		EstimatedNext100kDelegationApr:     0.15,
		EstimatedNext1mDelegationApr:       0.10,
		RelayAccountBalance:                models.BigInt{Int: *big.NewInt(3)},
		AprObservationDays:                 30,
		DelegationAprUpdatedAt:             1768000001,
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
		"delegation_apr_12m",
		"estimated_next_10k_delegation_apr",
		"estimated_next_100k_delegation_apr",
		"estimated_next_1m_delegation_apr",
		"relay_account_balance",
		"apr_observation_days",
		"delegation_apr_updated_at",
	} {
		if _, ok := fields[field]; !ok {
			t.Fatalf("missing field %s in %s", field, payload)
		}
	}
}
