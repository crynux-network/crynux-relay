package relayaccount

import (
	"crynux_relay/models"
	"encoding/json"
	"math/big"
	"testing"
	"time"
)

func decodePayload(t *testing.T, payload string) map[string]interface{} {
	t.Helper()
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	return decoded
}

func TestGetEventPayloadDeposit(t *testing.T) {
	payload, err := buildEventPayload(models.RelayAccountEvent{
		Type:   models.RelayAccountEventTypeDeposit,
		Reason: "3-0xabc123-dymension",
	}, nil)
	if err != nil {
		t.Fatalf("failed to build payload: %v", err)
	}
	decoded := decodePayload(t, payload)
	if decoded["network"] != "dymension" || decoded["tx_hash"] != "0xabc123" {
		t.Fatalf("unexpected payload: %#v", decoded)
	}
}

func TestGetEventPayloadInvalidDepositReturnsError(t *testing.T) {
	cases := []string{
		"",
		"3-0xabc123",
		"4-0xabc123-dymension",
		"invalid",
	}

	for _, c := range cases {
		_, err := buildEventPayload(models.RelayAccountEvent{
			Type:   models.RelayAccountEventTypeDeposit,
			Reason: c,
		}, nil)
		if err == nil {
			t.Fatalf("expected error for reason %q", c)
		}
	}
}

func TestGetEventPayloadNonDepositReturnsEmpty(t *testing.T) {
	payload, err := buildEventPayload(models.RelayAccountEvent{
		Type:   models.RelayAccountEventTypeTaskPayment,
		Reason: "4-task-id",
	}, nil)
	if err != nil {
		t.Fatalf("failed to build payload: %v", err)
	}
	if payload != emptyEventPayload {
		t.Fatalf("expected empty payload for non-deposit event, got %s", payload)
	}
}

func TestBuildEventPayloadVestingCreatedUsesCreationSnapshot(t *testing.T) {
	startTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	record := models.VestingRecord{
		Address:        "0xabc",
		TotalAmount:    models.BigInt{Int: *big.NewInt(1000)},
		ReleasedAmount: models.BigInt{Int: *big.NewInt(500)},
		StartTime:      startTime,
		DurationDays:   10,
		Type:           models.VestingTypeNode,
		AdminSignature: "0xsig",
	}
	record.ID = 42
	payload, err := buildEventPayload(models.RelayAccountEvent{
		Type:   models.RelayAccountEventTypeVestingCreated,
		Reason: models.BuildVestingCreatedReason(record),
	}, map[uint]models.VestingRecord{
		record.ID: record,
	})
	if err != nil {
		t.Fatalf("failed to build payload: %v", err)
	}

	decoded := decodePayload(t, payload)
	if decoded["vesting_id"] != float64(42) || decoded["released_amount"] != "0" {
		t.Fatalf("unexpected payload: %#v", decoded)
	}
}

func TestBuildEventPayloadVestingReleaseOnlyIncludesVestingID(t *testing.T) {
	payload, err := buildEventPayload(models.RelayAccountEvent{
		Type:   models.RelayAccountEventTypeVestingRelease,
		Reason: models.BuildVestingReleaseReason(42, big.NewInt(0), big.NewInt(100)),
	}, nil)
	if err != nil {
		t.Fatalf("failed to build payload: %v", err)
	}

	decoded := decodePayload(t, payload)
	if len(decoded) != 1 || decoded["vesting_id"] != float64(42) {
		t.Fatalf("unexpected payload: %#v", decoded)
	}
}

func TestBuildEventPayloadVestingCreatedMissingRecordReturnsError(t *testing.T) {
	record := models.VestingRecord{}
	record.ID = 42
	_, err := buildEventPayload(models.RelayAccountEvent{
		Type:   models.RelayAccountEventTypeVestingCreated,
		Reason: models.BuildVestingCreatedReason(record),
	}, map[uint]models.VestingRecord{})
	if err == nil {
		t.Fatal("expected error")
	}
}
