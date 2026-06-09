package relayaccount

import (
	"crynux_relay/api/v2/response"
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
