package admin

import (
	"context"
	"crynux_relay/models"
	"database/sql"
	"math/big"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newSlashReportTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Event{},
		&models.Node{},
		&models.BlockchainTransaction{},
		&models.DelegatedSlashJob{},
		&models.DelegatedStakingSlashRecord{},
		&models.VestingRecord{},
	); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return db
}

func createSlashReportEvent(t *testing.T, db *gorm.DB, event models.ToEventType, createdAt time.Time) models.Event {
	t.Helper()
	record, err := event.ToEvent()
	if err != nil {
		t.Fatalf("failed to build event: %v", err)
	}
	record.CreatedAt = createdAt
	if err := db.Create(record).Error; err != nil {
		t.Fatalf("failed to create event: %v", err)
	}
	return *record
}

func TestQuerySlashedNodeRecordsMapsEventTransactionAndJob(t *testing.T) {
	db := newSlashReportTestDB(t)
	ctx := context.Background()
	nodeAddress := "0x1111111111111111111111111111111111111111"

	if err := db.Create(&models.Node{
		Network: "base",
		Address: nodeAddress,
		GPUName: "RTX 4090",
	}).Error; err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	tx := models.BlockchainTransaction{
		Network:     "base",
		Type:        "slash_staking",
		Status:      models.TransactionStatusConfirmed,
		FromAddress: nodeAddress,
		ToAddress:   "0x2222222222222222222222222222222222222222",
		Value:       "0",
		TxHash:      sql.NullString{String: "0xslashtx", Valid: true},
	}
	if err := db.Create(&tx).Error; err != nil {
		t.Fatalf("failed to create transaction: %v", err)
	}

	createSlashReportEvent(t, db, &models.NodeQuitEvent{
		NodeAddress:             nodeAddress,
		BlockchainTransactionID: tx.ID,
		Network:                 "base",
	}, time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))
	slashEvent := createSlashReportEvent(t, db, &models.NodeSlashedEvent{
		NodeAddress:      nodeAddress,
		TaskIDCommitment: "0xtask",
		Amount:           models.BigInt{Int: *big.NewInt(1000)},
		Network:          "base",
	}, time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC))
	createSlashReportEvent(t, db, &models.NodeSlashedEvent{
		NodeAddress:      "0x3333333333333333333333333333333333333333",
		TaskIDCommitment: "0xother",
		Amount:           models.BigInt{Int: *big.NewInt(2000)},
		Network:          "near",
	}, time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC))

	job := models.DelegatedSlashJob{
		NodeAddress:       nodeAddress,
		Network:           "base",
		Status:            models.DelegatedSlashJobStatusCompleted,
		NodeSlashTxHash:   sql.NullString{String: "0xslashtx", Valid: true},
		NodeSlashLogIndex: sql.NullInt64{Int64: 1, Valid: true},
	}
	if err := db.Create(&job).Error; err != nil {
		t.Fatalf("failed to create slash job: %v", err)
	}

	records, total, err := querySlashedNodeRecords(ctx, db, "base", 1, 30)
	if err != nil {
		t.Fatalf("query slashed node records failed: %v", err)
	}
	if total != 1 || len(records) != 1 {
		t.Fatalf("expected one base slash record, total=%d len=%d", total, len(records))
	}

	record := records[0]
	if record.SlashEventID != slashEvent.ID {
		t.Fatalf("expected slash event id %d, got %d", slashEvent.ID, record.SlashEventID)
	}
	if record.CardName != "RTX 4090" {
		t.Fatalf("expected card name RTX 4090, got %s", record.CardName)
	}
	if record.QueuedTransactionID == nil || *record.QueuedTransactionID != tx.ID {
		t.Fatalf("expected queued transaction id %d, got %v", tx.ID, record.QueuedTransactionID)
	}
	if record.ConfirmedOperatorSlashTxHash != "0xslashtx" {
		t.Fatalf("expected tx hash 0xslashtx, got %s", record.ConfirmedOperatorSlashTxHash)
	}
	if record.DelegatedSlashJobID == nil || *record.DelegatedSlashJobID != job.ID {
		t.Fatalf("expected delegated slash job id %d, got %v", job.ID, record.DelegatedSlashJobID)
	}
}

func TestQuerySlashedNodeRecordPrefersEvidenceCardName(t *testing.T) {
	db := newSlashReportTestDB(t)
	ctx := context.Background()
	nodeAddress := "0x1111111111111111111111111111111111111111"

	if err := db.Create(&models.Node{
		Network: "base",
		Address: nodeAddress,
		GPUName: "RTX 5090 After Restart",
	}).Error; err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	slashEvent := createSlashReportEvent(t, db, &models.NodeSlashedEvent{
		NodeAddress:      nodeAddress,
		TaskIDCommitment: "0xtask",
		Amount:           models.BigInt{Int: *big.NewInt(1000)},
		Network:          "base",
		Evidence: &models.SlashEvidence{
			TaskSnapshots: []models.SlashEvidenceTaskSnapshot{
				{TaskIDCommitment: "0xtask"},
			},
			NodeSnapshots: []models.SlashEvidenceNodeSnapshot{
				{
					Address: nodeAddress,
					GPUName: "RTX 4090 At Assignment",
				},
			},
		},
	}, time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC))

	record, err := querySlashedNodeRecordByID(ctx, db, slashEvent.ID)
	if err != nil {
		t.Fatalf("query slashed node record failed: %v", err)
	}
	if record.CardName != "RTX 4090 At Assignment" {
		t.Fatalf("expected evidence card name, got %s", record.CardName)
	}
	if record.Evidence == nil {
		t.Fatal("expected evidence in slash record")
	}
}

func TestQueryPendingSlashRecordsIncludesEvidence(t *testing.T) {
	db := newSlashReportTestDB(t)
	ctx := context.Background()
	if err := db.AutoMigrate(&models.PendingSlash{}); err != nil {
		t.Fatalf("failed to migrate pending slashes: %v", err)
	}
	pendingSlash := models.PendingSlash{
		Status:           models.PendingSlashStatusPending,
		NodeAddress:      "0x1111111111111111111111111111111111111111",
		Network:          "base",
		TaskIDCommitment: "0xtask",
		EvidenceJSON:     `{"task_snapshots":[{"task_id_commitment":"0xtask"}],"node_snapshots":[{"gpu_name":"RTX 4090"}],"validation_context":{"reason":"task_end_invalidated"},"input_artifacts":[{"task_id_commitment":"0xtask","status":"missing"}],"result_artifacts":[{"task_id_commitment":"0xtask","status":"pending_upload"}]}`,
		EvidenceComplete: true,
	}
	if err := db.Create(&pendingSlash).Error; err != nil {
		t.Fatalf("failed to create pending slash: %v", err)
	}

	records, total, err := queryPendingSlashRecords(ctx, db, "pending", "base", 1, 30)
	if err != nil {
		t.Fatalf("query pending slash records failed: %v", err)
	}
	if total != 1 || len(records) != 1 {
		t.Fatalf("expected one pending slash, total=%d len=%d", total, len(records))
	}
	if records[0].Evidence == nil || len(records[0].Evidence.NodeSnapshots) != 1 || records[0].Evidence.NodeSnapshots[0].GPUName != "RTX 4090" {
		t.Fatalf("expected evidence gpu name RTX 4090, got %#v", records[0].Evidence)
	}
}

func TestFindPendingSlashArtifactAndCleanFileName(t *testing.T) {
	evidence := &models.SlashEvidence{
		ResultArtifacts: []models.SlashEvidenceArtifacts{
			{
				TaskIDCommitment: "0xtask",
				StoredPath:       "data/slashed_tasks/0xtask/results",
				Files:            []string{"0.png", "checkpoint.zip"},
				Status:           "uploaded",
			},
		},
	}

	artifact, err := findPendingSlashArtifact(evidence, "result", "0xtask")
	if err != nil {
		t.Fatalf("expected artifact lookup to succeed: %v", err)
	}
	if !artifactContainsFile(artifact, "0.png") {
		t.Fatal("expected artifact to contain 0.png")
	}
	if _, err := cleanArtifactFileName("../secret"); err == nil {
		t.Fatal("expected path traversal file name to be rejected")
	}
	if got, err := cleanArtifactFileName("checkpoint.zip"); err != nil || got != "checkpoint.zip" {
		t.Fatalf("expected clean checkpoint.zip, got %q err %v", got, err)
	}
}

func TestQueryDelegatedSlashAuditRecordsFiltersBySlashJob(t *testing.T) {
	db := newSlashReportTestDB(t)
	nodeAddress := "0x1111111111111111111111111111111111111111"
	jobID := uint(5)

	records := []models.DelegatedStakingSlashRecord{
		{
			SlashJobID:       sql.NullInt64{Int64: int64(jobID), Valid: true},
			NodeAddress:      nodeAddress,
			DelegatorAddress: "0x2222222222222222222222222222222222222222",
			Network:          "base",
			Amount:           models.BigInt{Int: *big.NewInt(100)},
			SlashTxHash:      "0xrecord1",
			BlockNumber:      11,
			LogIndex:         2,
		},
		{
			SlashJobID:       sql.NullInt64{Int64: 6, Valid: true},
			NodeAddress:      nodeAddress,
			DelegatorAddress: "0x3333333333333333333333333333333333333333",
			Network:          "base",
			Amount:           models.BigInt{Int: *big.NewInt(200)},
			SlashTxHash:      "0xrecord2",
			BlockNumber:      12,
			LogIndex:         3,
		},
	}
	if err := db.Create(&records).Error; err != nil {
		t.Fatalf("failed to create delegated slash records: %v", err)
	}

	result, total, err := queryDelegatedSlashAuditRecords(context.Background(), db, nodeAddress, "base", &jobID, 1, 30)
	if err != nil {
		t.Fatalf("query delegated slash audit records failed: %v", err)
	}
	if total != 1 || len(result) != 1 {
		t.Fatalf("expected one delegated slash record, total=%d len=%d", total, len(result))
	}
	if result[0].SlashJobID == nil || *result[0].SlashJobID != jobID {
		t.Fatalf("expected slash job id %d, got %v", jobID, result[0].SlashJobID)
	}
	if result[0].Amount != "100" {
		t.Fatalf("expected amount 100, got %s", result[0].Amount)
	}
}

func TestQuerySlashVestingRecordsReturnsAllAddressVestingsWithSlashedLockedAmount(t *testing.T) {
	db := newSlashReportTestDB(t)
	nodeAddress := "0x1111111111111111111111111111111111111111"
	now := time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC)

	records := []models.VestingRecord{
		{
			Address:        nodeAddress,
			TotalAmount:    models.BigInt{Int: *big.NewInt(1000)},
			ReleasedAmount: models.BigInt{Int: *big.NewInt(100)},
			StartTime:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			DurationDays:   10,
			Type:           models.VestingTypeNode,
			Source:         "emission",
			ExternalID:     "node-vesting",
			AdminSignature: "0xsig",
			Status:         models.VestingStatusActive,
			Slashed:        true,
		},
		{
			Address:        nodeAddress,
			TotalAmount:    models.BigInt{Int: *big.NewInt(2000)},
			ReleasedAmount: models.BigInt{Int: *big.NewInt(0)},
			StartTime:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			DurationDays:   10,
			Type:           models.VestingTypeDelegation,
			Source:         "emission",
			ExternalID:     "delegation-vesting",
			AdminSignature: "0xsig",
			Status:         models.VestingStatusActive,
		},
	}
	if err := db.Create(&records).Error; err != nil {
		t.Fatalf("failed to create vesting records: %v", err)
	}

	result, total, err := querySlashVestingRecords(context.Background(), db, nodeAddress, 1, 30, now)
	if err != nil {
		t.Fatalf("query slash vesting records failed: %v", err)
	}
	if total != 2 || len(result) != 2 {
		t.Fatalf("expected two vesting records, total=%d len=%d", total, len(result))
	}
	recordsByExternalID := make(map[string]SlashVestingRecord)
	for _, record := range result {
		recordsByExternalID[record.ExternalID] = record
	}
	nodeVesting := recordsByExternalID["node-vesting"]
	if nodeVesting.Amount != "1000" {
		t.Fatalf("expected node amount 1000, got %s", nodeVesting.Amount)
	}
	if nodeVesting.LockedAmount != "0" {
		t.Fatalf("expected slashed locked amount 0, got %s", nodeVesting.LockedAmount)
	}
	if !nodeVesting.Slashed {
		t.Fatal("expected slashed vesting record")
	}
	delegationVesting := recordsByExternalID["delegation-vesting"]
	if delegationVesting.Amount != "2000" {
		t.Fatalf("expected delegation amount 2000, got %s", delegationVesting.Amount)
	}
	if delegationVesting.LockedAmount != "1000" {
		t.Fatalf("expected delegation locked amount 1000, got %s", delegationVesting.LockedAmount)
	}
}
