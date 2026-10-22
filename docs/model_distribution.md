# Model Distribution Mechanism

This document specifies how Relay distributes model files across nodes ahead of task assignment. A periodic model distribution controller tracks, per required model file set, how many nodes hold the files on disk and how frequently tasks require them, and emits `DownloadModel` events to spread the files to more nodes when demand exceeds coverage. Separately, Relay can force-download configured model file sets onto nodes whose GPU VRAM meets a configured minimum, so competing nodes share the same on-disk inventory for fairness. Task matching never emits download events; it only applies the model file gate specified in [node_selection.md](./node_selection.md) and leaves blocked tasks queued.

## Inventory Versus Loaded Projection

`node_models` is the authoritative on-disk inventory for currently joined nodes. Each row is one `(node_address, model_file_hash)` pair that references the `model_files` catalog, whose primary key is the exact model file hash and whose canonical JSON includes the revision. Model distribution coverage, download completion, the task-matching model file gate, and create-time HuggingFace validation skip decisions MUST use exact model file hash matches or, for revision-less requirements, source hash matches through the catalog.

`node_in_memory_models` is the authoritative in-memory model of each node and MUST NOT participate in model distribution.

`loaded_models` is a historical successful-execution projection used by `GET /v2/loaded-models`. It MUST NOT drive model distribution, task matching, or create-time Hub skip decisions. See [loaded_models.md](./loaded_models.md).

## Required Model File Set

A task requires the model files that Relay derives from its TaskArgs when the task is created. For an LLM task the set contains the model repository. For an image task the set contains the diffusers base model and every component repository. Component entries MAY use format `diffusers` for Diffusers-layout weights; that format string does not make the entry a base pipeline. LoRA and quantization configuration are not model files. Each member is a model file entry with source type, repository, optional revision, format, and format kwargs.

Relay stores the set on the task as `RequiredModelFileHashes` (exact hashes of members with a revision) and `RequiredModelFileSourceHashes` (source hashes of members without a revision). Relay resolves a TaskArgs revision to its commit SHA at task creation unless the exact file is already held by a node. Validation-group members pin exact revisions inside TaskArgs before create, as specified in [task_validation_and_slashing.md](./task_validation_and_slashing.md).

The required model file set of a task is identified by its distribution key, the SHA-256 of the sorted concatenation of all exact and source member hashes. Model distribution, readiness evaluation, and download scheduling MUST operate on the complete set as one unit. `InferenceTask.ModelIDs` MUST NOT participate in model distribution.

## Demand Groups and VRAM Requirements

The unit of model distribution is the demand group: one required model file set combined with one VRAM demand, the LLM Darwin exclusion when applicable, and the task's `RequireLinuxCuda` flag. A task's VRAM demand is `RequiredGPUVRAM` when the task sets `RequiredGPU`, and `MinVRAM` otherwise. A node satisfies a demand group when its VRAM is at least the group's VRAM demand, the node is not a Darwin node when the group excludes Darwin, and, when the group sets `RequireLinuxCuda`, the node's GPU name platform suffix is `docker` or `Linux`. Model distribution MUST NOT match GPU names against `RequiredGPU`; exact-GPU matching is a task-matching concern. `RequireLinuxCuda` demand-group rules MUST match the matching filter in [node_selection.md](./node_selection.md) and [quantization_platform_support.md](./quantization_platform_support.md).

Tasks requiring the same model file set but different VRAM demands, Darwin exclusions, or `RequireLinuxCuda` values form separate demand groups. Demand measurement, target computation, coverage, deficit, and download target selection MUST all be evaluated per demand group. A node MUST count toward the coverage of a demand group, and MUST be eligible as a download target for it, only when the node satisfies the group. Task versions and node-status filters beyond the rules in this document remain task-matching concerns specified in [node_selection.md](./node_selection.md).

Selection records remain keyed by the distribution key only: a download commanded for one demand group produces on-disk files that serve every demand group of that set on nodes that satisfy them.

## Controller Loop

Relay MUST run one model distribution controller. The controller MUST run on a fixed interval of `model_distribution.controller_interval_seconds`, independent of task matching rounds. Each run MUST evaluate every demand group with current demand.

A demand group has current demand when at least one of these holds:

- At least one queued task requires the model file set with the group's VRAM demand.
- At least one task requiring the model file set with the group's VRAM demand was created within the demand window of `model_distribution.demand_window_seconds`.

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

A node holds a model file set when the node is currently joined in `Available` or `Busy` status and its authoritative on-disk model file inventory contains every exact hash and at least one file of every source hash of the set. Holding MUST be computed from `node_models` and the `model_files` catalog. Nodes in `Paused`, `PendingPause`, `PendingQuit`, or `Quit` status MUST NOT count toward coverage, because they cannot accept new tasks.

The deficit of a demand group is:

```
deficit = target - (qualified_holding_node_count + qualified_pending_selection_count)
```

`qualified_holding_node_count` counts holding nodes that satisfy the group. `qualified_pending_selection_count` counts nodes with a non-expired pending selection for the model file set that satisfy the group. A holding node or pending selection on a node that does not satisfy the group MUST NOT count toward the group's coverage.

When the deficit is zero or negative, the controller MUST NOT select nodes or emit events for the group. Coverage above the target MUST NOT be reduced by the controller; it converges as demand falls because no new selections are created.

## Download Target Selection

When the deficit is positive, the controller MUST select up to `deficit` new download target nodes for the demand group.

The selection pool consists of currently joined nodes in `Available` or `Busy` status that satisfy the group, pass the node-name policy filter of [node_selection.md](./node_selection.md) and do not hold the model file set. Nodes in `Paused`, `PendingPause`, `PendingQuit`, or `Quit` status MUST NOT be selected. Model downloads run on the node independently of inference execution, so a node executing a task remains a valid download target. The controller MUST additionally exclude from the pool:

- nodes with a non-expired pending selection for the model file set
- nodes with any selection record for the model file set created during the set's current demand period

Only when the pool is otherwise empty MAY the controller re-admit nodes whose previous selection for the model file set expired, ordered so that every eligible node is attempted once before any node is attempted twice. A re-selection is a new attempt and emits a new event.

Sampling MUST be weighted sampling without replacement using the staking and QoS base weights defined in [node_selection.md](./node_selection.md). The model file gate and the in-memory locality boost MUST NOT apply. Candidates with zero weight MUST be selected only after all positive-weight candidates are exhausted, in ascending node address order.

For each selected node, the controller MUST emit one `DownloadModel` event carrying the complete required model file set.

## Selection Records

Relay SHALL persist one selection record per selection attempt in `node_model_download_selections`. Each record MUST contain:

- the distribution key of the required model file set, together with the exact and source member hash lists
- the selected node address
- `min_vram`, the VRAM requirement of the demand group that triggered the selection, used to derive the `vram_tier` label of the model download metrics
- `sent_at`, the selection time
- `deadline`, equal to `sent_at` plus `model_distribution.download_timeout_seconds`
- `status`, one of `pending`, `completed`, `expired`
- creation and update timestamps

More than one non-expired record per distribution key and node address MUST NOT exist. The controller MUST insert the selection record and the corresponding `DownloadModel` event in one database transaction. Concurrent controller runs MUST be prevented from creating duplicate non-expired records for the same model and node through this uniqueness constraint.

### Status Transitions

- A `pending` selection becomes `completed` when a controller run observes that the node's authoritative on-disk model file inventory covers the required model file set. Completion is derived from `node_models`; it does not require any coupling to the reporting transaction.
- A `pending` selection becomes `expired` when a controller run observes that `deadline` has passed without completion.
- `completed` and `expired` are terminal for that record. A later attempt for the same model and node creates a new record.

Before its deadline, a `pending` selection MUST keep occupying capacity regardless of node state changes, including the node becoming Busy, Paused, or unreachable. Relay MUST NOT resend `DownloadModel` events for a pending selection.

When a node quits or is slashed, Relay MUST delete the node's selection records in the same transaction that deletes its `node_models` rows, so a rejoining node can be selected again.

When joined-node capability synchronization commits a full on-disk inventory, Relay MUST delete that node's selection records whose required model file set is not covered by the reported inventory, in the same transaction. Pending, completed, and expired attempts for those sets MUST all be deleted so the node can be selected again while the demand period is still open. Selection records whose set is covered by the reported inventory MUST remain unchanged.

When a controller run finds a model file set without current demand, and the set is not listed in `required_models_by_vram`, it MUST delete the set's selection records. When the set is listed in `required_models_by_vram` and has no current task demand, the controller MUST delete only `completed` and `expired` selection records for that set and MUST retain `pending` records until they complete or expire. Downloads already commanded on nodes are not cancelled; files that finish downloading later enter the on-disk inventory through the normal reporting paths and count toward coverage if demand returns.

### Demand Period

A demand period of a model file set starts at the first controller run that finds the set with current demand while holding no selection records, and ends at the first controller run that finds the set without current demand. The exclusion of already-attempted nodes applies within one demand period. A new demand period starts with no attempted-node history.

## Required Models by VRAM

Relay MAY configure a list of required model file sets under `model_distribution.required_models_by_vram`. Each entry binds a `min_vram`, a `task_type`, and a `model_files` list that forms one required model file set. An empty list disables the feature. Every HuggingFace `repository` value in `model_files`, in TaskArgs-derived model file entries, and in node-reported model file entries MUST be normalized with `NormalizeModelName` before hashing, catalog upsert, download selection, and `DownloadModel` emission, so repository names that differ only by letter case identify the same model file.

After a successful node join commit, and after a successful joined-node capability synchronization commit, Relay MUST evaluate every configured entry whose `min_vram` is less than or equal to the node's `GPUVram`. For each matching set that the node's authoritative on-disk inventory does not cover, and for which the node has no non-expired selection record, Relay MUST insert a selection record and emit one `DownloadModel` event in the same transaction, using the entry's `task_type` and `model_distribution.download_timeout_seconds`. A node whose VRAM is below an entry's `min_vram` MUST NOT receive that entry's set. When several entries match the same node, Relay MUST emit downloads for every unmatched set.

Required-model downloads reuse `node_model_download_selections` and share uniqueness with demand-driven selections. Relay MUST NOT scan already-online nodes for required models outside join and capability synchronization. An expired required-model selection is retried only when the node joins again or synchronizes capabilities again while still missing the set.

## Completion and On-Disk Authority

The signed AddModelID node API is the authoritative incremental report that one model file is present on disk. The reported model file entry MUST have source type `huggingface`, a non-empty repository, and a concrete revision. Relay MUST upsert the entry into the `model_files` catalog and create the `node_models` row when it does not already exist. Model files reported at node join MUST create `node_models` rows through the same validation, and node join MUST clear the node's in-memory model record. After the join transaction commits, Relay MUST run the required-models-by-VRAM evaluation for that node.

The signed joined-node capability synchronization is the authoritative full on-disk inventory report after node startup. Relay MUST validate and deduplicate the reported entries, upsert them into the catalog, permanently delete `node_models` rows absent from the report, create rows for newly reported entries, and delete the node's selection records whose required model file set is no longer covered, all in one transaction. The synchronization MUST refresh the node scheduling index after the inventory transaction commits. After that commit, Relay MUST run the required-models-by-VRAM evaluation for that node.

An AddModelID report MUST remain authoritative even when no matching selection exists. Relay MUST retain the reported `node_models` row and MUST NOT require a selection before accepting the report.

`node_models` rows MUST NOT use soft delete. When a node quits, Relay MUST permanently remove its rows, its in-memory model record, and its selection records. All rows MUST be created through `NewNodeModel`.

A queued task MUST start as soon as task matching finds a qualified node that holds the task's required model files on disk, independent of any selection state. Outstanding downloads on other nodes continue and serve later tasks.

## Failure Behavior

- Event loss: a node that never receives or never acts on a `DownloadModel` event produces no report. The selection expires at its deadline and the controller selects a replacement when a deficit remains.
- Silent node-side failure: the node reports nothing on download failure. Handling is identical to event loss.
- Expiry racing with completion: a node that reports after its selection expired still enters the on-disk inventory and counts toward coverage. The expired record stays expired.
- Relay restart: selection records, deadlines, and demand history are persisted; the controller resumes from persisted state. `DownloadModel` events emitted while a node was offline are not redelivered; the affected selections expire and are replaced through the normal deficit path.
- Node capability mismatch: a node may hold a model file set without qualifying for every task that requires it, because the node downloaded the files for a different demand group, reported them at join, or reported them without a selection. Such a copy MUST NOT count toward the coverage of demand groups the node does not satisfy; the controller keeps selecting qualified nodes for those groups. Exact-GPU matching, task version, and node-status filtering remain task-matching concerns; model distribution does not compensate for them.

## Task Start Model State

Task start MUST NOT create `NodeModel` rows, MUST NOT modify the node in-memory model record, and MUST NOT emit `DownloadModel` events. The model file gate guarantees that the assigned node has already reported every required model file. Task start MUST only store the resolved `ExecutionModelFileHashes` on the task as specified in [task_matching.md](./task_matching.md).

The in-memory model of a node changes only through the signed node in-memory model report, `POST /v2/nodes/:address/in_memory_model`. The report carries the hex `model_cache_hash`, the `model_cache_config` the hash is computed from, the exact `model_files` with revisions, and the `execution_dtype`, or a null model when the node memory is empty. Relay MUST reject a report whose hash is not the SHA-256 of the canonical JSON of the reported config, and MUST refresh the node scheduling index after the report commits.

## Event Contract

Each emitted request MUST use event type `DownloadModel` with this payload:

```go
type DownloadModelEvent struct {
    NodeAddress string           `json:"node_address"`
    ModelFiles  []ModelFileEntry `json:"model_files"`
    TaskType    TaskType         `json:"task_type"`
}
```

`ModelFiles` MUST contain every member of the required model file set. A member with a revision is an exact file; a member with an empty revision is a source of which the node downloads one revision. For demand-driven downloads, `TaskType` MUST be the task type of the most recently created demanding task of the set. For required-models-by-VRAM downloads, `TaskType` MUST be the configured `task_type` of the matching entry. The event MUST be stored in the `events` table and addressed to the selected node. Node polling and delivery are specified in [relay_event_stream.md](./relay_event_stream.md).

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
| `model_distribution.required_models_by_vram` | `[]` |

## Relevant Source Files

| File | Responsibility |
|------|----------------|
| `service/model_distribution.go` | Model distribution controller loop, demand-group measurement, VRAM qualification, target computation, download-target sampling, and event emission |
| `service/required_model_downloads.go` | Config-driven required model downloads by minimum VRAM after join and capability synchronization |
| `models/node_model_download_selection.go` | Persistent selection records, uniqueness, and status transitions |
| `api/v1/nodes/add_model_id.go` | Authoritative incremental on-disk model file report |
| `api/v2/nodes/in_memory_model.go` | Authoritative in-memory model report |
| `service/node_model_state.go` | Model file coverage, node inventory replacement, and in-memory model updates |
| `api/v2/nodes/capabilities.go` | Authenticated full on-disk inventory report for joined nodes |
| `service/node_capabilities.go` | Joined-node capability and full model-inventory synchronization |
| `service/node.go` | Join-time model file reporting, join-time in-memory model reset, and quit-time cleanup |
| `models/model_file.go` | Model file entries, hashes, and the `model_files` catalog |
| `models/model_file_task.go` | Derivation of task required model files and the requested model cache hash |
| `models/event.go` | `DownloadModelEvent` payload |
