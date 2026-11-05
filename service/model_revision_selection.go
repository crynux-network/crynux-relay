package service

import (
	"bytes"
	"context"
	"crynux_relay/models"
	"encoding/json"
	"sort"

	"gorm.io/gorm"
)

type networkRevisionCandidate struct {
	entry       models.ModelFileEntry
	hash        models.Hash32
	diskNodes   map[string]struct{}
	memoryNodes map[string]struct{}
}

// SelectNetworkModelFileRevision chooses one exact commit already held by
// Available or Busy nodes for the source identity of entry. Loaded revisions
// are preferred; among the same tier, the revision held by more online nodes
// wins; ties use the exact model file hash.
func SelectNetworkModelFileRevision(
	ctx context.Context,
	db *gorm.DB,
	entry models.ModelFileEntry,
) (models.ModelFileEntry, bool, error) {
	sourceHash, err := entry.ModelFileSourceHash()
	if err != nil {
		return models.ModelFileEntry{}, false, err
	}
	rows, err := models.ListOnlineNodeModelFilesBySourceHash(ctx, db, sourceHash)
	if err != nil {
		return models.ModelFileEntry{}, false, err
	}
	if len(rows) == 0 {
		return models.ModelFileEntry{}, false, nil
	}

	byHash := make(map[models.Hash32]*networkRevisionCandidate)
	addressSet := make(map[string]struct{})
	addresses := make([]string, 0)
	for _, row := range rows {
		var catalogEntry models.ModelFileEntry
		if err := json.Unmarshal(row.CanonicalJSON, &catalogEntry); err != nil {
			return models.ModelFileEntry{}, false, err
		}
		if !models.IsCommitSHA(catalogEntry.Revision) {
			continue
		}
		hash, err := models.Hash32FromBytes(row.ModelFileHash)
		if err != nil {
			return models.ModelFileEntry{}, false, err
		}
		candidate, ok := byHash[hash]
		if !ok {
			candidate = &networkRevisionCandidate{
				entry:       catalogEntry,
				hash:        hash,
				diskNodes:   make(map[string]struct{}),
				memoryNodes: make(map[string]struct{}),
			}
			byHash[hash] = candidate
		}
		candidate.diskNodes[row.NodeAddress] = struct{}{}
		if _, seen := addressSet[row.NodeAddress]; !seen {
			addressSet[row.NodeAddress] = struct{}{}
			addresses = append(addresses, row.NodeAddress)
		}
	}
	if len(byHash) == 0 {
		return models.ModelFileEntry{}, false, nil
	}

	memoryRows, err := models.ListNodeInMemoryModelsByAddresses(ctx, db, addresses)
	if err != nil {
		return models.ModelFileEntry{}, false, err
	}
	for i := range memoryRows {
		hashes, err := memoryRows[i].FileHashes()
		if err != nil {
			return models.ModelFileEntry{}, false, err
		}
		for _, hash := range hashes {
			if candidate, ok := byHash[hash]; ok {
				candidate.memoryNodes[memoryRows[i].NodeAddress] = struct{}{}
			}
		}
	}

	candidates := make([]networkRevisionCandidate, 0, len(byHash))
	for _, candidate := range byHash {
		candidates = append(candidates, *candidate)
	}
	sort.Slice(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		aMem, bMem := len(a.memoryNodes), len(b.memoryNodes)
		aLoaded := aMem > 0
		bLoaded := bMem > 0
		if aLoaded != bLoaded {
			return aLoaded
		}
		if aLoaded {
			if aMem != bMem {
				return aMem > bMem
			}
		} else {
			aDisk, bDisk := len(a.diskNodes), len(b.diskNodes)
			if aDisk != bDisk {
				return aDisk > bDisk
			}
		}
		return bytes.Compare(a.hash[:], b.hash[:]) < 0
	})
	selected := candidates[0].entry
	selected.Revision = models.NormalizeCommitRevision(selected.Revision)
	return selected, true, nil
}
