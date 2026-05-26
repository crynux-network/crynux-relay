package models

import (
	"math/big"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestComputeVestingShouldReleased(t *testing.T) {
	total := big.NewInt(0).SetUint64(1000)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		now      time.Time
		duration uint
		expected string
	}{
		{
			name:     "before start",
			now:      start.Add(-time.Second),
			duration: 10,
			expected: "0",
		},
		{
			name:     "same day no release",
			now:      start.Add(6 * time.Hour),
			duration: 10,
			expected: "0",
		},
		{
			name:     "mid schedule",
			now:      start.Add(3 * 24 * time.Hour),
			duration: 10,
			expected: "300",
		},
		{
			name:     "final day and beyond",
			now:      start.Add(15 * 24 * time.Hour),
			duration: 10,
			expected: "1000",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ComputeVestingShouldReleased(total, start, tc.duration, tc.now)
			if got.String() != tc.expected {
				t.Fatalf("expected %s, got %s", tc.expected, got.String())
			}
		})
	}
}

func TestVestingReasonRoundTrip(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	record := VestingRecord{
		Model:          gorm.Model{ID: 42},
		Address:        "0xabc",
		TotalAmount:    BigInt{Int: *big.NewInt(1000)},
		ReleasedAmount: BigInt{Int: *big.NewInt(500)},
		StartTime:      start,
		DurationDays:   10,
		Source:         "airdrop",
		ExternalID:     "item-1",
		AdminSignature: "0xsig",
	}
	createdReason := BuildVestingCreatedReason(record)
	parsedCreatedID, ok := ParseVestingCreatedReason(createdReason)
	if !ok || parsedCreatedID != 42 {
		t.Fatalf("created reason parse failed: %s", createdReason)
	}
	createdPayload := BuildVestingCreatedPayload(record)
	if createdPayload.Address != record.Address ||
		createdPayload.TotalAmount != "1000" ||
		createdPayload.ReleasedAmount != "0" ||
		createdPayload.StartTime != start.Unix() ||
		createdPayload.DurationDays != record.DurationDays ||
		createdPayload.Source != record.Source ||
		createdPayload.ExternalID != record.ExternalID ||
		createdPayload.AdminSignature != record.AdminSignature {
		t.Fatalf("created payload mismatch: %#v", createdPayload)
	}

	from := big.NewInt(125)
	to := big.NewInt(300)
	releaseReason := BuildVestingReleaseReason(42, from, to)
	parsedID, parsedFrom, parsedTo, ok := ParseVestingReleaseReason(releaseReason)
	if !ok {
		t.Fatalf("release reason parse failed: %s", releaseReason)
	}
	if parsedID != 42 || parsedFrom.Cmp(from) != 0 || parsedTo.Cmp(to) != 0 {
		t.Fatalf("release reason mismatch: id=%d from=%s to=%s", parsedID, parsedFrom.String(), parsedTo.String())
	}
}
