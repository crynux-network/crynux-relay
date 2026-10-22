package service

import (
	"context"
	"crynux_relay/models"
	"errors"
	"fmt"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

var (
	ErrInvalidModelFile        = errors.New("invalid model file entry")
	ErrNodeModelStateIllegal   = errors.New("node model state cannot be updated after quit")
	errModelFileNotHeldForTask = errors.New("node does not hold the required model file")
)

// taskModelRequirements is the decoded model file requirement of a task.
type taskModelRequirements struct {
	exact     []models.Hash32
	source    []models.Hash32
	cacheHash models.Hash32
}

func newTaskModelRequirements(task *models.InferenceTask) (*taskModelRequirements, error) {
	exact, err := task.RequiredModelFileHashList()
	if err != nil {
		return nil, fmt.Errorf("task %s required model file hashes: %w", task.TaskIDCommitment, err)
	}
	source, err := task.RequiredModelFileSourceHashList()
	if err != nil {
		return nil, fmt.Errorf("task %s required model file source hashes: %w", task.TaskIDCommitment, err)
	}
	cacheHash, err := task.RequestedModelCacheHash32()
	if err != nil {
		return nil, fmt.Errorf("task %s requested model cache hash: %w", task.TaskIDCommitment, err)
	}
	return &taskModelRequirements{exact: exact, source: source, cacheHash: cacheHash}, nil
}

// modelFilesCovered reports whether the held sets contain every exact hash and
// at least one file for every source hash.
func modelFilesCovered(exactHeld, sourceHeld map[models.Hash32]struct{}, exact, source []models.Hash32) bool {
	for _, hash := range exact {
		if _, ok := exactHeld[hash]; !ok {
			return false
		}
	}
	for _, hash := range source {
		if _, ok := sourceHeld[hash]; !ok {
			return false
		}
	}
	return true
}

// holdsRequiredModelFiles is the disk hard gate of task matching.
func (e *NodeIndexEntry) holdsRequiredModelFiles(r *taskModelRequirements) bool {
	return modelFilesCovered(e.OnDiskModelFileHashes, e.OnDiskModelFileSourceHashes, r.exact, r.source)
}

// holdsRequiredModelInMemory reports whether the in-memory model is the
// requested cache entry and its files satisfy the exact and source requirements.
func (e *NodeIndexEntry) holdsRequiredModelInMemory(r *taskModelRequirements) bool {
	if e.CurrentModelCacheHash == nil || *e.CurrentModelCacheHash != r.cacheHash {
		return false
	}
	return modelFilesCovered(e.CurrentModelFileHashes, e.CurrentModelFileSourceHashes, r.exact, r.source)
}

// BuildHeldModelFiles validates the model files reported by a node and returns
// their catalog rows without duplicates. Held entries must carry a concrete revision.
func BuildHeldModelFiles(entries []models.ModelFileEntry) ([]models.ModelFile, error) {
	files := make([]models.ModelFile, 0, len(entries))
	seen := make(map[models.Hash32]struct{}, len(entries))
	for _, entry := range entries {
		entry = entry.Normalized()
		if entry.SourceType != models.ModelSourceTypeHuggingFace {
			return nil, fmt.Errorf("%w: unsupported source type %q", ErrInvalidModelFile, entry.SourceType)
		}
		if entry.Repository == "" {
			return nil, fmt.Errorf("%w: repository is empty", ErrInvalidModelFile)
		}
		if entry.Revision == "" {
			return nil, fmt.Errorf("%w: revision of %q is empty", ErrInvalidModelFile, entry.Repository)
		}
		file, err := models.NewModelFile(entry)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidModelFile, err)
		}
		hash, err := models.Hash32FromBytes(file.ModelFileHash)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[hash]; ok {
			continue
		}
		seen[hash] = struct{}{}
		files = append(files, *file)
	}
	return files, nil
}

func modelFileHashes(files []models.ModelFile) ([]models.Hash32, error) {
	hashes := make([]models.Hash32, 0, len(files))
	for _, file := range files {
		hash, err := models.Hash32FromBytes(file.ModelFileHash)
		if err != nil {
			return nil, err
		}
		hashes = append(hashes, hash)
	}
	return hashes, nil
}

// replaceNodeModelFiles makes the node disk inventory equal to files and drops
// the download selections whose required set is no longer covered.
func replaceNodeModelFiles(ctx context.Context, tx *gorm.DB, address string, files []models.ModelFile) error {
	files, err := filterWhitelistedModelFiles(files)
	if err != nil {
		return err
	}
	if err := models.UpsertModelFiles(ctx, tx, files); err != nil {
		return err
	}
	reportedHashes, err := modelFileHashes(files)
	if err != nil {
		return err
	}
	reported := make(map[models.Hash32]struct{}, len(reportedHashes))
	for _, hash := range reportedHashes {
		reported[hash] = struct{}{}
	}

	existingModels, err := models.GetNodeModelsByNodeAddress(ctx, tx, address)
	if err != nil {
		return err
	}
	existing := make(map[models.Hash32]struct{}, len(existingModels))
	absent := make([]models.Hash32, 0)
	for _, model := range existingModels {
		hash, err := models.Hash32FromBytes(model.ModelFileHash)
		if err != nil {
			return err
		}
		existing[hash] = struct{}{}
		if _, ok := reported[hash]; !ok {
			absent = append(absent, hash)
		}
	}
	if err := models.DeleteNodeModelsByFileHashes(ctx, tx, address, absent); err != nil {
		return err
	}

	missing := make([]models.NodeModel, 0)
	for _, hash := range reportedHashes {
		if _, ok := existing[hash]; !ok {
			missing = append(missing, models.NewNodeModel(address, hash))
		}
	}
	if len(missing) > 0 {
		if err := models.CreateNodeModels(ctx, tx, missing); err != nil {
			return err
		}
	}

	sourceHashes := make([]models.Hash32, 0, len(files))
	for _, file := range files {
		sourceHash, err := models.Hash32FromBytes(file.ModelFileSourceHash)
		if err != nil {
			return err
		}
		sourceHashes = append(sourceHashes, sourceHash)
	}
	return pruneUncoveredSelections(ctx, tx, address, reported, hashSet(sourceHashes))
}

func hashSet(hashes []models.Hash32) map[models.Hash32]struct{} {
	set := make(map[models.Hash32]struct{}, len(hashes))
	for _, hash := range hashes {
		set[hash] = struct{}{}
	}
	return set
}

// pruneUncoveredSelections deletes the node's selection records whose required
// model file set is not covered by the reported inventory.
func pruneUncoveredSelections(ctx context.Context, tx *gorm.DB, address string, exactHeld, sourceHeld map[models.Hash32]struct{}) error {
	selections, err := models.GetNodeModelDownloadSelectionsByNodeAddress(ctx, tx, address)
	if err != nil {
		return err
	}
	uncovered := make([]uint, 0)
	for i := range selections {
		exact, source, err := selections[i].RequiredHashes()
		if err != nil {
			return err
		}
		if !modelFilesCovered(exactHeld, sourceHeld, exact, source) {
			uncovered = append(uncovered, selections[i].ID)
		}
	}
	return models.DeleteNodeModelDownloadSelectionsByIDs(ctx, tx, uncovered)
}

// AddNodeModelFile records one downloaded model file in the node disk inventory.
func AddNodeModelFile(ctx context.Context, db *gorm.DB, address string, file models.ModelFile) error {
	rejected, err := modelFileRejectedByWhitelist(file)
	if err != nil {
		return err
	}
	if rejected {
		return nil
	}
	hash, err := models.Hash32FromBytes(file.ModelFileHash)
	if err != nil {
		return err
	}
	return ExecuteNodeStateUpdate(ctx, db, []string{address}, func() error {
		return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			node, err := models.GetNodeByAddress(ctx, tx, address)
			if err != nil {
				return err
			}
			if node.Status == models.NodeStatusQuit {
				return ErrNodeModelStateIllegal
			}
			if err := models.UpsertModelFiles(ctx, tx, []models.ModelFile{file}); err != nil {
				return err
			}
			_, err = models.GetNodeModel(ctx, tx, address, hash)
			if errors.Is(err, gorm.ErrRecordNotFound) {
				nodeModel := models.NewNodeModel(address, hash)
				return nodeModel.Save(ctx, tx)
			}
			return err
		})
	})
}

// NodeInMemoryModelReport is the model a node currently holds in memory.
type NodeInMemoryModelReport struct {
	CacheHash      models.Hash32
	Files          []models.ModelFile
	CacheConfig    []byte
	ExecutionDType string
}

// SetNodeInMemoryModel replaces the in-memory model of a node. A nil report
// marks the node memory as empty.
func SetNodeInMemoryModel(ctx context.Context, db *gorm.DB, address string, report *NodeInMemoryModelReport) error {
	if report != nil {
		if err := assertWhitelistedModelFiles(report.Files); err != nil {
			return err
		}
	}
	return ExecuteNodeStateUpdate(ctx, db, []string{address}, func() error {
		return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			node, err := models.GetNodeByAddress(ctx, tx, address)
			if err != nil {
				return err
			}
			if node.Status == models.NodeStatusQuit {
				return ErrNodeModelStateIllegal
			}
			if report == nil {
				return models.ClearNodeInMemoryModel(ctx, tx, address)
			}
			if err := models.UpsertModelFiles(ctx, tx, report.Files); err != nil {
				return err
			}
			hashes, err := modelFileHashes(report.Files)
			if err != nil {
				return err
			}
			row := models.NewNodeInMemoryModel(address, report.CacheHash, hashes, report.CacheConfig, report.ExecutionDType)
			return models.SaveNodeInMemoryModel(ctx, tx, &row)
		})
	})
}

// clearNodeModelState removes the disk inventory, the in-memory model and the
// download selections of a node.
func clearNodeModelState(ctx context.Context, tx *gorm.DB, address string) error {
	if err := models.DeleteNodeModelsByNodeAddress(ctx, tx, address); err != nil {
		return err
	}
	if err := models.ClearNodeInMemoryModel(ctx, tx, address); err != nil {
		return err
	}
	return models.DeleteNodeModelDownloadSelectionsByNodeAddress(ctx, tx, address)
}

// resolveExecutionModelFileHashes picks the exact model files a node executes a
// task with. Exact requirements use their own hash. A source requirement uses
// the in-memory revision when the node holds it in memory, otherwise the
// smallest held file hash of the source.
func resolveExecutionModelFileHashes(r *taskModelRequirements, files []models.NodeModelFileRow, current map[models.Hash32]struct{}) ([]models.Hash32, error) {
	held := make(map[models.Hash32]struct{}, len(files))
	bySource := make(map[models.Hash32][]models.Hash32)
	for _, file := range files {
		hash, err := models.Hash32FromBytes(file.ModelFileHash)
		if err != nil {
			return nil, err
		}
		sourceHash, err := models.Hash32FromBytes(file.ModelFileSourceHash)
		if err != nil {
			return nil, err
		}
		held[hash] = struct{}{}
		bySource[sourceHash] = append(bySource[sourceHash], hash)
	}

	resolved := make([]models.Hash32, 0, len(r.exact)+len(r.source))
	for _, hash := range r.exact {
		if _, ok := held[hash]; !ok {
			return nil, fmt.Errorf("%w: exact file %x", errModelFileNotHeldForTask, hash)
		}
		resolved = append(resolved, hash)
	}
	for _, sourceHash := range r.source {
		candidates := models.SortedUniqueHashes(bySource[sourceHash])
		if len(candidates) == 0 {
			return nil, fmt.Errorf("%w: source %x", errModelFileNotHeldForTask, sourceHash)
		}
		selected := candidates[0]
		for _, candidate := range candidates {
			if _, ok := current[candidate]; ok {
				selected = candidate
				break
			}
		}
		resolved = append(resolved, selected)
	}
	return models.SortedUniqueHashes(resolved), nil
}

// LoadModelFileEntries returns the exact model file entries of the hashes from
// the model_files catalog.
func LoadModelFileEntries(ctx context.Context, db *gorm.DB, hashes []models.Hash32) (map[models.Hash32]models.ModelFileEntry, error) {
	files, err := models.GetModelFiles(ctx, db, hashes)
	if err != nil {
		return nil, err
	}
	entries := make(map[models.Hash32]models.ModelFileEntry, len(files))
	for i := range files {
		hash, err := models.Hash32FromBytes(files[i].ModelFileHash)
		if err != nil {
			return nil, err
		}
		entry, err := files[i].Entry()
		if err != nil {
			return nil, err
		}
		entries[hash] = entry
	}
	return entries, nil
}

// TaskExecutionModelFileEntries returns the exact model file entries frozen for
// the execution of a task, ordered by file hash.
func TaskExecutionModelFileEntries(ctx context.Context, db *gorm.DB, task *models.InferenceTask) ([]models.ModelFileEntry, error) {
	hashes, err := task.ExecutionModelFileHashList()
	if err != nil {
		return nil, err
	}
	if len(hashes) == 0 {
		return []models.ModelFileEntry{}, nil
	}
	entries, err := LoadModelFileEntries(ctx, db, hashes)
	if err != nil {
		return nil, err
	}
	return orderedModelFileEntries(task.TaskIDCommitment, hashes, entries), nil
}

func orderedModelFileEntries(taskIDCommitment string, hashes []models.Hash32, entries map[models.Hash32]models.ModelFileEntry) []models.ModelFileEntry {
	ordered := make([]models.ModelFileEntry, 0, len(hashes))
	for _, hash := range hashes {
		entry, ok := entries[hash]
		if !ok {
			log.Errorf("TaskExecutionModelFileEntries: model file %x of task %s is missing from the catalog", hash, taskIDCommitment)
			continue
		}
		ordered = append(ordered, entry)
	}
	return ordered
}

// BatchTaskExecutionModelFileEntries returns the execution model file entries
// of every task keyed by task id commitment using one catalog query.
func BatchTaskExecutionModelFileEntries(ctx context.Context, db *gorm.DB, tasks []models.InferenceTask) (map[string][]models.ModelFileEntry, error) {
	hashesByTask := make(map[string][]models.Hash32, len(tasks))
	all := make([]models.Hash32, 0)
	for i := range tasks {
		hashes, err := tasks[i].ExecutionModelFileHashList()
		if err != nil {
			return nil, err
		}
		if len(hashes) == 0 {
			continue
		}
		hashesByTask[tasks[i].TaskIDCommitment] = hashes
		all = append(all, hashes...)
	}
	entries, err := LoadModelFileEntries(ctx, db, models.SortedUniqueHashes(all))
	if err != nil {
		return nil, err
	}
	result := make(map[string][]models.ModelFileEntry, len(hashesByTask))
	for commitment, hashes := range hashesByTask {
		result[commitment] = orderedModelFileEntries(commitment, hashes, entries)
	}
	return result, nil
}

// NodeModelFileEntries returns the model file entries a node holds on disk and
// the entries of the model it holds in memory.
func NodeModelFileEntries(ctx context.Context, db *gorm.DB, address string) (onDisk, inMemory []models.ModelFileEntry, err error) {
	nodeModels, err := models.GetNodeModelsByNodeAddress(ctx, db, address)
	if err != nil {
		return nil, nil, err
	}
	memory, err := models.GetNodeInMemoryModel(ctx, db, address)
	if err != nil {
		return nil, nil, err
	}

	diskHashes := make([]models.Hash32, 0, len(nodeModels))
	for _, nodeModel := range nodeModels {
		hash, err := models.Hash32FromBytes(nodeModel.ModelFileHash)
		if err != nil {
			return nil, nil, err
		}
		diskHashes = append(diskHashes, hash)
	}
	var memoryHashes []models.Hash32
	if memory != nil {
		memoryHashes, err = memory.FileHashes()
		if err != nil {
			return nil, nil, err
		}
	}
	entries, err := LoadModelFileEntries(ctx, db, models.SortedUniqueHashes(append(append([]models.Hash32{}, diskHashes...), memoryHashes...)))
	if err != nil {
		return nil, nil, err
	}
	onDisk = orderedModelFileEntries(address, diskHashes, entries)
	inMemory = orderedModelFileEntries(address, memoryHashes, entries)
	return onDisk, inMemory, nil
}
