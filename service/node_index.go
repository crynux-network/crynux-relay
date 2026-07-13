package service

import (
	"context"
	"crynux_relay/models"
	"database/sql"
	"errors"
	"math/big"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// NodeIndexEntry is one node's view in the in-memory node scheduling index.
// Entries are immutable: updates replace the whole entry, so snapshot readers
// never observe a partially updated node.
type NodeIndexEntry struct {
	Address         string
	Network         string
	Status          models.NodeStatus
	HasCurrentTask  bool
	GPUName         string
	GPUVram         uint64
	MajorVersion    uint64
	MinorVersion    uint64
	PatchVersion    uint64
	QOSScore        float64
	HealthBase      float64
	HealthUpdatedAt sql.NullTime
	StakeAmount     *big.Int
	OnDiskModelIDs  map[string]struct{}
	InUseModelIDs   map[string]struct{}
}

// scoreNode converts the entry to the minimal models.Node view consumed by
// the weight calculation helpers.
func (e *NodeIndexEntry) scoreNode() models.Node {
	return models.Node{
		Address:         e.Address,
		Network:         e.Network,
		Status:          e.Status,
		QOSScore:        e.QOSScore,
		HealthBase:      e.HealthBase,
		HealthUpdatedAt: e.HealthUpdatedAt,
		StakeAmount:     models.BigInt{Int: *new(big.Int).Set(e.StakeAmount)},
	}
}

type nodeIndexSlot struct {
	// mu serializes index entry updates for one node. The DB read inside
	// RefreshNodeIndex happens under this lock and after the owning
	// transaction commits, so the stored entry never regresses to a state
	// older than the latest committed one.
	mu    sync.Mutex
	entry atomic.Pointer[NodeIndexEntry]
}

type nodeIndex struct {
	mu    sync.RWMutex
	slots map[string]*nodeIndexSlot
}

var globalNodeIndex = &nodeIndex{
	slots: make(map[string]*nodeIndexSlot),
}

func buildNodeIndexEntry(node *models.Node) *NodeIndexEntry {
	entry := &NodeIndexEntry{
		Address:         node.Address,
		Network:         node.Network,
		Status:          node.Status,
		HasCurrentTask:  node.CurrentTaskIDCommitment.Valid,
		GPUName:         node.GPUName,
		GPUVram:         node.GPUVram,
		MajorVersion:    node.MajorVersion,
		MinorVersion:    node.MinorVersion,
		PatchVersion:    node.PatchVersion,
		QOSScore:        node.QOSScore,
		HealthBase:      node.HealthBase,
		HealthUpdatedAt: node.HealthUpdatedAt,
		StakeAmount:     new(big.Int).Set(&node.StakeAmount.Int),
		OnDiskModelIDs:  make(map[string]struct{}, len(node.Models)),
		InUseModelIDs:   make(map[string]struct{}),
	}
	for _, model := range node.Models {
		entry.OnDiskModelIDs[model.ModelID] = struct{}{}
		if model.InUse {
			entry.InUseModelIDs[model.ModelID] = struct{}{}
		}
	}
	return entry
}

func (idx *nodeIndex) getSlot(address string) *nodeIndexSlot {
	idx.mu.RLock()
	slot, ok := idx.slots[address]
	idx.mu.RUnlock()
	if ok {
		return slot
	}
	idx.mu.Lock()
	defer idx.mu.Unlock()
	slot, ok = idx.slots[address]
	if !ok {
		slot = &nodeIndexSlot{}
		idx.slots[address] = slot
	}
	return slot
}

func (idx *nodeIndex) removeSlot(address string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	delete(idx.slots, address)
}

// InitNodeIndex rebuilds the node scheduling index from the database. The
// matching scheduler must not start before the initial rebuild completes.
func InitNodeIndex(ctx context.Context, db *gorm.DB) error {
	dbCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var nodes []models.Node
	if err := db.WithContext(dbCtx).Model(&models.Node{}).
		Preload("Models").
		Where("status != ?", models.NodeStatusQuit).
		Find(&nodes).Error; err != nil {
		return err
	}

	slots := make(map[string]*nodeIndexSlot, len(nodes))
	for i := range nodes {
		slot := &nodeIndexSlot{}
		slot.entry.Store(buildNodeIndexEntry(&nodes[i]))
		slots[nodes[i].Address] = slot
	}

	globalNodeIndex.mu.Lock()
	defer globalNodeIndex.mu.Unlock()
	globalNodeIndex.slots = slots
	return nil
}

// refreshNodeIndexLocked re-reads one node from the database and replaces its
// index entry. The caller must hold the node's slot lock. Nodes that no
// longer exist or have quit are removed from the index.
func refreshNodeIndexLocked(ctx context.Context, db *gorm.DB, slot *nodeIndexSlot, address string) error {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var node models.Node
	err := db.WithContext(dbCtx).Model(&models.Node{}).
		Preload("Models").
		Where("address = ?", address).
		First(&node).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		globalNodeIndex.removeSlot(address)
		return nil
	}
	if err != nil {
		return err
	}
	if node.Status == models.NodeStatusQuit {
		globalNodeIndex.removeSlot(address)
		return nil
	}
	slot.entry.Store(buildNodeIndexEntry(&node))
	return nil
}

// LockNodeIndexByAddress acquires the per-node index lock and returns the
// unlock function. It is used by flows that must hold the lock across a
// larger section than a single transaction, such as node join. The caller
// must refresh the node's index entry with RefreshNodeIndexLocked before
// unlocking when it changed node state.
func LockNodeIndexByAddress(address string) func() {
	slot := globalNodeIndex.getSlot(address)
	slot.mu.Lock()
	return func() {
		slot.mu.Unlock()
	}
}

// RefreshNodeIndexLocked refreshes the index entry of a node whose index lock
// is already held by the caller.
func RefreshNodeIndexLocked(ctx context.Context, db *gorm.DB, address string) error {
	slot := globalNodeIndex.getSlot(address)
	return refreshNodeIndexLocked(ctx, db, slot, address)
}

// ExecuteNodeStateUpdate wraps a node state mutation so that the node
// scheduling index stays ordered with the database: it holds the per-node
// index lock of every listed node across both the database transaction inside
// fn and the index entry update, then refreshes each node's index entry from
// the committed database state. Locks are acquired in sorted address order so
// concurrent multi-node updates cannot deadlock.
//
// fn MUST NOT call ExecuteNodeStateUpdate for any of the same addresses.
//
// The index entries are refreshed even when fn returns an error, because a
// failed fn may still have committed partial node changes and a matching
// decision that lost a race requires a resync before the node is reused.
func ExecuteNodeStateUpdate(ctx context.Context, db *gorm.DB, addresses []string, fn func() error) error {
	unique := make([]string, 0, len(addresses))
	seen := make(map[string]struct{}, len(addresses))
	for _, address := range addresses {
		if address == "" {
			continue
		}
		if _, ok := seen[address]; ok {
			continue
		}
		seen[address] = struct{}{}
		unique = append(unique, address)
	}
	sort.Strings(unique)

	slots := make([]*nodeIndexSlot, len(unique))
	for i, address := range unique {
		slot := globalNodeIndex.getSlot(address)
		slot.mu.Lock()
		slots[i] = slot
	}
	defer func() {
		for i := len(slots) - 1; i >= 0; i-- {
			slots[i].mu.Unlock()
		}
	}()

	err := fn()
	for i, address := range unique {
		if refreshErr := refreshNodeIndexLocked(ctx, db, slots[i], address); refreshErr != nil {
			log.Errorf("NodeIndex: refresh node %s error: %v", address, refreshErr)
		}
	}
	return err
}

// SnapshotNodeIndex returns the current index entries. Entries are immutable
// snapshots; the returned slice is the round's node view.
func SnapshotNodeIndex() []*NodeIndexEntry {
	globalNodeIndex.mu.RLock()
	defer globalNodeIndex.mu.RUnlock()
	entries := make([]*NodeIndexEntry, 0, len(globalNodeIndex.slots))
	for _, slot := range globalNodeIndex.slots {
		if entry := slot.entry.Load(); entry != nil {
			entries = append(entries, entry)
		}
	}
	return entries
}
