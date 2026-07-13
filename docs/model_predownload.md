# Model Pre-download Mechanism

This document describes how the Crynux Relay proactively distributes popular models across nodes, so that future tasks are more likely to find nodes that already have the required models locally.

## Overview

When a task starts, the relay not only ensures the assigned node has the required models, but also spreads those models to additional nodes if they are not yet widely available. This is called the **model pre-download mechanism**.

The mechanism works in two stages:

1. **Assigned node download**: If the task's assigned node does not have a required model locally, it is told to download it.
2. **Proactive spreading**: If fewer than 3 available nodes in the network have a required model, the relay selects additional nodes and tells them to download it preemptively.

This increases the model locality coverage over time, improving the chance that future tasks benefit from the [Model Locality Boost](qos.md#model-locality-boost) during node selection.

## Trigger Point

The pre-download logic is triggered inside `SetTaskStatusStarted` (in `service/task_status.go`), which is called after a task is dispatched to a node. The flow leading to this point is:

```
TaskDispatcher.Dispatch()
  └─> selectNodeForInferenceTask()   // select a node using QoS + Staking + Model Locality Boost
       └─> TaskDispatcher.Process()
            └─> processDispatchedTasks()
                 └─> SetTaskStatusStarted()   // <-- pre-download logic runs here
```

See `service/start_task.go` line 244 for the call to `SetTaskStatusStarted`.

## Detailed Flow

### Step 1: Assigned Node Download

After the task is committed to the database and the node's status is updated, the relay checks whether the assigned node already has each of the task's required models locally.

For each model in the task's `ModelIDs` list:

- The relay builds a lookup set of the node's locally tracked models (from the `node_models` table).
- If the model is **not** in the node's local set, a `DownloadModelEvent` is emitted targeting that node.

```go
// service/task_status.go, lines 92-106
localModelSet := make(map[string]models.NodeModel)
for _, model := range node.Models {
    localModelSet[model.ModelID] = model
}

for _, modelID := range task.ModelIDs {
    download := false
    if _, ok := localModelSet[modelID]; !ok {
        emitEvent(ctx, db, &models.DownloadModelEvent{
            NodeAddress: node.Address,
            ModelID:     modelID,
            TaskType:    task.TaskType,
        })
        download = true
    }
    // ... (Step 2 continues below)
}
```

### Step 2: Proactive Spreading

For each required model (regardless of whether the assigned node had it or not), the relay checks how widely available the model is across the network:

1. **Count available nodes with the model**: The function `countAvailableNodesWithModelID` counts how many nodes in `Available` status have the model in their local `node_models` records.

2. **Threshold check**: If the count is **less than 3**, the relay decides to spread the model to more nodes.

3. **Select additional nodes**: The function `selectNodesForDownloadTask` selects up to `10 - count` nodes that do **not** already have the model. These nodes are chosen using the same QoS + Staking weighted random selection used for inference task assignment (but without the model locality boost).

4. **Emit download events**: A `DownloadModelEvent` is emitted for each selected node, unless it is the same node that was already told to download in Step 1.

```go
// service/task_status.go, lines 108-128
count, err := countAvailableNodesWithModelID(ctx, db, modelID)
if err != nil {
    return err
}
if count < 3 {
    downloadNodes, err := selectNodesForDownloadTask(ctx, &task, modelID, 10-int(count))
    if err != nil {
        return err
    }
    if len(downloadNodes) > 0 {
        for _, downloadNode := range downloadNodes {
            if !download || node.Address != downloadNode.Address {
                emitEvent(ctx, db, &models.DownloadModelEvent{
                    NodeAddress: downloadNode.Address,
                    ModelID:     modelID,
                    TaskType:    task.TaskType,
                })
            }
        }
    }
}
```

### Example Scenario

Suppose a task requires model `M` and the network state is:

| Metric | Value |
|--------|-------|
| Available nodes with model `M` | 2 |
| Assigned node has model `M` locally | No |

The relay will:

1. Emit `DownloadModelEvent` to the assigned node for model `M` (Step 1).
2. Count that only 2 available nodes have `M` (which is < 3).
3. Call `selectNodesForDownloadTask` to pick up to `10 - 2 = 8` additional nodes that don't have `M`.
4. Emit `DownloadModelEvent` to each of those selected nodes (skipping the assigned node since it was already notified).

After this, up to 11 nodes (2 existing + 1 assigned + 8 additional) could potentially have model `M`.

## Node Selection for Download Tasks

The function `selectNodesForDownloadTask` (`service/select_nodes.go`, lines 203-248) selects nodes for pre-download using the following process:

### 1. Filter by Hardware

Nodes are first filtered to ensure they can run the model:

- If the task specifies a `RequiredGPU`, only nodes with that exact GPU model are included (`filterNodesByGPU`).
- Otherwise, nodes are filtered by minimum VRAM requirement (`filterNodesByVram`).
- In both cases, nodes must also have a compatible software version.

### 2. Exclude Nodes That Already Have the Model

Nodes that already have the target model in their `node_models` records are excluded from the candidate list.

```go
// service/select_nodes.go, lines 222-234
var validNodes []models.Node
for _, node := range nodes {
    valid := true
    for _, model := range node.Models {
        if model.ModelID == modelID {
            valid = false
            break
        }
    }
    if valid {
        validNodes = append(validNodes, node)
    }
}
```

### 3. Calculate Selection Probabilities

For each remaining candidate node, a selection probability is computed using the same formula as inference task selection:

```
StakingScore = sqrt(staking / maxStaking)
QoSProb      = nodeQoSScore / maxQoSScore   (default 0.5 if 0)
Prob         = StakingScore * QoSProb / (StakingScore + QoSProb)
```

This means higher-staking, higher-QoS nodes are more likely to be chosen for pre-download, which makes sense because these are the nodes most likely to be selected for future inference tasks.

### 4. Weighted Random Selection

The final selection uses weighted random sampling (`gonum/stat/sampleuv.NewWeighted`) to pick up to `n` nodes from the candidates based on their computed probabilities.

**Note**: Unlike inference task selection, download task selection does **not** apply the model locality boost (since the entire point is to select nodes that do *not* have the model).

## DownloadModelEvent

The `DownloadModelEvent` is the mechanism by which the relay tells a node to download a model. It is defined in `models/event.go` (lines 50-66):

```go
type DownloadModelEvent struct {
    NodeAddress string   `json:"node_address"`
    ModelID     string   `json:"model_id"`
    TaskType    ModelType `json:"task_type"`
}
```

When emitted, this event is serialized and stored in the `events` database table with type `"DownloadModel"`. Nodes poll for events addressed to them and act on download requests accordingly.

The event includes:
- **NodeAddress**: Which node should download the model.
- **ModelID**: The identifier of the model to download.
- **TaskType**: The type of task (SD, LLM, etc.), which may help the node determine how to handle the model.

## Model Tracking (NodeModel)

The relay tracks which models each node has locally using the `NodeModel` database table (`models/node.go`, lines 105-111):

```go
type NodeModel struct {
    ID          uint      `json:"id" gorm:"primaryKey"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
    NodeAddress string    `json:"node_address" gorm:"index;index:idx_node_models_hf_model_id_node_address,priority:2"`
    ModelID     string    `json:"model_id" gorm:"index"`
    HFModelID   string    `json:"hf_model_id" gorm:"column:hf_model_id;not null;default:'';size:191;index:idx_node_models_hf_model_id_node_address,priority:1"`
    InUse       bool      `json:"in_use"`
    Node        Node      `gorm:"foreignKey:Address;references:NodeAddress"`
}
```

`node_models` rows MUST NOT use soft delete. When a node quits, its rows are removed permanently. All rows MUST be created through the `NewNodeModel` constructor, which normalizes `model_id` to lowercase and derives the lowercase `hf_model_id` from it.

Key behaviors:

- **When a task starts** (`nodeStartTask` in `service/node.go`, lines 110-163): The relay creates new `NodeModel` entries for any models the node didn't previously have, and marks models used by the current task as `InUse: true`. Models not in the current task are marked `InUse: false`.
- **When a node quits** (`SetNodeStatusQuit` in `service/node.go`, lines 69-108): All `NodeModel` entries for that node are deleted.
- **countAvailableNodesWithModelID** (`service/select_nodes.go`, lines 250-265): Queries the `node_models` table joined with `nodes` to count only nodes in `Available` status that have the model.

## Hardcoded Constants

| Constant | Value | Location | Description |
|----------|-------|----------|-------------|
| Minimum node threshold | 3 | `service/task_status.go:112` | If fewer than this many available nodes have the model, proactive spreading is triggered |
| Maximum download targets | 10 | `service/task_status.go:113` | Up to `10 - count` additional nodes are selected for pre-download |

These values are currently hardcoded and not configurable.

## Relevant Source Files

| File | Description |
|------|-------------|
| `service/task_status.go` | `SetTaskStatusStarted` — main function containing the pre-download logic (lines 44-131) |
| `service/select_nodes.go` | `selectNodesForDownloadTask` (lines 203-248) and `countAvailableNodesWithModelID` (lines 250-265) |
| `service/selecting_prob.go` | `CalculateSelectingProb` — probability calculation used for node selection |
| `service/event.go` | `emitEvent` — persists events to the database |
| `service/node.go` | `nodeStartTask` (model tracking updates) and `SetNodeStatusQuit` (model cleanup) |
| `service/start_task.go` | `processDispatchedTasks` — calls `SetTaskStatusStarted` |
| `models/event.go` | `DownloadModelEvent` struct definition |
| `models/node.go` | `NodeModel` struct definition and query functions |
