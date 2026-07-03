package stats

import (
	"crynux_relay/api/v1/response"
	"crynux_relay/models"
	"encoding/json"
	"math/big"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDelegationEmissionLineChartDataUsesEmissionField(t *testing.T) {
	payload, err := json.Marshal(GetDelegationEmissionLineChartData{
		Timestamps: []int64{1},
		Emission:   []models.BigInt{{Int: *big.NewInt(100)}},
	})
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if _, ok := data["emission"]; !ok {
		t.Fatalf("expected emission field, got %v", data)
	}
	if _, ok := data["emission_income"]; ok {
		t.Fatalf("did not expect emission_income field, got %v", data)
	}
}

func TestGetDelegationEmissionLineChartRejectsInvalidCount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	invalidCount := 261

	_, err := GetDelegationEmissionLineChart(c, &GetDelegationEmissionLineChartInput{
		UserAddress: "0xuser",
		NodeAddress: "0xnode",
		Network:     "base",
		Count:       &invalidCount,
	})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}

	validationErr, ok := err.(*response.ValidationErrorResponse)
	if !ok {
		t.Fatalf("expected ValidationErrorResponse, got %T", err)
	}
	if validationErr.Data.FieldName != "count" {
		t.Fatalf("expected field 'count', got %s", validationErr.Data.FieldName)
	}
}
