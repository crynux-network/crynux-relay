# Task Matching

This document specifies how Relay matches queued inference tasks to execution nodes.

## Scope

Task matching covers the dispatch path between the task queue and the task start transaction. Queue ordering and task priority are specified in [task-pricing.md](./task-pricing.md). Candidate filtering rules, weight formulas, and sampling semantics are specified in [node_selection.md](./node_selection.md). This document specifies where that data comes from at dispatch time and how tasks and nodes are paired.

## Definitions

- Node scheduling index: the in-memory view of node state used by the matching scheduler.
- Matching round: one scheduler iteration that pairs a batch of queued tasks with nodes.
- Reservation: the in-round exclusive hold of a node for one matched task before the task start transaction commits.
- Model file: one independently downloadable model source, identified by an exact model file hash over its canonical JSON including the revision, and by a source hash over the same canonical JSON without the revision. A model file entry carries no dtype, quantization, LoRA, component usage name, executor, or GPU.
- Required model files: the model files derived from the task's TaskArgs when the task is created and stored on the task as `RequiredModelFileHashes` (entries with a revision) and `RequiredModelFileSourceHashes` (entries without a revision). A source hash is satisfied by any revision of that source.
- Requested model cache hash: the SHA-256 of the canonical cache config that identifies the in-memory model the task requests. Revisions do not participate in it.
- Requirement signature: the tuple (`TaskType`, `MinVRAM`, `RequiredGPU`, `RequiredGPUVRAM`, `TaskVersion`, `RequiredModelFileHashes`, `RequiredModelFileSourceHashes`, `RequestedModelCacheHash`) that determines a task's candidate set.
- Qualified node: a node that passes availability, hardware, version, task-specific, and node-name policy filters.
- Model-ready node: a qualified node whose on-disk model files cover every exact hash and at least one file for every source hash of the task's required model files.

## Node Scheduling Index

Relay MUST maintain one in-memory node scheduling index covering every non-Quit node. Each index entry MUST contain the fields required by candidate filtering and weight calculation:

- node address
- node status
- current task occupancy (`current_task_id_commitment`)
- GPU name, GPU VRAM
- node version (`major.minor.patch`)
- on-disk exact model file hash set and on-disk model file source hash set
- in-memory model: the cache hash, the exact model file hash set, and the model file source hash set; empty when the node holds no in-memory model
- `QOSScore`, `HealthBase`, `HealthUpdatedAt`, `HealthExcluded`
- `StakeAmount`

Delegated staking, vesting stake, max staking, and node name policy data MUST be read from their existing in-memory caches.

The database is the authoritative store for node state. The node scheduling index is a scheduling acceleration view and MUST NOT be used as a source of truth for any API response or persisted projection.

### Index Maintenance

Relay MUST rebuild the index from the database at startup. The matching scheduler MUST NOT start before the initial rebuild completes.

Relay MUST update the index after the owning database transaction commits, at every code path that changes an indexed field. For each node, Relay MUST hold a per-node lock that spans both the database transaction commit and the index entry update, so that index updates for a node are applied in the same order as their transactions commit:

- node join
- node pause and resume, including deferred pause completion
- node quit, including deferred quit completion
- node kickout
- node slash
- blockchain unstake handling
- task start (`nodeStartTask`): status and occupancy
- task finish (`nodeFinishTask`): status, occupancy, and deferred status transitions
- timeout abort completion
- node model file report (add-model API)
- node in-memory model report
- joined-node capability synchronization (GPU, VRAM, runner version, and full on-disk model file inventory)
- node health penalty and health boost
- node QoS score update
- stake amount changes

When a task start transaction fails with a node status conflict, Relay MUST re-read that node from the database and replace its index entry before the node participates in another matching round.

## Matching Round

Relay MUST run one matching scheduler. Each matching round executes these steps in order:

1. Fetch queued tasks from the database ordered by `priority DESC, id ASC`, limited to `task_matching.batch_size` tasks. Expired queued tasks MUST be skipped and left to the timeout processor as specified in [task_timeout.md](./task_timeout.md).
2. Take the current node scheduling index state as the round's node view.
3. Iterate the fetched tasks in queue order. For each task:
   1. Compute the qualified node set from the index by applying the hard filters defined in [node_selection.md](./node_selection.md), including active health exclusion, and excluding nodes already reserved in this round. Every health calculation in one candidate-set computation MUST use the same current time.
   2. Decode the task's `RequiredModelFileHashes`, `RequiredModelFileSourceHashes`, and `RequestedModelCacheHash`. The task's `ModelIDs` MUST NOT participate in Relay matching.
   3. Retain only qualified nodes whose on-disk model files cover the required model files.
   4. If no model-ready node remains, leave the task queued and continue with the next task. Relay MUST NOT fall back to the qualified node set. The matching round MUST NOT emit `DownloadModel` events; model distribution is owned by the model distribution controller specified in [model_distribution.md](./model_distribution.md).
   5. Compute model-ready candidate weights and select one node by weighted random sampling as defined in [node_selection.md](./node_selection.md).
   6. Reserve the selected node for this task for the remainder of the round.
4. If every fetched task is either matched or blocked and unreserved eligible nodes remain, fetch the next page of queued tasks and continue matching within the same round.
5. Execute `SetTaskStatusStarted` for all matched pairs. Executions MAY run concurrently across pairs. Before changing task or node state, task start MUST reload the node from the database and recompute effective health. If persistent health exclusion is active, task start MUST fail and leave the task `Queued` and node `Available`. If the persistent flag is set but effective health has reached the exit threshold, the transaction MUST set the task `Started`, set the node `Busy`, set the current task, and clear `health_excluded` atomically. The task start transaction MUST retain its optimistic status guards, trace snapshot capture, and metrics. Task start MUST resolve the exact model files of the execution and MUST store them on the task as `ExecutionModelFileHashes` in the same transaction that starts the task, as specified in [Execution Model Files](#execution-model-files). Task start MUST NOT emit model download events, create `NodeModel` rows, or modify the node's in-memory model state.
6. Release the reservation of every pair whose task start failed. The task remains queued and re-enters matching in a later round. The node's index entry MUST be resynced from the database before reuse.

Relay MUST start the next round immediately when the current round matched at least one task, and MUST otherwise wait `task_matching.tick_interval_seconds` before the next round.

### Priority and Reservation Semantics

Within a round, tasks select nodes in queue-priority order. A node reserved by a higher-priority task MUST NOT appear in the candidate set of any lower-priority task in the same round. Node contention between tasks is resolved only by this in-round reservation order; Relay MUST NOT hold a matched pair in a pending window and MUST NOT replace a matched pair with another task before the task start transaction runs.

Blocked tasks MUST NOT block the round: a task with no model-ready candidate is skipped for the round and lower-priority tasks continue matching, so eligible idle nodes are never left unused behind blocked higher-priority tasks.

### Blocked-Task Handling

A task with qualified nodes but zero model-ready nodes stays queued and re-enters matching in later rounds. Its demand is visible to the model distribution controller through the task records defined in [model_distribution.md](./model_distribution.md); the matching scheduler itself MUST NOT select download target nodes, persist download selection state, or emit `DownloadModel` events.

### Sampling Semantics

Node selection remains weighted random sampling per task. Within a round, sampling is without replacement across tasks because reserved nodes are excluded. Across rounds, every unmatched task re-samples independently from the then-current candidate set. The matching scheduler MUST NOT convert selection into deterministic assignment such as highest-weight-first or best-fit pairing.

### Candidate Set Reuse

Within a round, tasks that share the same requirement signature MUST be served from one computed candidate set. Weighted sampling MUST still run per task against that shared set minus nodes reserved earlier in the round.

## Execution Model Files

When a task starts, Relay MUST resolve the exact model files that the selected node executes the task with, from the node's authoritative on-disk model file inventory and its in-memory model:

- An exact required hash MUST resolve to itself and MUST be present in the node's on-disk inventory.
- A source required hash MUST resolve to the node's on-disk file of that source that is also a member of the node's in-memory model when one exists. Otherwise it MUST resolve to the on-disk file of that source with the smallest exact hash.

The resolved hashes MUST be stored sorted and without duplicates as `ExecutionModelFileHashes`. They are immutable after task start. Task start MUST fail without changing task or node state when a required file is not present in the node's on-disk inventory.

A task start MUST record `model_swtiched` as false only when the node's in-memory model has the task's `RequestedModelCacheHash` and its model files cover the required model files. Otherwise it MUST record true.

The task APIs MUST expose the execution model files as exact entries (`execution_model_files`), the hex `requested_model_cache_hash`, and, on the single-task fetch, the canonical `requested_model_cache_config` from which the hash is computed. The node echoes this identity when it reports its in-memory model.

## Consistency Requirements

The index is advisory. The task start transaction keeps its existing guards: the conditional task status update, the node availability pre-check, the database health-exclusion recheck, and the status-conditional node update. A matching decision that loses a race with a concurrent node state or health change MUST fail at the database and MUST NOT be forced through. Task state, node status, current task, execution model files, and exclusion clear MUST commit or roll back together.

The matching scheduler, the node scheduling index, and the in-round reservation set are process-local structures. This design requires the single-process Relay deployment that the existing in-memory caches already assume.

## Dispatch Path Exclusivity

The matching scheduler is the only task dispatch path. Relay MUST NOT run per-task dispatch loops that independently query the database for candidate nodes, and MUST NOT query the node table with model preloading as part of per-task dispatch attempts. Candidate node data at dispatch time MUST come from the node scheduling index.

## Configuration

Relay MUST define these settings in every runtime configuration template:

- `task_matching.batch_size`
- `task_matching.tick_interval_seconds`

## Trace and Metrics Visibility

Successful matches MUST record the node selection trace data specified in [task_tracing.md](./task_tracing.md), including the final model-ready candidate pool used by weighted sampling. Node selection metrics (candidate pool size, empty-selection counter) and dispatch metrics (dispatched counter, queue wait histogram) MUST continue to be reported from the matching path.

## Relevant Source Files

| File | Responsibility |
|------|----------------|
| `service/start_task.go` | Dispatch scheduler entry point and timeout processor startup |
| `service/task_matching.go` | Qualification filters, model file gate, weight calculation, and weighted sampling |
| `service/node_model_state.go` | Task model requirements, model file coverage, execution model file resolution, and node model state updates |
| `service/selecting_prob.go` | Staking score, QoS combination, and max staking cache |
| `service/task_status.go` | Task start transaction and its optimistic guards |
| `service/node.go` | Node status transitions (`nodeStartTask`, `nodeFinishTask`, join, quit, kickout, slash) |
| `service/node_name_policy.go` | Node name policy caches used during candidate filtering |
| `api/v1/nodes/add_model_id.go` | Node-reported on-disk model file registration |
| `api/v2/nodes/in_memory_model.go` | Node-reported in-memory model |
| `models/node.go` | Node fields and status-conditional update guard |
