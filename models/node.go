package models

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"gorm.io/gorm"
)

type NodeStatus uint8

var ErrNodeStatusChanged = errors.New("Node status changed during update")

const (
	NodeStatusQuit = iota
	NodeStatusAvailable
	NodeStatusBusy
	NodeStatusPendingPause
	NodeStatusPendingQuit
	NodeStatusPaused
)

type Node struct {
	gorm.Model
	Network                 string         `json:"network" gorm:"index"`
	Address                 string         `json:"address" gorm:"index"`
	Status                  NodeStatus     `json:"status" gorm:"index"`
	GPUName                 string         `json:"gpu_name" gorm:"index"`
	GPUVram                 uint64         `json:"gpu_vram" gorm:"index"`
	QOSScore                float64        `json:"qos_score"`
	MajorVersion            uint64         `json:"major_version"`
	MinorVersion            uint64         `json:"minor_version"`
	PatchVersion            uint64         `json:"patch_version"`
	JoinTime                time.Time      `json:"join_time"`
	StakeAmount             BigInt         `json:"stake_amount"`
	HealthBase              float64        `json:"health_base" gorm:"default:1.0"`
	HealthUpdatedAt         sql.NullTime   `json:"health_updated_at" gorm:"null;default:null"`
	DelegatorShare          uint8          `json:"delegator_share"`
	CurrentTaskIDCommitment sql.NullString `json:"current_task_id_commitment" gorm:"null;default:null"`
	LastSeenTime            sql.NullTime   `json:"last_seen_time" gorm:"null;default:null"`
	CurrentTask             InferenceTask  `json:"-" gorm:"foreignKey:TaskIDCommitment;references:CurrentTaskIDCommitment"`
	Models                  []NodeModel    `json:"-" gorm:"foreignKey:NodeAddress;references:Address"`
	TaskFee                 TaskFee        `json:"-" gorm:"foreignKey:Address;references:Address"`
}

func (node *Node) Save(ctx context.Context, db *gorm.DB) error {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.WithContext(dbCtx).Save(&node).Error; err != nil {
		return err
	}
	return nil
}

func (node *Node) Update(ctx context.Context, db *gorm.DB, values map[string]interface{}) error {
	if node.ID == 0 {
		return errors.New("Node.ID cannot be 0 when update")
	}
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var result *gorm.DB
	if _, ok := values["status"]; ok {
		result = db.WithContext(dbCtx).Model(node).Where("status = ?", node.Status).Updates(values)
		if result.RowsAffected == 0 {
			return ErrNodeStatusChanged
		}
	} else {
		result = db.WithContext(dbCtx).Model(node).Updates(values)
	}
	if err := result.Error; err != nil {
		return err
	}
	return nil
}

func (node *Node) Sync(ctx context.Context, db *gorm.DB) error {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.WithContext(dbCtx).Model(node).First(node).Error; err != nil {
		return err
	}
	return nil
}

func (node *Node) SyncStatus(ctx context.Context, db *gorm.DB) error {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var res Node
	if err := db.WithContext(dbCtx).Model(node).Select("status").First(&res, node.ID).Error; err != nil {
		return err
	}
	node.Status = res.Status
	return nil
}

func GetNodeByAddress(ctx context.Context, db *gorm.DB, address string) (*Node, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	node := &Node{Address: address}
	if err := db.WithContext(dbCtx).Model(node).Where(node).First(node).Error; err != nil {
		return nil, err
	}
	return node, nil
}

func GetNodesByAddresses(ctx context.Context, db *gorm.DB, addresses []string) ([]*Node, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var nodes []*Node
	if err := db.WithContext(dbCtx).Model(&Node{}).Where("address in (?)", addresses).Find(&nodes).Error; err != nil {
		return nil, err
	}
	return nodes, nil
}

func GetDelegatedNodes(ctx context.Context, db *gorm.DB) ([]*Node, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var nodes []*Node
	if err := db.WithContext(dbCtx).Model(&Node{}).Where("delegator_share > ?", 0).Find(&nodes).Error; err != nil {
		return nil, err
	}
	return nodes, nil
}

type NodeModel struct {
	gorm.Model
	NodeAddress string `json:"node_address" gorm:"index;index:idx_node_models_hf_model_id_node_address,priority:2"`
	ModelID     string `json:"model_id" gorm:"index"`
	HFModelID   string `json:"hf_model_id" gorm:"column:hf_model_id;not null;default:'';size:191;index:idx_node_models_hf_model_id_node_address,priority:1"`
	InUse       bool   `json:"in_use"`
	Node        Node   `gorm:"foreignKey:Address;references:NodeAddress"`
}

// NewNodeModel builds a NodeModel with HFModelID derived from the dispatch
// model ID, so hf_model_id stays consistent across all write paths.
func NewNodeModel(nodeAddress, modelID string, inUse bool) NodeModel {
	hfModelID, _ := BaseModelHuggingFaceID(modelID)
	return NodeModel{
		NodeAddress: nodeAddress,
		ModelID:     modelID,
		HFModelID:   hfModelID,
		InUse:       inUse,
	}
}

func (nodeModel *NodeModel) Save(ctx context.Context, db *gorm.DB) error {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.WithContext(dbCtx).Save(nodeModel).Error; err != nil {
		return err
	}
	return nil
}

func (nodeModel *NodeModel) Update(ctx context.Context, db *gorm.DB, values map[string]interface{}) error {
	if nodeModel.ID == 0 {
		return errors.New("Node.ID cannot be 0 when update")
	}
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.WithContext(dbCtx).Model(nodeModel).Updates(values).Error; err != nil {
		return err
	}
	return nil
}

func CreateNodeModels(ctx context.Context, db *gorm.DB, nodeModels []NodeModel) error {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.WithContext(dbCtx).Create(&nodeModels).Error; err != nil {
		return err
	}
	return nil
}

func GetNodeModelsByNodeAddress(ctx context.Context, db *gorm.DB, nodeAddress string) ([]NodeModel, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var nodeModels []NodeModel
	if err := db.WithContext(dbCtx).Model(&NodeModel{}).Where("node_address = ?", nodeAddress).Order("id").Find(&nodeModels).Error; err != nil {
		return nil, err
	}
	return nodeModels, nil
}

func GetNodeModel(ctx context.Context, db *gorm.DB, nodeAddress, modelID string) (*NodeModel, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	nodeModel := &NodeModel{NodeAddress: nodeAddress, ModelID: modelID}
	if err := db.WithContext(dbCtx).Model(nodeModel).Where(nodeModel).First(nodeModel).Error; err != nil {
		return nil, err
	}
	return nodeModel, nil
}

type HFModelNodeCount struct {
	OnDisk   int64
	InMemory int64
}

// CountNodesByHFModelID returns, per huggingface base model, the number of
// distinct nodes that have the model on disk and the number of distinct nodes
// that currently have it loaded in GPU memory. Node addresses are counted
// distinctly because one node may hold several variants of the same model.
func CountNodesByHFModelID(ctx context.Context, db *gorm.DB) (map[string]HFModelNodeCount, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	type row struct {
		HFModelID string
		OnDisk    int64
		InMemory  int64
	}
	var rows []row
	if err := db.WithContext(dbCtx).Model(&NodeModel{}).
		Select("hf_model_id, COUNT(DISTINCT node_address) AS on_disk, COUNT(DISTINCT CASE WHEN in_use THEN node_address END) AS in_memory").
		Where("hf_model_id <> ''").
		Group("hf_model_id").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	counts := make(map[string]HFModelNodeCount, len(rows))
	for _, r := range rows {
		counts[r.HFModelID] = HFModelNodeCount{OnDisk: r.OnDisk, InMemory: r.InMemory}
	}
	return counts, nil
}

func GetBusyNodeCount(ctx context.Context, db *gorm.DB) (int64, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var res int64
	if err := db.WithContext(dbCtx).Model(&Node{}).Where("status = ?", NodeStatusBusy).Count(&res).Error; err != nil {
		return 0, err
	}
	return res, nil
}

func GetAllNodeCount(ctx context.Context, db *gorm.DB) (int64, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var res int64
	if err := db.WithContext(dbCtx).Model(&NetworkNodeData{}).Count(&res).Error; err != nil {
		return 0, err
	}
	return res, nil
}

func GetNodeCountsByStatus(ctx context.Context, db *gorm.DB) (map[NodeStatus]int64, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	type row struct {
		Status NodeStatus
		Count  int64
	}
	var rows []row
	if err := db.WithContext(dbCtx).Model(&Node{}).
		Select("status, count(*) as count").
		Group("status").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	res := make(map[NodeStatus]int64, len(rows))
	for _, r := range rows {
		res[r.Status] = r.Count
	}
	return res, nil
}

func GetAliveNodeCount(ctx context.Context, db *gorm.DB, since time.Time) (int64, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var res int64
	if err := db.WithContext(dbCtx).Model(&Node{}).
		Where("last_seen_time >= ?", since).
		Count(&res).Error; err != nil {
		return 0, err
	}
	return res, nil
}

func GetActiveNodeCount(ctx context.Context, db *gorm.DB) (int64, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var res int64
	if err := db.WithContext(dbCtx).Model(&Node{}).Where("status != ?", NodeStatusQuit).Count(&res).Error; err != nil {
		return 0, err
	}
	return res, nil
}
