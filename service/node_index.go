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
	HealthExcluded  bool
	StakeAmount     *big.Int
	// OnDiskModelFileHashes holds the exact hashes of the model files on disk.
	OnDiskModelFileHashes map[models.Hash32]struct{}
	// OnDiskModelFileSourceHashes holds the source hashes of the model files on disk.
	OnDiskModelFileSourceHashes map[models.Hash32]struct{}
	// CurrentModelCacheHash is the cache hash of the in-memory model; nil when memory is empty.
	CurrentModelCacheHash *models.Hash32
	// CurrentModelFileHashes holds the exact model files of the in-memory model.
	CurrentModelFileHashes map[models.Hash32]struct{}
	// CurrentModelFileSourceHashes holds the source hashes of the in-memory model files.
	CurrentModelFileSourceHashes map[models.Hash32]struct{}
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
		HealthExcluded:  e.HealthExcluded,
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

func buildNodeIndexEntry(node *models.Node, files []models.NodeModelFileRow, inMemory *models.NodeInMemoryModel) (*NodeIndexEntry, error) {
	entry := &NodeIndexEntry{
		Address:                      node.Address,
		Network:                      node.Network,
		Status:                       node.Status,
		HasCurrentTask:               node.CurrentTaskIDCommitment.Valid,
		GPUName:                      node.GPUName,
		GPUVram:                      node.GPUVram,
		MajorVersion:                 node.MajorVersion,
		MinorVersion:                 node.MinorVersion,
		PatchVersion:                 node.PatchVersion,
		QOSScore:                     node.QOSScore,
		HealthBase:                   node.HealthBase,
		HealthUpdatedAt:              node.HealthUpdatedAt,
		HealthExcluded:               node.HealthExcluded,
		StakeAmount:                  new(big.Int).Set(&node.StakeAmount.Int),
		OnDiskModelFileHashes:        make(map[models.Hash32]struct{}, len(files)),
		OnDiskModelFileSourceHashes:  make(map[models.Hash32]struct{}, len(files)),
		CurrentModelFileHashes:       make(map[models.Hash32]struct{}),
		CurrentModelFileSourceHashes: make(map[models.Hash32]struct{}),
	}
	sourceOf := make(map[models.Hash32]models.Hash32, len(files))
	for _, file := range files {
		hash, err := models.Hash32FromBytes(file.ModelFileHash)
		if err != nil {
			return nil, err
		}
		sourceHash, err := models.Hash32FromBytes(file.ModelFileSourceHash)
		if err != nil {
			return nil, err
		}
		entry.OnDiskModelFileHashes[hash] = struct{}{}
		entry.OnDiskModelFileSourceHashes[sourceHash] = struct{}{}
		sourceOf[hash] = sourceHash
	}
	if inMemory == nil {
		return entry, nil
	}
	cacheHash, err := models.Hash32FromBytes(inMemory.CurrentModelCacheHash)
	if err != nil {
		return nil, err
	}
	currentFiles, err := inMemory.FileHashes()
	if err != nil {
		return nil, err
	}
	entry.CurrentModelCacheHash = &cacheHash
	for _, hash := range currentFiles {
		entry.CurrentModelFileHashes[hash] = struct{}{}
		if sourceHash, ok := sourceOf[hash]; ok {
			entry.CurrentModelFileSourceHashes[sourceHash] = struct{}{}
		}
	}
	return entry, nil
}

// nodeModelState is the model inventory of nodes loaded for index building.
type nodeModelState struct {
	files    map[string][]models.NodeModelFileRow
	inMemory map[string]*models.NodeInMemoryModel
}

// loadNodeModelState loads the disk inventory and in-memory model of the given
// nodes, or of every node when no address is given.
func loadNodeModelState(ctx context.Context, db *gorm.DB, addresses ...string) (*nodeModelState, error) {
	rows, err := models.ListNodeModelFileRows(ctx, db, addresses...)
	if err != nil {
		return nil, err
	}
	state := &nodeModelState{
		files:    make(map[string][]models.NodeModelFileRow),
		inMemory: make(map[string]*models.NodeInMemoryModel),
	}
	for _, row := range rows {
		state.files[row.NodeAddress] = append(state.files[row.NodeAddress], row)
	}
	if len(addresses) == 0 {
		memoryRows, err := models.ListNodeInMemoryModels(ctx, db)
		if err != nil {
			return nil, err
		}
		for i := range memoryRows {
			state.inMemory[memoryRows[i].NodeAddress] = &memoryRows[i]
		}
		return state, nil
	}
	for _, address := range addresses {
		memory, err := models.GetNodeInMemoryModel(ctx, db, address)
		if err != nil {
			return nil, err
		}
		if memory != nil {
			state.inMemory[address] = memory
		}
	}
	return state, nil
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
		Where("status != ?", models.NodeStatusQuit).
		Find(&nodes).Error; err != nil {
		return err
	}
	state, err := loadNodeModelState(dbCtx, db)
	if err != nil {
		return err
	}

	slots := make(map[string]*nodeIndexSlot, len(nodes))
	for i := range nodes {
		entry, err := buildNodeIndexEntry(&nodes[i], state.files[nodes[i].Address], state.inMemory[nodes[i].Address])
		if err != nil {
			return err
		}
		slot := &nodeIndexSlot{}
		slot.entry.Store(entry)
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
	state, err := loadNodeModelState(dbCtx, db, address)
	if err != nil {
		return err
	}
	entry, err := buildNodeIndexEntry(&node, state.files[address], state.inMemory[address])
	if err != nil {
		return err
	}
	slot.entry.Store(entry)
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
