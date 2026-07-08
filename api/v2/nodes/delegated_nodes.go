package nodes

import (
	"context"
	"crynux_relay/api/v2/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const defaultDelegatedNodeSortBy = "operator_emission_4w"

var delegatedNodeSortColumns = map[string]string{
	"operator_emission_4w":                  "operator_emission_4w",
	"delegator_emission_4w":                 "delegator_emission_4w",
	"operator_staking":                      "operator_staking",
	"delegator_staking":                     "delegator_staking",
	"total_staking":                         "total_staking",
	"delegators_num":                        "delegators_num",
	"prob_weight":                           "prob_weight",
	"qos":                                   "qos",
	"gpu_vram":                              "gpu_vram",
	"estimated_upcoming_operator_emission":  "estimated_upcoming_operator_emission",
	"estimated_upcoming_delegator_emission": "estimated_upcoming_delegator_emission",
	"delegation_apr_12m":                    "delegation_apr_12m",
	"estimated_next_10k_delegation_apr":     "estimated_next_10k_delegation_apr",
	"estimated_next_100k_delegation_apr":    "estimated_next_100k_delegation_apr",
	"estimated_next_1m_delegation_apr":      "estimated_next_1m_delegation_apr",
}

type GetDelegatedNodesInput struct {
	Page     int    `json:"page" query:"page" description:"The page" default:"1" validate:"min=1"`
	PageSize int    `json:"page_size" query:"page_size" description:"The page size" default:"30" validate:"max=100,min=1"`
	SortBy   string `json:"sort_by" query:"sort_by" description:"The sort key"`
}

type DelegatedNodesResult struct {
	Nodes []*Node `json:"nodes"`
	Total int64   `json:"total"`
}

type GetDelegatedNodesOutput struct {
	response.Response
	Data DelegatedNodesResult `json:"data"`
}

type DelegatedNodeFilterOptionsResult struct {
	Statuses []string `json:"statuses"`
	GPUVrams []uint64 `json:"gpu_vrams"`
	GPUNames []string `json:"gpu_names"`
	Versions []string `json:"versions"`
}

type GetDelegatedNodeFilterOptionsOutput struct {
	response.Response
	Data DelegatedNodeFilterOptionsResult `json:"data"`
}

type GetDelegatedNodeFilterOptionsInput struct{}

type delegatedNodeListFilters struct {
	StatusGroups []string
	GPUVrams     []uint64
	GPUNames     []string
	Versions     []string
	SortBy       string
}

type delegatedNodeListItem struct {
	Node     *models.Node
	Snapshot models.DelegatedStakingNodeListSnapshot
}

func getRepeatedQueryValues(c *gin.Context, key string) []string {
	values := c.QueryArray(key)
	bracketValues := c.QueryArray(key + "[]")
	values = append(values, bracketValues...)
	res := make([]string, 0, len(values))
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				res = append(res, part)
			}
		}
	}
	return res
}

func parseDelegatedNodeListFilters(c *gin.Context, sortBy string) (*delegatedNodeListFilters, error) {
	filters := &delegatedNodeListFilters{SortBy: sortBy}
	if filters.SortBy == "" {
		filters.SortBy = defaultDelegatedNodeSortBy
	}
	if _, ok := delegatedNodeSortColumns[filters.SortBy]; !ok {
		return nil, response.NewValidationErrorResponse("sort_by", "Invalid sort key")
	}

	for _, status := range getRepeatedQueryValues(c, "status") {
		switch status {
		case models.DelegatedStakingNodeStatusGroupRunning, models.DelegatedStakingNodeStatusGroupStopped:
			filters.StatusGroups = append(filters.StatusGroups, status)
		default:
			return nil, response.NewValidationErrorResponse("status", "Invalid status")
		}
	}
	for _, raw := range getRepeatedQueryValues(c, "gpu_vram") {
		vram, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return nil, response.NewValidationErrorResponse("gpu_vram", "Invalid GPU VRAM")
		}
		filters.GPUVrams = append(filters.GPUVrams, vram)
	}
	filters.GPUNames = getRepeatedQueryValues(c, "gpu_name")
	filters.Versions = getRepeatedQueryValues(c, "version")
	return filters, nil
}

func applyDelegatedNodeSnapshotFilters(query *gorm.DB, filters *delegatedNodeListFilters) *gorm.DB {
	if len(filters.StatusGroups) > 0 {
		query = query.Where("status_group IN ?", filters.StatusGroups)
	}
	if len(filters.GPUVrams) > 0 {
		query = query.Where("gpu_vram IN ?", filters.GPUVrams)
	}
	if len(filters.GPUNames) > 0 {
		query = query.Where("gpu_name IN ?", filters.GPUNames)
	}
	if len(filters.Versions) > 0 {
		query = query.Where("version IN ?", filters.Versions)
	}
	return query
}

func getDelegatedNodes(ctx context.Context, db *gorm.DB, filters *delegatedNodeListFilters, offset, limit int) ([]*delegatedNodeListItem, int64, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	dbi := applyDelegatedNodeSnapshotFilters(db.WithContext(dbCtx).Model(&models.DelegatedStakingNodeListSnapshot{}), filters)
	var total int64
	if err := dbi.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	sortColumn := delegatedNodeSortColumns[filters.SortBy]
	var snapshots []models.DelegatedStakingNodeListSnapshot
	if err := dbi.
		Select("node_address, delegation_apr_12m, estimated_next_10k_delegation_apr, estimated_next_100k_delegation_apr, estimated_next_1m_delegation_apr, apr_observation_days, delegation_apr_updated_at").
		Order("status_rank ASC").
		Order(sortColumn + " DESC").
		Order("node_address ASC").
		Offset(offset).
		Limit(limit).
		Find(&snapshots).Error; err != nil {
		return nil, 0, err
	}

	addresses := make([]string, 0, len(snapshots))
	for _, snapshot := range snapshots {
		addresses = append(addresses, snapshot.NodeAddress)
	}
	if len(addresses) == 0 {
		return []*delegatedNodeListItem{}, total, nil
	}

	var loadedNodes []*models.Node
	if err := db.WithContext(dbCtx).Model(&models.Node{}).Where("address IN ?", addresses).Find(&loadedNodes).Error; err != nil {
		return nil, 0, err
	}
	nodeByAddress := make(map[string]*models.Node, len(loadedNodes))
	for _, node := range loadedNodes {
		nodeByAddress[node.Address] = node
	}

	items := make([]*delegatedNodeListItem, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if node, ok := nodeByAddress[snapshot.NodeAddress]; ok {
			items = append(items, &delegatedNodeListItem{
				Node:     node,
				Snapshot: snapshot,
			})
		}
	}
	return items, total, nil
}

func GetDelegatedNodes(c *gin.Context, input *GetDelegatedNodesInput) (*GetDelegatedNodesOutput, error) {
	page := 1
	if input.Page > 0 {
		page = input.Page
	}
	pageSize := 30
	if input.PageSize > 0 {
		pageSize = input.PageSize
	}
	offset := (page - 1) * pageSize
	limit := pageSize
	filters, err := parseDelegatedNodeListFilters(c, input.SortBy)
	if err != nil {
		return nil, err
	}
	items, total, err := getDelegatedNodes(c.Request.Context(), config.GetDB(), filters, offset, limit)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	nodeDatas := make([]*Node, 0)
	results := make([]*Node, len(items))
	semaphore := make(chan struct{}, 10)
	errCh := make(chan error, len(items))
	var wg sync.WaitGroup
	for i, item := range items {
		idx := i
		listItem := item
		wg.Add(1)
		go func() {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() {
				<-semaphore
			}()
			nodeData, err := getNodeData(c.Request.Context(), listItem.Node)
			if err != nil {
				errCh <- err
				return
			}
			applyDelegationAPRSnapshot(nodeData, &listItem.Snapshot)
			results[idx] = nodeData
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return nil, response.NewExceptionResponse(err)
		}
	}
	for _, nodeData := range results {
		if nodeData != nil {
			nodeData.OperatorStaking = models.BigInt{Int: *big.NewInt(0).Add(&nodeData.OperatorStaking.Int, &nodeData.LockedEmission.Int)}
			nodeDatas = append(nodeDatas, nodeData)
		}
	}

	return &GetDelegatedNodesOutput{
		Data: DelegatedNodesResult{
			Nodes: nodeDatas,
			Total: total,
		},
	}, nil
}

func GetDelegatedNodeFilterOptions(c *gin.Context, input *GetDelegatedNodeFilterOptionsInput) (*GetDelegatedNodeFilterOptionsOutput, error) {
	options, err := getDelegatedNodeFilterOptions(c.Request.Context(), config.GetDB())
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	return &GetDelegatedNodeFilterOptionsOutput{
		Data: *options,
	}, nil
}

func getDelegatedNodeFilterOptions(ctx context.Context, db *gorm.DB) (*DelegatedNodeFilterOptionsResult, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var statuses []string
	if err := db.WithContext(dbCtx).Model(&models.DelegatedStakingNodeListSnapshot{}).Select("status_group").Group("status_group").Order("MIN(status_rank) ASC").Pluck("status_group", &statuses).Error; err != nil {
		return nil, err
	}
	var gpuVrams []uint64
	if err := db.WithContext(dbCtx).Model(&models.DelegatedStakingNodeListSnapshot{}).Select("gpu_vram").Group("gpu_vram").Order("gpu_vram ASC").Pluck("gpu_vram", &gpuVrams).Error; err != nil {
		return nil, err
	}
	var gpuNames []string
	if err := db.WithContext(dbCtx).Model(&models.DelegatedStakingNodeListSnapshot{}).Where("gpu_name <> ''").Group("gpu_name").Order("gpu_name ASC").Pluck("gpu_name", &gpuNames).Error; err != nil {
		return nil, err
	}
	var versions []string
	if err := db.WithContext(dbCtx).Model(&models.DelegatedStakingNodeListSnapshot{}).Where("version <> ''").Group("version").Order("version ASC").Pluck("version", &versions).Error; err != nil {
		return nil, err
	}
	return &DelegatedNodeFilterOptionsResult{
		Statuses: statuses,
		GPUVrams: gpuVrams,
		GPUNames: gpuNames,
		Versions: versions,
	}, nil
}
