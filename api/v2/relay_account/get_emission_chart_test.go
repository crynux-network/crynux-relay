package relayaccount

import (
	"crynux_relay/api/v2/response"
	"crynux_relay/models"
	"encoding/json"
	"math/big"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGetEmissionChartRejectsAddressMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("user_address", "0x123")

	_, err := GetEmissionChart(c, &GetEmissionChartInput{
		Address: "0x456",
	})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}

	validationErr, ok := err.(*response.ValidationErrorResponse)
	if !ok {
		t.Fatalf("expected ValidationErrorResponse, got %T", err)
	}
	if validationErr.Data.FieldName != "address" {
		t.Fatalf("expected field 'address', got %s", validationErr.Data.FieldName)
	}
}

func TestEmissionChartDataUsesTypedFieldsOnly(t *testing.T) {
	payload, err := json.Marshal(EmissionChartData{
		Timestamps:               []int64{1},
		NodeEmissionIncome:       []models.BigInt{{Int: *big.NewInt(100)}},
		DelegationEmissionIncome: []models.BigInt{{Int: *big.NewInt(50)}},
	})
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if _, ok := data["node_emission_income"]; !ok {
		t.Fatalf("expected node_emission_income field, got %v", data)
	}
	if _, ok := data["delegation_emission_income"]; !ok {
		t.Fatalf("expected delegation_emission_income field, got %v", data)
	}
	if _, ok := data["emission_income"]; ok {
		t.Fatalf("did not expect legacy emission_income field, got %v", data)
	}
}
