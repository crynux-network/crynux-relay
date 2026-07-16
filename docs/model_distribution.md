# Model Distribution Mechanism

This document specifies how Relay distributes base models across nodes ahead of task assignment. A periodic model distribution controller tracks, per base model, how many nodes hold the model on disk and how frequently tasks require it, and emits `DownloadModel` events to spread the model to more nodes when demand exceeds coverage. Task matching never emits download events; it only applies the base-model gate specified in [node_selection.md](./node_selection.md) and leaves blocked tasks queued.

## Single Base Model

Every inference task requires exactly one base model. `InferenceTask.ModelIDs` stores entries in the dispatch format `<usage>:<name>`, and exactly one entry carries the `base:` usage; all other entries are auxiliary `lora:` and `controlnet:` models that are downloaded as part of task execution on the assigned node.

Relay MUST treat a task's base model as a single scalar value, extracted as the lowercase `base:` entry of `ModelIDs`. Model distribution, readiness evaluation, and download scheduling MUST operate on this single model ID. Auxiliary model IDs MUST NOT participate in model distribution.

## Demand Groups and VRAM Requirements

The unit of model distribution is the demand group: one base model combined with one VRAM demand and, for LLM tasks, the Darwin exclusion. A task's VRAM demand is `RequiredGPUVRAM` when the task sets `RequiredGPU`, and `MinVRAM` otherwise. A node satisfies a demand group when its VRAM is at least the group's VRAM demand and, for LLM demand groups, the node is not a Darwin node. Model distribution MUST NOT match GPU names against `RequiredGPU`; exact-GPU matching is a task-matching concern.

Tasks requiring the same base model but different VRAM demands form separate demand groups. Demand measurement, target computation, coverage, deficit, and download target selection MUST all be evaluated per demand group. A node MUST count toward the coverage of a demand group, and MUST be eligible as a download target for it, only when the node satisfies the group. Task versions and node-status filters beyond the rules in this document remain task-matching concerns specified in [node_selection.md](./node_selection.md).

Selection records and `DownloadModel` events remain keyed by base model ID only: a download commanded for one demand group produces an on-disk copy that serves every demand group of that model on nodes that satisfy them.

## Controller Loop

Relay MUST run one model distribution controller. The controller MUST run on a fixed interval of `model_distribution.controller_interval_seconds`, independent of task matching rounds. Each run MUST evaluate every demand group with current demand.

A demand group has current demand when at least one of these holds:

- At least one queued task requires the model with the group's VRAM demand.
- At least one task requiring the model with the group's VRAM demand was created within the demand window of `model_distribution.demand_window_seconds`.

A controller run MUST execute these steps for each demand group with current demand:

1. Measure demand and compute the target node count.
2. Count nodes holding the model and non-expired pending selections.
3. When a deficit exists, select new download target nodes and emit `DownloadModel` events.

A controller run failure MUST NOT affect task matching, matched task pairs, or task start transactions. The next run MUST recompute from persisted state and the authoritative on-disk inventory.

## Target Node Count

For each demand group, the controller MUST measure over the demand window:

- `arrival_rate`: the number of tasks in the group created within the window divided by the window length in seconds.
- `avg_execution_seconds`: the mean measured execution duration (`ScoreReadyTime - StartTime`) of completed tasks in the group within the window. When the window contains no such completed task, the controller MUST use the mean of the stored estimated node seconds, defined in [task-pricing.md](./task-pricing.md), of the demanding tasks.

The target node count is:

```
target = clamp(ceil(arrival_rate * avg_execution_seconds * model_distribution.safety_factor),
               model_distribution.min_nodes,
               model_distribution.max_nodes)
```

A demand group with current demand MUST receive at least `model_distribution.min_nodes` as its target, which covers the cold-start case of a model that has never been executed.

## Coverage and Deficit

A node holds a base model when the node is currently joined in `Available` or `Busy` status and its authoritative on-disk model set contains the model ID. Holding MUST be computed from `node_models`. Nodes in `Paused`, `PendingPause`, `PendingQuit`, or `Quit` status MUST NOT count toward coverage, because they cannot accept new tasks.

The deficit of a demand group is:

```
deficit = target - (qualified_holding_node_count + qualified_pending_selection_count)
```

`qualified_holding_node_count` counts holding nodes that satisfy the group. `qualified_pending_selection_count` counts nodes with a non-expired pending selection for the model that satisfy the group. A holding node or pending selection on a node that does not satisfy the group MUST NOT count toward the group's coverage.

When the deficit is zero or negative, the controller MUST NOT select nodes or emit events for the group. Coverage above the target MUST NOT be reduced by the controller; it converges as demand falls because no new selections are created.

## Download Target Selection

When the deficit is positive, the controller MUST select up to `deficit` new download target nodes for the demand group.

The selection pool consists of currently joined nodes in `Available` or `Busy` status that satisfy the group, pass the node-name policy filter of [node_selection.md](./node_selection.md) and do not hold the model. Nodes in `Paused`, `PendingPause`, `PendingQuit`, or `Quit` status MUST NOT be selected. Model downloads run on the node independently of inference execution, so a node executing a task remains a valid download target. The controller MUST additionally exclude from the pool:

- nodes with a non-expired pending selection for the model
- nodes with any selection record for the model created during the model's current demand period

Only when the pool is otherwise empty MAY the controller re-admit nodes whose previous selection for the model expired, ordered so that every eligible node is attempted once before any node is attempted twice. A re-selection is a new attempt and emits a new event.

Sampling MUST be weighted sampling without replacement using the staking and QoS base weights defined in [node_selection.md](./node_selection.md). The base-model gate and the in-memory locality boost MUST NOT apply. Candidates with zero weight MUST be selected only after all positive-weight candidates are exhausted, in ascending node address order.

For each selected node, the controller MUST emit one `DownloadModel` event carrying the base model ID.

## Selection Records

Relay SHALL persist one selection record per selection attempt in `node_model_download_selections`. Each record MUST contain:

- the base model ID
- the selected node address
- `min_vram`, the VRAM requirement of the demand group that triggered the selection, used to derive the `vram_tier` label of the model download metrics
- `sent_at`, the selection time
- `deadline`, equal to `sent_at` plus `model_distribution.download_timeout_seconds`
- `status`, one of `pending`, `completed`, `expired`
- creation and update timestamps

More than one non-expired record per base model and node address MUST NOT exist. The controller MUST insert the selection record and the corresponding `DownloadModel` event in one database transaction. Concurrent controller runs MUST be prevented from creating duplicate non-expired records for the same model and node through this uniqueness constraint.

### Status Transitions

- A `pending` selection becomes `completed` when a controller run observes that the node's authoritative on-disk model set contains the model ID. Completion is derived from `node_models`; it does not require any coupling to the reporting transaction.
- A `pending` selection becomes `expired` when a controller run observes that `deadline` has passed without completion.
- `completed` and `expired` are terminal for that record. A later attempt for the same model and node creates a new record.

Before its deadline, a `pending` selection MUST keep occupying capacity regardless of node state changes, including the node becoming Busy, Paused, or unreachable. Relay MUST NOT resend `DownloadModel` events for a pending selection.

When a node quits or is slashed, Relay MUST delete the node's selection records in the same transaction that deletes its `node_models` rows, so a rejoining node can be selected again.

When a controller run finds a base model without current demand, it MUST delete the model's selection records. Downloads already commanded on nodes are not cancelled; models that finish downloading later enter the on-disk inventory through the normal reporting paths and count toward coverage if demand returns.

### Demand Period

A demand period of a base model starts at the first controller run that finds the model with current demand while holding no selection records, and ends at the first controller run that finds the model without current demand. The exclusion of already-attempted nodes applies within one demand period. A new demand period starts with no attempted-node history.

## Completion and On-Disk Authority

The signed AddModelID node API is the authoritative report that a model is present on disk. Relay MUST normalize the reported model ID and create the `node_models` row when it does not already exist. Model IDs reported at node join MUST create `node_models` rows through the same normalization.

An AddModelID report MUST remain authoritative even when no matching selection exists. Relay MUST retain the reported `node_models` row and MUST NOT require a selection before accepting the report.

`node_models` rows MUST NOT use soft delete. When a node quits, Relay MUST permanently remove its rows. All rows MUST be created through `NewNodeModel`, which normalizes `model_id` to lowercase and derives the lowercase `hf_model_id` when the ID is a supported Hugging Face base model.

A queued task MUST start as soon as task matching finds a qualified node that holds the task's base model on disk, independent of any selection state. Outstanding downloads on other nodes continue and serve later tasks.

## Failure Behavior

- Event loss: a node that never receives or never acts on a `DownloadModel` event produces no report. The selection expires at its deadline and the controller selects a replacement when a deficit remains.
- Silent node-side failure: the node reports nothing on download failure. Handling is identical to event loss.
- Expiry racing with completion: a node that reports after its selection expired still enters the on-disk inventory and counts toward coverage. The expired record stays expired.
- Relay restart: selection records, deadlines, and demand history are persisted; the controller resumes from persisted state. `DownloadModel` events emitted while a node was offline are not redelivered; the affected selections expire and are replaced through the normal deficit path.
- Node capability mismatch: a node may hold a base model without qualifying for every task that requires it, because the node downloaded the model for a different demand group, reported it at join, or reported it without a selection. Such a copy MUST NOT count toward the coverage of demand groups the node does not satisfy; the controller keeps selecting qualified nodes for those groups. Exact-GPU matching, task version, and node-status filtering remain task-matching concerns; model distribution does not compensate for them.

## Task Start Model State

Task start MUST NOT create `NodeModel` rows and MUST NOT emit `DownloadModel` events. The base-model gate guarantees that the assigned node has already reported the task's base model.

When a task starts, Relay MUST update only existing, node-reported base-model rows:

- The existing row for the task's base model MUST be marked `in_use = true`.
- Existing base-model rows currently in use but not required by the task MUST be marked `in_use = false`.
- Missing rows, auxiliary task model IDs, and non-base rows MUST NOT create or modify rows.

## Event Contract

Each emitted request MUST use event type `DownloadModel` with this payload:

```go
type DownloadModelEvent struct {
    NodeAddress string   `json:"node_address"`
    ModelID     string   `json:"model_id"`
    TaskType    TaskType `json:"task_type"`
}
```

`TaskType` MUST be the task type of the most recently created demanding task of the model. The event MUST be stored in the `events` table and addressed to the selected node. Node polling and delivery are specified in [relay_event_stream.md](./relay_event_stream.md).

## Configuration

Relay MUST define these settings in every runtime configuration template with these defaults:

| Setting | Default |
|---------|---------|
| `model_distribution.controller_interval_seconds` | `60` |
| `model_distribution.demand_window_seconds` | `1800` |
| `model_distribution.safety_factor` | `2.0` |
| `model_distribution.min_nodes` | `1` |
| `model_distribution.max_nodes` | `10` |
| `model_distribution.download_timeout_seconds` | `1800` |

## Relevant Source Files

| File | Responsibility |
|------|----------------|
| `service/model_distribution.go` | Model distribution controller loop, demand-group measurement, VRAM qualification, target computation, download-target sampling, and event emission |
| `models/node_model_download_selection.go` | Persistent selection records, uniqueness, and status transitions |
| `api/v1/nodes/add_model_id.go` | Authoritative on-disk report |
| `service/node.go` | Join-time model reporting, quit-time cleanup, and task-start updates to existing base-model in-use state |
| `models/model_id.go` | Model ID normalization and base-model extraction |
| `models/event.go` | `DownloadModelEvent` payload |
