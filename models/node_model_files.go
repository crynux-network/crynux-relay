package models

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// NodeModelFileRow is one model file held on a node's disk together with its
// source hash from the model_files catalog.
type NodeModelFileRow struct {
	NodeAddress         string
	ModelFileHash       []byte
	ModelFileSourceHash []byte
}

// ListNodeModelFileRows returns the disk inventory of the given nodes in
// deterministic order. An empty address list returns the inventory of every node.
func ListNodeModelFileRows(ctx context.Context, db *gorm.DB, nodeAddresses ...string) ([]NodeModelFileRow, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	query := db.WithContext(dbCtx).Model(&NodeModel{}).
		Select("node_models.node_address AS node_address, node_models.model_file_hash AS model_file_hash, model_files.model_file_source_hash AS model_file_source_hash").
		Joins("INNER JOIN model_files ON model_files.model_file_hash = node_models.model_file_hash").
		Order("node_models.node_address, node_models.model_file_hash")
	if len(nodeAddresses) > 0 {
		query = query.Where("node_models.node_address IN ?", nodeAddresses)
	}
	var rows []NodeModelFileRow
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// NodeModelSourceExists reports whether any node holds a model file with the source hash.
func NodeModelSourceExists(ctx context.Context, db *gorm.DB, sourceHash Hash32) (bool, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var count int64
	err := db.WithContext(dbCtx).Model(&NodeModel{}).
		Joins("INNER JOIN model_files ON model_files.model_file_hash = node_models.model_file_hash").
		Where("model_files.model_file_source_hash = ?", sourceHash.Bytes()).
		Limit(1).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// DeleteNodeModelsByNodeAddress removes the whole disk inventory of a node.
func DeleteNodeModelsByNodeAddress(ctx context.Context, db *gorm.DB, nodeAddress string) error {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return db.WithContext(dbCtx).Where("node_address = ?", nodeAddress).Delete(&NodeModel{}).Error
}

// DeleteNodeModelsByFileHashes removes the given files from a node's disk inventory.
func DeleteNodeModelsByFileHashes(ctx context.Context, db *gorm.DB, nodeAddress string, hashes []Hash32) error {
	if len(hashes) == 0 {
		return nil
	}
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return db.WithContext(dbCtx).
		Where("node_address = ?", nodeAddress).
		Where("model_file_hash IN ?", hashesToBytes(hashes)).
		Delete(&NodeModel{}).Error
}

// ListNodeInMemoryModels returns the in-memory model rows of every node.
func ListNodeInMemoryModels(ctx context.Context, db *gorm.DB) ([]NodeInMemoryModel, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var rows []NodeInMemoryModel
	if err := db.WithContext(dbCtx).Order("node_address").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ModelRepositoryNodeCount holds the distinct node counts of one model repository.
type ModelRepositoryNodeCount struct {
	OnDisk   int64
	InMemory int64
}

// CountNodesByModelRepository returns, per model repository, the number of
// distinct nodes that hold any file of the repository on disk and the number
// of distinct nodes that currently hold any file of the repository in memory.
func CountNodesByModelRepository(ctx context.Context, db *gorm.DB) (map[string]ModelRepositoryNodeCount, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	type diskRow struct {
		NodeAddress   string
		CanonicalJSON JSONText
	}
	var diskRows []diskRow
	if err := db.WithContext(dbCtx).Model(&NodeModel{}).
		Select("node_models.node_address AS node_address, model_files.canonical_json AS canonical_json").
		Joins("INNER JOIN model_files ON model_files.model_file_hash = node_models.model_file_hash").
		Find(&diskRows).Error; err != nil {
		return nil, err
	}
	onDisk := make(map[string]map[string]struct{})
	for _, row := range diskRows {
		repository, err := repositoryOfCanonicalJSON(row.CanonicalJSON)
		if err != nil {
			return nil, err
		}
		addNodeToRepository(onDisk, repository, row.NodeAddress)
	}

	inMemoryRows, err := ListNodeInMemoryModels(ctx, db)
	if err != nil {
		return nil, err
	}
	inMemory := make(map[string]map[string]struct{})
	if len(inMemoryRows) > 0 {
		hashSet := make(map[Hash32]struct{})
		for i := range inMemoryRows {
			hashes, err := inMemoryRows[i].FileHashes()
			if err != nil {
				return nil, err
			}
			for _, hash := range hashes {
				hashSet[hash] = struct{}{}
			}
		}
		hashes := make([]Hash32, 0, len(hashSet))
		for hash := range hashSet {
			hashes = append(hashes, hash)
		}
		files, err := GetModelFiles(ctx, db, hashes)
		if err != nil {
			return nil, err
		}
		repositoryByHash := make(map[Hash32]string, len(files))
		for _, file := range files {
			hash, err := Hash32FromBytes(file.ModelFileHash)
			if err != nil {
				return nil, err
			}
			repository, err := repositoryOfCanonicalJSON(file.CanonicalJSON)
			if err != nil {
				return nil, err
			}
			repositoryByHash[hash] = repository
		}
		for i := range inMemoryRows {
			hashes, err := inMemoryRows[i].FileHashes()
			if err != nil {
				return nil, err
			}
			for _, hash := range hashes {
				if repository, ok := repositoryByHash[hash]; ok {
					addNodeToRepository(inMemory, repository, inMemoryRows[i].NodeAddress)
				}
			}
		}
	}

	counts := make(map[string]ModelRepositoryNodeCount, len(onDisk))
	for repository, nodes := range onDisk {
		counts[repository] = ModelRepositoryNodeCount{OnDisk: int64(len(nodes)), InMemory: int64(len(inMemory[repository]))}
	}
	return counts, nil
}

func addNodeToRepository(target map[string]map[string]struct{}, repository, nodeAddress string) {
	nodes, ok := target[repository]
	if !ok {
		nodes = make(map[string]struct{})
		target[repository] = nodes
	}
	nodes[nodeAddress] = struct{}{}
}

func repositoryOfCanonicalJSON(data []byte) (string, error) {
	var entry struct {
		Repository string `json:"repository"`
	}
	if err := json.Unmarshal(data, &entry); err != nil {
		return "", fmt.Errorf("decode model file canonical json: %w", err)
	}
	return entry.Repository, nil
}

// GetNodeModelDownloadSelectionsByNodeAddress returns every download selection of a node.
func GetNodeModelDownloadSelectionsByNodeAddress(ctx context.Context, db *gorm.DB, nodeAddress string) ([]NodeModelDownloadSelection, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var selections []NodeModelDownloadSelection
	if err := db.WithContext(dbCtx).Where("node_address = ?", nodeAddress).Find(&selections).Error; err != nil {
		return nil, err
	}
	return selections, nil
}

// DeleteNodeModelDownloadSelectionsByIDs deletes selection records by primary key.
func DeleteNodeModelDownloadSelectionsByIDs(ctx context.Context, db *gorm.DB, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return db.WithContext(dbCtx).Where("id IN ?", ids).Delete(&NodeModelDownloadSelection{}).Error
}
