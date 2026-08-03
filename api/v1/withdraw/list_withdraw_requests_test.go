package withdraw

import (
	"crynux_relay/models"
	"database/sql"
	"math/big"
	"testing"
)

func TestWithdrawalRecordResponseIncludesAuthorization(t *testing.T) {
	record := models.WithdrawRecord{
		Amount:        models.BigInt{Int: *big.NewInt(10)},
		WithdrawalFee: models.BigInt{Int: *big.NewInt(1)},
		Timestamp:     sql.NullInt64{Int64: 1234, Valid: true},
		Signature:     sql.NullString{String: "0xsigned", Valid: true},
	}

	result := withdrawalRecordResponse(record)
	if result.Timestamp != 1234 || result.Signature != "0xsigned" {
		t.Fatalf("authorization fields missing from response: %#v", result)
	}
}

func TestWithdrawalRecordResponseLeavesHistoricalAuthorizationEmpty(t *testing.T) {
	record := models.WithdrawRecord{
		Amount:        models.BigInt{Int: *big.NewInt(10)},
		WithdrawalFee: models.BigInt{Int: *big.NewInt(1)},
	}

	result := withdrawalRecordResponse(record)
	if result.Timestamp != 0 || result.Signature != "" {
		t.Fatalf("historical authorization must remain empty: %#v", result)
	}
}
