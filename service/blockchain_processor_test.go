package service

import (
	"context"
	"crynux_relay/blockchain"
	"crynux_relay/blockchain/bindings"
	"crynux_relay/models"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestProcessNodeStakingReceiptLogsHandlesNodeSlashed(t *testing.T) {
	nodeAddress := common.HexToAddress("0x00000000000000000000000000000000000000AA")
	receiptLog := &types.Log{}

	var slashedNode common.Address
	err := processNodeStakingReceiptLogsWithParsers([]*types.Log{receiptLog}, nodeStakingReceiptLogParsers{
		parseNodeSlashed: func(receiptLog types.Log) (*bindings.NodeStakingNodeSlashed, error) {
			return &bindings.NodeStakingNodeSlashed{NodeAddress: nodeAddress}, nil
		},
	}, nodeStakingReceiptLogHandlers{
		onNodeSlashed: func(event *bindings.NodeStakingNodeSlashed) error {
			slashedNode = event.NodeAddress
			return nil
		},
	})
	if err != nil {
		t.Fatalf("processNodeStakingReceiptLogs returned error: %v", err)
	}
	if slashedNode != nodeAddress {
		t.Fatalf("expected node slash handler to receive node %s, got %s", nodeAddress.Hex(), slashedNode.Hex())
	}
}

func TestFilterReceiptLogsByContractRoutesByAddress(t *testing.T) {
	nodeStakingAddress := common.HexToAddress("0x00000000000000000000000000000000000000AA")
	delegatedStakingAddress := common.HexToAddress("0x00000000000000000000000000000000000000BB")
	otherAddress := common.HexToAddress("0x00000000000000000000000000000000000000CC")

	nodeLog := &types.Log{Address: nodeStakingAddress}
	delegatedLog := &types.Log{Address: delegatedStakingAddress}
	otherLog := &types.Log{Address: otherAddress}

	nodeLogs, delegatedLogs := filterReceiptLogsByContract(
		[]*types.Log{nodeLog, delegatedLog, otherLog},
		nodeStakingAddress.Hex(),
		delegatedStakingAddress.Hex(),
	)

	if len(nodeLogs) != 1 || nodeLogs[0] != nodeLog {
		t.Fatalf("expected one node staking log, got %d", len(nodeLogs))
	}
	if len(delegatedLogs) != 1 || delegatedLogs[0] != delegatedLog {
		t.Fatalf("expected one delegated staking log, got %d", len(delegatedLogs))
	}
}

func TestRelayAccountDepositTransferRequiresPositiveValueAndEmptyInput(t *testing.T) {
	depositAddress := common.HexToAddress("0x00000000000000000000000000000000000000AA")
	recipientAddress := common.HexToAddress("0x00000000000000000000000000000000000000BB")

	nativeTransfer := &blockchain.TransactionTransfer{
		To:    &depositAddress,
		Value: big.NewInt(10),
	}
	if !isRelayAccountDepositTransfer(nativeTransfer, depositAddress.Hex()) {
		t.Fatal("expected positive native transfer to deposit address to be a relay account deposit")
	}

	contractCall := &blockchain.TransactionTransfer{
		To:    &depositAddress,
		Value: big.NewInt(10),
		Input: []byte{0x01},
	}
	if isRelayAccountDepositTransfer(contractCall, depositAddress.Hex()) {
		t.Fatal("expected non-empty input transaction to be skipped as a relay account deposit")
	}

	otherTransfer := &blockchain.TransactionTransfer{
		To:    &recipientAddress,
		Value: big.NewInt(10),
	}
	if isRelayAccountDepositTransfer(otherTransfer, depositAddress.Hex()) {
		t.Fatal("expected native transfer to another address to be skipped as a relay account deposit")
	}

	zeroValueTransfer := &blockchain.TransactionTransfer{
		To:    &depositAddress,
		Value: big.NewInt(0),
	}
	if isRelayAccountDepositTransfer(zeroValueTransfer, depositAddress.Hex()) {
		t.Fatal("expected zero-value transfer to be skipped as a relay account deposit")
	}
}

func setupBlockchainProcessorTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Node{},
		&models.NodeModel{},
		&models.Delegation{},
		&models.Event{},
		&models.DelegatedSlashJob{},
		&models.DelegatedStakingSlashRecord{},
		&models.NetworkNodeData{},
		&models.NodeNameCount{},
	); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX idx_network_node_data_address_test ON network_node_data(address)").Error; err != nil {
		t.Fatalf("failed to create network node data address index: %v", err)
	}
	resetBlockchainProcessorTestCaches()
	return db
}

func resetBlockchainProcessorTestCaches() {
	globalDelegationCaches = map[string]*delegationCache{
		"network-a": newTestDelegationCache(),
		"network-b": newTestDelegationCache(),
	}
	globalDelegatorShareCache = &DelegatorShareCache{delegatorShares: make(map[string]uint8)}
	globalMaxStaking = newMaxStaking()
	resetNodeNamePolicyCacheForTest()
}

func newTestDelegationCache() *delegationCache {
	return &delegationCache{
		nodeDelegations: make(map[string]map[string]*big.Int),
		userDelegations: make(map[string]map[string]*big.Int),
		userStakeAmount: make(map[string]*big.Int),
		nodeStakeAmount: make(map[string]*big.Int),
	}
}

func seedTestNode(t *testing.T, db *gorm.DB, address string, network string, status models.NodeStatus, stakeAmount int64) models.Node {
	t.Helper()
	node := models.Node{
		Address:        address,
		Network:        network,
		Status:         status,
		GPUName:        "A100",
		GPUVram:        40,
		MajorVersion:   1,
		MinorVersion:   2,
		PatchVersion:   3,
		QOSScore:       1,
		StakeAmount:    models.BigInt{Int: *big.NewInt(stakeAmount)},
		DelegatorShare: 20,
		JoinTime:       time.Now(),
	}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("failed to seed node: %v", err)
	}
	return node
}

func countEvents(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var count int64
	if err := db.Model(&models.Event{}).Count(&count).Error; err != nil {
		t.Fatalf("failed to count events: %v", err)
	}
	return count
}

func TestNodeStakedSkipsMismatchedNetwork(t *testing.T) {
	ctx := context.Background()
	db := setupBlockchainProcessorTestDB(t)
	nodeAddress := common.HexToAddress("0x00000000000000000000000000000000000000AA")
	seedTestNode(t, db, nodeAddress.Hex(), "network-a", models.NodeStatusAvailable, 10)

	err := nodeStaked(ctx, db, &bindings.NodeStakingNodeStaked{
		NodeAddress:   nodeAddress,
		StakedBalance: big.NewInt(100),
		StakedCredits: big.NewInt(20),
	}, "network-b")
	if err != nil {
		t.Fatalf("nodeStaked should skip mismatched network without error: %v", err)
	}

	var node models.Node
	if err := db.First(&node, "address = ?", nodeAddress.Hex()).Error; err != nil {
		t.Fatalf("failed to load node: %v", err)
	}
	if node.StakeAmount.Int.Cmp(big.NewInt(10)) != 0 {
		t.Fatalf("expected stake amount to remain 10, got %s", node.StakeAmount.String())
	}
	if count := countEvents(t, db); count != 0 {
		t.Fatalf("expected no events, got %d", count)
	}
	if _, ok := globalMaxStaking.stakingMap[nodeAddress.Hex()]; ok {
		t.Fatal("expected max-staking cache to remain unchanged")
	}
}

func TestDelegatorStakedSkipsMismatchedNetwork(t *testing.T) {
	ctx := context.Background()
	db := setupBlockchainProcessorTestDB(t)
	nodeAddress := common.HexToAddress("0x00000000000000000000000000000000000000AA")
	delegatorAddress := common.HexToAddress("0x00000000000000000000000000000000000000BB")
	seedTestNode(t, db, nodeAddress.Hex(), "network-a", models.NodeStatusAvailable, 10)

	err := updateDelegatedStaking(ctx, db, &bindings.DelegatedStakingDelegatorStaked{
		DelegatorAddress: delegatorAddress,
		NodeAddress:      nodeAddress,
		Amount:           big.NewInt(15),
	}, "network-b")
	if err != nil {
		t.Fatalf("updateDelegatedStaking should skip mismatched network without error: %v", err)
	}

	var count int64
	if err := db.Model(&models.Delegation{}).Count(&count).Error; err != nil {
		t.Fatalf("failed to count delegations: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no delegation rows, got %d", count)
	}
	if amount := GetNodeTotalStakeAmount(nodeAddress.Hex(), "network-b"); amount.Sign() != 0 {
		t.Fatalf("expected network-b delegation cache to remain empty, got %s", amount.String())
	}
	if count := countEvents(t, db); count != 0 {
		t.Fatalf("expected no events, got %d", count)
	}
}

func TestDelegatorStakedWithZeroShareAffectsEffectiveCache(t *testing.T) {
	ctx := context.Background()
	db := setupBlockchainProcessorTestDB(t)
	nodeAddress := common.HexToAddress("0x00000000000000000000000000000000000000AA")
	delegatorAddress := common.HexToAddress("0x00000000000000000000000000000000000000BB")
	node := seedTestNode(t, db, nodeAddress.Hex(), "network-a", models.NodeStatusAvailable, 10)
	if err := db.Model(&node).Update("delegator_share", 0).Error; err != nil {
		t.Fatalf("failed to seed zero delegator share: %v", err)
	}

	err := updateDelegatedStaking(ctx, db, &bindings.DelegatedStakingDelegatorStaked{
		DelegatorAddress: delegatorAddress,
		NodeAddress:      nodeAddress,
		Amount:           big.NewInt(15),
	}, "network-a")
	if err != nil {
		t.Fatalf("updateDelegatedStaking failed: %v", err)
	}

	var delegation models.Delegation
	if err := db.First(&delegation, "delegator_address = ? AND node_address = ? AND network = ?", delegatorAddress.Hex(), nodeAddress.Hex(), "network-a").Error; err != nil {
		t.Fatalf("failed to load delegation: %v", err)
	}
	if !delegation.Valid || delegation.Amount.Int.Cmp(big.NewInt(15)) != 0 {
		t.Fatalf("expected stored delegation amount 15 and valid=true, got amount=%s valid=%v", delegation.Amount.String(), delegation.Valid)
	}
	if amount := GetNodeTotalStakeAmount(nodeAddress.Hex(), "network-a"); amount.Cmp(big.NewInt(15)) != 0 {
		t.Fatalf("expected zero-share delegation to be cached as 15, got %s", amount.String())
	}
	if got := globalMaxStaking.stakingMap[nodeAddress.Hex()]; got == nil || got.Cmp(big.NewInt(25)) != 0 {
		t.Fatalf("expected max-staking cache entry to be total stake 25, got %v", got)
	}
}

func TestDelegatorUnstakedSkipsMismatchedNetwork(t *testing.T) {
	ctx := context.Background()
	db := setupBlockchainProcessorTestDB(t)
	nodeAddress := common.HexToAddress("0x00000000000000000000000000000000000000AA")
	delegatorAddress := common.HexToAddress("0x00000000000000000000000000000000000000BB")
	seedTestNode(t, db, nodeAddress.Hex(), "network-a", models.NodeStatusAvailable, 10)
	if err := db.Create(&models.Delegation{
		DelegatorAddress: delegatorAddress.Hex(),
		NodeAddress:      nodeAddress.Hex(),
		Amount:           models.BigInt{Int: *big.NewInt(15)},
		Valid:            true,
		Network:          "network-a",
	}).Error; err != nil {
		t.Fatalf("failed to seed delegation: %v", err)
	}
	UpdateDelegation(delegatorAddress.Hex(), nodeAddress.Hex(), big.NewInt(15), "network-a")

	err := unstakeDelegatedStaking(ctx, db, &bindings.DelegatedStakingDelegatorUnstaked{
		DelegatorAddress: delegatorAddress,
		NodeAddress:      nodeAddress,
		Amount:           big.NewInt(15),
	}, "network-b")
	if err != nil {
		t.Fatalf("unstakeDelegatedStaking should skip mismatched network without error: %v", err)
	}

	var delegation models.Delegation
	if err := db.First(&delegation, "delegator_address = ? AND node_address = ? AND network = ?", delegatorAddress.Hex(), nodeAddress.Hex(), "network-a").Error; err != nil {
		t.Fatalf("failed to load delegation: %v", err)
	}
	if !delegation.Valid {
		t.Fatal("expected existing same-network delegation to stay valid")
	}
	if amount := GetNodeTotalStakeAmount(nodeAddress.Hex(), "network-a"); amount.Cmp(big.NewInt(15)) != 0 {
		t.Fatalf("expected network-a cache to remain 15, got %s", amount.String())
	}
	if count := countEvents(t, db); count != 0 {
		t.Fatalf("expected no events, got %d", count)
	}
}

func TestChangeNodeDelegatorShareSkipsMismatchedAndUnknownNodes(t *testing.T) {
	ctx := context.Background()
	db := setupBlockchainProcessorTestDB(t)
	nodeAddress := common.HexToAddress("0x00000000000000000000000000000000000000AA")
	unknownAddress := common.HexToAddress("0x00000000000000000000000000000000000000CC")
	seedTestNode(t, db, nodeAddress.Hex(), "network-a", models.NodeStatusAvailable, 10)

	if err := changeNodeDelegatorShare(ctx, db, &bindings.DelegatedStakingNodeDelegatorShareChanged{
		NodeAddress: nodeAddress,
		Share:       0,
	}, "network-b"); err != nil {
		t.Fatalf("changeNodeDelegatorShare should skip mismatched network without error: %v", err)
	}
	if err := changeNodeDelegatorShare(ctx, db, &bindings.DelegatedStakingNodeDelegatorShareChanged{
		NodeAddress: unknownAddress,
		Share:       30,
	}, "network-a"); err != nil {
		t.Fatalf("changeNodeDelegatorShare should skip unknown node without error: %v", err)
	}

	var node models.Node
	if err := db.First(&node, "address = ?", nodeAddress.Hex()).Error; err != nil {
		t.Fatalf("failed to load node: %v", err)
	}
	if node.DelegatorShare != 20 {
		t.Fatalf("expected delegator share to remain 20, got %d", node.DelegatorShare)
	}
	if count := countEvents(t, db); count != 0 {
		t.Fatalf("expected no events, got %d", count)
	}
}

func TestChangeNodeDelegatorShareZeroKeepsEffectiveDelegation(t *testing.T) {
	ctx := context.Background()
	db := setupBlockchainProcessorTestDB(t)
	nodeAddress := common.HexToAddress("0x00000000000000000000000000000000000000AA")
	delegatorAddress := common.HexToAddress("0x00000000000000000000000000000000000000BB")
	seedTestNode(t, db, nodeAddress.Hex(), "network-a", models.NodeStatusAvailable, 10)
	if err := db.Create(&models.Delegation{
		DelegatorAddress: delegatorAddress.Hex(),
		NodeAddress:      nodeAddress.Hex(),
		Amount:           models.BigInt{Int: *big.NewInt(15)},
		Valid:            true,
		Network:          "network-a",
	}).Error; err != nil {
		t.Fatalf("failed to seed delegation: %v", err)
	}
	UpdateDelegation(delegatorAddress.Hex(), nodeAddress.Hex(), big.NewInt(15), "network-a")
	UpdateMaxStaking(nodeAddress.Hex(), big.NewInt(25))
	SetDelegatorShare(nodeAddress.Hex(), "network-a", 20)

	if err := changeNodeDelegatorShare(ctx, db, &bindings.DelegatedStakingNodeDelegatorShareChanged{
		NodeAddress: nodeAddress,
		Share:       0,
	}, "network-a"); err != nil {
		t.Fatalf("changeNodeDelegatorShare failed: %v", err)
	}

	var node models.Node
	if err := db.First(&node, "address = ? AND network = ?", nodeAddress.Hex(), "network-a").Error; err != nil {
		t.Fatalf("failed to load node: %v", err)
	}
	if node.DelegatorShare != 0 {
		t.Fatalf("expected delegator share to be 0, got %d", node.DelegatorShare)
	}
	if amount := GetNodeTotalStakeAmount(nodeAddress.Hex(), "network-a"); amount.Cmp(big.NewInt(15)) != 0 {
		t.Fatalf("expected delegation cache to remain 15, got %s", amount.String())
	}
	if share := GetDelegatorShare(nodeAddress.Hex(), "network-a"); share != 0 {
		t.Fatalf("expected delegator share cache to be 0, got %d", share)
	}
	if got := globalMaxStaking.stakingMap[nodeAddress.Hex()]; got == nil || got.Cmp(big.NewInt(25)) != 0 {
		t.Fatalf("expected max-staking cache entry to remain total stake 25, got %v", got)
	}
}

func TestDelegatorSlashedClearsMatchingNetworkDelegationForQuitNode(t *testing.T) {
	ctx := context.Background()
	db := setupBlockchainProcessorTestDB(t)
	nodeAddress := common.HexToAddress("0x00000000000000000000000000000000000000AA")
	delegatorAddress := common.HexToAddress("0x00000000000000000000000000000000000000BB")
	seedTestNode(t, db, nodeAddress.Hex(), "network-a", models.NodeStatusQuit, 0)
	if err := db.Create(&models.Delegation{
		DelegatorAddress: delegatorAddress.Hex(),
		NodeAddress:      nodeAddress.Hex(),
		Amount:           models.BigInt{Int: *big.NewInt(15)},
		Valid:            true,
		Network:          "network-a",
	}).Error; err != nil {
		t.Fatalf("failed to seed delegation: %v", err)
	}
	UpdateDelegation(delegatorAddress.Hex(), nodeAddress.Hex(), big.NewInt(15), "network-a")

	if err := slashDelegatedStaking(ctx, db, &bindings.DelegatedStakingDelegatorSlashed{
		DelegatorAddress: delegatorAddress,
		NodeAddress:      nodeAddress,
		Amount:           big.NewInt(15),
		Raw: types.Log{
			TxHash:      common.HexToHash("0x01"),
			BlockNumber: 100,
			Index:       2,
		},
	}, "network-a"); err != nil {
		t.Fatalf("slashDelegatedStaking should process matching quit node: %v", err)
	}

	var delegation models.Delegation
	if err := db.First(&delegation, "delegator_address = ? AND node_address = ? AND network = ?", delegatorAddress.Hex(), nodeAddress.Hex(), "network-a").Error; err != nil {
		t.Fatalf("failed to load delegation: %v", err)
	}
	if delegation.Valid {
		t.Fatal("expected matching-network delegation to be invalidated")
	}
	if amount := GetNodeTotalStakeAmount(nodeAddress.Hex(), "network-a"); amount.Sign() != 0 {
		t.Fatalf("expected delegation cache to be cleared, got %s", amount.String())
	}
	var auditCount int64
	if err := db.Model(&models.DelegatedStakingSlashRecord{}).Count(&auditCount).Error; err != nil {
		t.Fatalf("failed to count slash audits: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("expected one slash audit, got %d", auditCount)
	}
	if count := countEvents(t, db); count != 1 {
		t.Fatalf("expected one relay event, got %d", count)
	}
}

func TestSetNodeStatusJoinRebuildsDelegationsFromChain(t *testing.T) {
	ctx := context.Background()
	db := setupBlockchainProcessorTestDB(t)
	nodeAddress := common.HexToAddress("0x00000000000000000000000000000000000000AA")
	delegatorAddress := common.HexToAddress("0x00000000000000000000000000000000000000BB")
	staleDelegatorAddress := common.HexToAddress("0x00000000000000000000000000000000000000CC")
	node := seedTestNode(t, db, nodeAddress.Hex(), "network-b", models.NodeStatusQuit, 10)
	if err := db.Create(&models.Delegation{
		DelegatorAddress: staleDelegatorAddress.Hex(),
		NodeAddress:      nodeAddress.Hex(),
		Amount:           models.BigInt{Int: *big.NewInt(99)},
		Valid:            true,
		Network:          "network-b",
	}).Error; err != nil {
		t.Fatalf("failed to seed stale delegation: %v", err)
	}
	UpdateDelegation(staleDelegatorAddress.Hex(), nodeAddress.Hex(), big.NewInt(99), "network-b")

	originalGetStakingInfo := getStakingInfo
	originalGetNodeDelegatorShare := getNodeDelegatorShare
	originalGetNodeStakingInfos := getNodeStakingInfos
	t.Cleanup(func() {
		getStakingInfo = originalGetStakingInfo
		getNodeDelegatorShare = originalGetNodeDelegatorShare
		getNodeStakingInfos = originalGetNodeStakingInfos
	})
	getStakingInfo = func(ctx context.Context, address common.Address, network string) (bindings.NodeStakingStakingInfo, error) {
		if address != nodeAddress || network != "network-b" {
			t.Fatalf("unexpected staking info query: %s %s", address.Hex(), network)
		}
		return bindings.NodeStakingStakingInfo{
			StakedBalance: big.NewInt(10),
			StakedCredits: big.NewInt(0),
		}, nil
	}
	getNodeDelegatorShare = func(ctx context.Context, address common.Address, network string) (uint8, error) {
		if address != nodeAddress || network != "network-b" {
			t.Fatalf("unexpected delegator share query: %s %s", address.Hex(), network)
		}
		return 35, nil
	}
	getNodeStakingInfos = func(ctx context.Context, address common.Address, network string) ([]common.Address, []*big.Int, error) {
		if address != nodeAddress || network != "network-b" {
			t.Fatalf("unexpected delegation query: %s %s", address.Hex(), network)
		}
		return []common.Address{delegatorAddress}, []*big.Int{big.NewInt(12)}, nil
	}

	if err := SetNodeStatusJoin(ctx, db, &node, []string{"model-a"}); err != nil {
		t.Fatalf("SetNodeStatusJoin failed: %v", err)
	}

	var stale models.Delegation
	if err := db.First(&stale, "delegator_address = ? AND node_address = ? AND network = ?", staleDelegatorAddress.Hex(), nodeAddress.Hex(), "network-b").Error; err != nil {
		t.Fatalf("failed to load stale delegation: %v", err)
	}
	if stale.Valid {
		t.Fatal("expected stale delegation to be invalidated")
	}
	var current models.Delegation
	if err := db.First(&current, "delegator_address = ? AND node_address = ? AND network = ?", delegatorAddress.Hex(), nodeAddress.Hex(), "network-b").Error; err != nil {
		t.Fatalf("failed to load current delegation: %v", err)
	}
	if !current.Valid || current.Amount.Int.Cmp(big.NewInt(12)) != 0 {
		t.Fatalf("expected current delegation amount 12 and valid=true, got amount=%s valid=%v", current.Amount.String(), current.Valid)
	}
	if amount := GetNodeTotalStakeAmount(nodeAddress.Hex(), "network-b"); amount.Cmp(big.NewInt(12)) != 0 {
		t.Fatalf("expected delegation cache to be rebuilt to 12, got %s", amount.String())
	}
	if share := GetDelegatorShare(nodeAddress.Hex(), "network-b"); share != 35 {
		t.Fatalf("expected delegator share cache to be 35, got %d", share)
	}
	var networkNodeData models.NetworkNodeData
	if err := db.First(&networkNodeData, "address = ?", nodeAddress.Hex()).Error; err != nil {
		t.Fatalf("failed to load network node data: %v", err)
	}
	if networkNodeData.Staking.Int.Cmp(big.NewInt(22)) != 0 {
		t.Fatalf("expected network node staking to be 22, got %s", networkNodeData.Staking.String())
	}
	if got := globalMaxStaking.stakingMap[nodeAddress.Hex()]; got == nil || got.Cmp(big.NewInt(22)) != 0 {
		t.Fatalf("expected max-staking cache entry to be 22, got %v", got)
	}
}

func TestSetNodeStatusJoinWithZeroShareKeepsDelegationsInEffectiveCache(t *testing.T) {
	ctx := context.Background()
	db := setupBlockchainProcessorTestDB(t)
	nodeAddress := common.HexToAddress("0x00000000000000000000000000000000000000AA")
	delegatorAddress := common.HexToAddress("0x00000000000000000000000000000000000000BB")
	node := seedTestNode(t, db, nodeAddress.Hex(), "network-b", models.NodeStatusQuit, 10)

	originalGetStakingInfo := getStakingInfo
	originalGetNodeDelegatorShare := getNodeDelegatorShare
	originalGetNodeStakingInfos := getNodeStakingInfos
	t.Cleanup(func() {
		getStakingInfo = originalGetStakingInfo
		getNodeDelegatorShare = originalGetNodeDelegatorShare
		getNodeStakingInfos = originalGetNodeStakingInfos
	})
	getStakingInfo = func(ctx context.Context, address common.Address, network string) (bindings.NodeStakingStakingInfo, error) {
		if address != nodeAddress || network != "network-b" {
			t.Fatalf("unexpected staking info query: %s %s", address.Hex(), network)
		}
		return bindings.NodeStakingStakingInfo{
			StakedBalance: big.NewInt(10),
			StakedCredits: big.NewInt(0),
		}, nil
	}
	getNodeDelegatorShare = func(ctx context.Context, address common.Address, network string) (uint8, error) {
		if address != nodeAddress || network != "network-b" {
			t.Fatalf("unexpected delegator share query: %s %s", address.Hex(), network)
		}
		return 0, nil
	}
	getNodeStakingInfos = func(ctx context.Context, address common.Address, network string) ([]common.Address, []*big.Int, error) {
		if address != nodeAddress || network != "network-b" {
			t.Fatalf("unexpected delegation query: %s %s", address.Hex(), network)
		}
		return []common.Address{delegatorAddress}, []*big.Int{big.NewInt(12)}, nil
	}

	if err := SetNodeStatusJoin(ctx, db, &node, []string{"model-a"}); err != nil {
		t.Fatalf("SetNodeStatusJoin failed: %v", err)
	}

	var delegation models.Delegation
	if err := db.First(&delegation, "delegator_address = ? AND node_address = ? AND network = ?", delegatorAddress.Hex(), nodeAddress.Hex(), "network-b").Error; err != nil {
		t.Fatalf("failed to load delegation: %v", err)
	}
	if !delegation.Valid || delegation.Amount.Int.Cmp(big.NewInt(12)) != 0 {
		t.Fatalf("expected stored delegation amount 12 and valid=true, got amount=%s valid=%v", delegation.Amount.String(), delegation.Valid)
	}
	if amount := GetNodeTotalStakeAmount(nodeAddress.Hex(), "network-b"); amount.Cmp(big.NewInt(12)) != 0 {
		t.Fatalf("expected zero-share delegation to be cached as 12, got %s", amount.String())
	}
	if share := GetDelegatorShare(nodeAddress.Hex(), "network-b"); share != 0 {
		t.Fatalf("expected delegator share cache to be 0, got %d", share)
	}
	var networkNodeData models.NetworkNodeData
	if err := db.First(&networkNodeData, "address = ?", nodeAddress.Hex()).Error; err != nil {
		t.Fatalf("failed to load network node data: %v", err)
	}
	if networkNodeData.Staking.Int.Cmp(big.NewInt(22)) != 0 {
		t.Fatalf("expected network node staking to be 22, got %s", networkNodeData.Staking.String())
	}
	if got := globalMaxStaking.stakingMap[nodeAddress.Hex()]; got == nil || got.Cmp(big.NewInt(22)) != 0 {
		t.Fatalf("expected max-staking cache entry to be 22, got %v", got)
	}
}
