package migrations

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type resetNodeForM20260812Test struct {
	ID                      uint `gorm:"primaryKey"`
	Address                 string
	Network                 string
	CurrentTaskIDCommitment *string
}

func (resetNodeForM20260812Test) TableName() string { return "nodes" }

type nodeChildForM20260812Test struct {
	ID          uint `gorm:"primaryKey"`
	NodeAddress string
}

type nodeModelForM20260812Test nodeChildForM20260812Test

func (nodeModelForM20260812Test) TableName() string { return "node_models" }

type nodeModelSelectionForM20260812Test nodeChildForM20260812Test

func (nodeModelSelectionForM20260812Test) TableName() string {
	return "node_model_download_selections"
}

type nodeNameCountForM20260812Test struct {
	ID uint `gorm:"primaryKey"`
}

func (nodeNameCountForM20260812Test) TableName() string { return "node_name_counts" }

type networkRowForM20260812Test struct {
	ID      uint `gorm:"primaryKey"`
	Network string
}

type delegationForM20260812Test networkRowForM20260812Test

func (delegationForM20260812Test) TableName() string { return "delegations" }

type delegatedSlashJobForM20260812Test networkRowForM20260812Test

func (delegatedSlashJobForM20260812Test) TableName() string { return "delegated_slash_jobs" }

type delegatedSlashRecordForM20260812Test networkRowForM20260812Test

func (delegatedSlashRecordForM20260812Test) TableName() string {
	return "delegated_staking_slash_records"
}

type delegatedSnapshotForM20260812Test networkRowForM20260812Test

func (delegatedSnapshotForM20260812Test) TableName() string {
	return "delegated_staking_node_list_snapshots"
}

type leaderboardSnapshotForM20260812Test networkRowForM20260812Test

func (leaderboardSnapshotForM20260812Test) TableName() string {
	return "delegation_task_fee_leaderboard_snapshots"
}

type blockchainCursorForM20260812Test networkRowForM20260812Test

func (blockchainCursorForM20260812Test) TableName() string { return "blockchain_cursors" }

type inferenceTaskForM20260812Test struct {
	ID         uint `gorm:"primaryKey"`
	Commitment string
}

func (inferenceTaskForM20260812Test) TableName() string { return "inference_tasks" }

type blockchainTransactionForM20260812Test struct {
	ID     uint `gorm:"primaryKey"`
	TxHash string
}

func (blockchainTransactionForM20260812Test) TableName() string {
	return "blockchain_transactions"
}

type preservedRowForM20260812Test struct {
	ID    uint `gorm:"primaryKey"`
	Value string
}

type relayAccountForM20260812Test preservedRowForM20260812Test

func (relayAccountForM20260812Test) TableName() string { return "relay_accounts" }

type networkNodeDataForM20260812Test preservedRowForM20260812Test

func (networkNodeDataForM20260812Test) TableName() string { return "network_node_data" }

type eventForM20260812Test preservedRowForM20260812Test

func (eventForM20260812Test) TableName() string { return "events" }

func openM20260812TestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	tables := []interface{}{
		&resetNodeForM20260812Test{},
		&nodeModelForM20260812Test{},
		&nodeModelSelectionForM20260812Test{},
		&nodeNameCountForM20260812Test{},
		&delegationForM20260812Test{},
		&delegatedSlashJobForM20260812Test{},
		&delegatedSlashRecordForM20260812Test{},
		&delegatedSnapshotForM20260812Test{},
		&leaderboardSnapshotForM20260812Test{},
		&blockchainCursorForM20260812Test{},
		&inferenceTaskForM20260812Test{},
		&blockchainTransactionForM20260812Test{},
		&relayAccountForM20260812Test{},
		&networkNodeDataForM20260812Test{},
		&eventForM20260812Test{},
	}
	for _, table := range tables {
		if err := db.Migrator().CreateTable(table); err != nil {
			t.Fatalf("create %T: %v", table, err)
		}
	}
	return db
}

func mustCreateM20260812Test(t *testing.T, db *gorm.DB, value interface{}) {
	t.Helper()
	if err := db.Create(value).Error; err != nil {
		t.Fatalf("seed %T: %v", value, err)
	}
}

func countM20260812Test(t *testing.T, db *gorm.DB, model interface{}, where string, args ...interface{}) int64 {
	t.Helper()
	var count int64
	query := db.Model(model)
	if where != "" {
		query = query.Where(where, args...)
	}
	if err := query.Count(&count).Error; err != nil {
		t.Fatalf("count %T: %v", model, err)
	}
	return count
}

func TestM20260812ResetsOnlyStakingCurrentState(t *testing.T) {
	db := openM20260812TestDB(t)
	const nodeAddress = "0x0000000000000000000000000000000000000001"

	mustCreateM20260812Test(t, db, &resetNodeForM20260812Test{Address: nodeAddress, Network: stakingResetNetworkForM20260812})
	mustCreateM20260812Test(t, db, &nodeModelForM20260812Test{NodeAddress: nodeAddress})
	mustCreateM20260812Test(t, db, &nodeModelForM20260812Test{NodeAddress: "orphan"})
	mustCreateM20260812Test(t, db, &nodeModelSelectionForM20260812Test{NodeAddress: nodeAddress})
	mustCreateM20260812Test(t, db, &nodeModelSelectionForM20260812Test{NodeAddress: "orphan"})
	mustCreateM20260812Test(t, db, &nodeNameCountForM20260812Test{})
	for _, network := range []string{stakingResetNetworkForM20260812, "other-network"} {
		mustCreateM20260812Test(t, db, &delegationForM20260812Test{Network: network})
		mustCreateM20260812Test(t, db, &delegatedSlashJobForM20260812Test{Network: network})
		mustCreateM20260812Test(t, db, &delegatedSlashRecordForM20260812Test{Network: network})
		mustCreateM20260812Test(t, db, &delegatedSnapshotForM20260812Test{Network: network})
		mustCreateM20260812Test(t, db, &leaderboardSnapshotForM20260812Test{Network: network})
		mustCreateM20260812Test(t, db, &blockchainCursorForM20260812Test{Network: network})
	}
	mustCreateM20260812Test(t, db, &inferenceTaskForM20260812Test{Commitment: "task-1"})
	mustCreateM20260812Test(t, db, &blockchainTransactionForM20260812Test{TxHash: "0xtx"})
	mustCreateM20260812Test(t, db, &relayAccountForM20260812Test{Value: "account"})
	mustCreateM20260812Test(t, db, &networkNodeDataForM20260812Test{Value: "history"})
	mustCreateM20260812Test(t, db, &eventForM20260812Test{Value: "event"})

	migration := M20260812(db)
	if err := migration.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	for _, model := range []interface{}{
		&resetNodeForM20260812Test{},
		&nodeModelForM20260812Test{},
		&nodeModelSelectionForM20260812Test{},
		&nodeNameCountForM20260812Test{},
	} {
		if count := countM20260812Test(t, db, model, ""); count != 0 {
			t.Fatalf("%T has %d rows after reset", model, count)
		}
	}
	for _, model := range []interface{}{
		&delegationForM20260812Test{},
		&delegatedSlashJobForM20260812Test{},
		&delegatedSlashRecordForM20260812Test{},
		&delegatedSnapshotForM20260812Test{},
		&leaderboardSnapshotForM20260812Test{},
		&blockchainCursorForM20260812Test{},
	} {
		if count := countM20260812Test(t, db, model, "network = ?", stakingResetNetworkForM20260812); count != 0 {
			t.Fatalf("%T retained target-network rows", model)
		}
		if count := countM20260812Test(t, db, model, "network = ?", "other-network"); count != 1 {
			t.Fatalf("%T changed other-network rows: %d", model, count)
		}
	}
	for _, model := range []interface{}{
		&inferenceTaskForM20260812Test{},
		&blockchainTransactionForM20260812Test{},
		&relayAccountForM20260812Test{},
		&networkNodeDataForM20260812Test{},
		&eventForM20260812Test{},
	} {
		if count := countM20260812Test(t, db, model, ""); count != 1 {
			t.Fatalf("%T changed preserved data: %d", model, count)
		}
	}

	if err := migration.RollbackLast(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if err := migration.Migrate(); err != nil {
		t.Fatalf("reapply: %v", err)
	}
}

func TestM20260812StopsWhenNodeHasTaskCommitment(t *testing.T) {
	db := openM20260812TestDB(t)
	commitment := "task-1"
	mustCreateM20260812Test(t, db, &resetNodeForM20260812Test{
		Address:                 "0x0000000000000000000000000000000000000001",
		Network:                 stakingResetNetworkForM20260812,
		CurrentTaskIDCommitment: &commitment,
	})
	mustCreateM20260812Test(t, db, &nodeNameCountForM20260812Test{})

	if err := M20260812(db).Migrate(); err == nil {
		t.Fatal("expected migration to reject a node with a task commitment")
	}
	if count := countM20260812Test(t, db, &resetNodeForM20260812Test{}, ""); count != 1 {
		t.Fatalf("node rows changed after rejected reset: %d", count)
	}
	if count := countM20260812Test(t, db, &nodeNameCountForM20260812Test{}, ""); count != 1 {
		t.Fatalf("node name counts changed after rejected reset: %d", count)
	}
}

func TestM20260812RollsBackTheWholeResetOnFailure(t *testing.T) {
	db := openM20260812TestDB(t)
	mustCreateM20260812Test(t, db, &resetNodeForM20260812Test{
		Address: "0x0000000000000000000000000000000000000001",
		Network: stakingResetNetworkForM20260812,
	})
	mustCreateM20260812Test(t, db, &nodeNameCountForM20260812Test{})
	mustCreateM20260812Test(t, db, &delegationForM20260812Test{Network: stakingResetNetworkForM20260812})
	if err := db.Migrator().DropTable(&blockchainCursorForM20260812Test{}); err != nil {
		t.Fatalf("drop cursor table: %v", err)
	}

	if err := M20260812(db).Migrate(); err == nil {
		t.Fatal("expected migration to fail when the cursor table is missing")
	}
	for _, model := range []interface{}{
		&resetNodeForM20260812Test{},
		&nodeNameCountForM20260812Test{},
		&delegationForM20260812Test{},
	} {
		if count := countM20260812Test(t, db, model, ""); count != 1 {
			t.Fatalf("%T was not restored by transaction rollback: %d", model, count)
		}
	}
}
