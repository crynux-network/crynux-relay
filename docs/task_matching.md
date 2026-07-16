# Task Matching

This document specifies how Relay matches queued inference tasks to execution nodes.

## Scope

Task matching covers the dispatch path between the task queue and the task start transaction. Queue ordering and task priority are specified in [task-pricing.md](./task-pricing.md). Candidate filtering rules, weight formulas, and sampling semantics are specified in [node_selection.md](./node_selection.md). This document specifies where that data comes from at dispatch time and how tasks and nodes are paired.

## Definitions

- Node scheduling index: the in-memory view of node state used by the matching scheduler.
- Matching round: one scheduler iteration that pairs a batch of queued tasks with nodes.
- Reservation: the in-round exclusive hold of a node for one matched task before the task start transaction commits.
- Requirement signature: the tuple (`TaskType`, `MinVRAM`, `RequiredGPU`, `RequiredGPUVRAM`, `TaskVersion`, normalized `ModelIDs`) that determines a task's candidate set.
- Qualified node: a node that passes availability, hardware, version, task-specific, and node-name policy filters.
- Base-ready node: a qualified node whose on-disk model set contains the task's base model ID. Every inference task requires exactly one base model, the single `base:` entry of `ModelIDs`; all other entries are auxiliary `lora:` and `controlnet:` models.

## Node Scheduling Index

Relay MUST maintain one in-memory node scheduling index covering every non-Quit node. Each index entry MUST contain the fields required by candidate filtering and weight calculation:

- node address
- node status
- current task occupancy (`current_task_id_commitment`)
- GPU name, GPU VRAM
- node version (`major.minor.patch`)
- on-disk model ID set
- in-use model ID set
- `QOSScore`, `HealthBase`, `HealthUpdatedAt`
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
- task start (`nodeStartTask`): status, occupancy, and in-use model set
- task finish (`nodeFinishTask`): status, occupancy, and deferred status transitions
- timeout abort completion
- node model registration (add-model API)
- node health penalty and health boost
- node QoS score update
- stake amount changes

When a task start transaction fails with a node status conflict, Relay MUST re-read that node from the database and replace its index entry before the node participates in another matching round.

## Matching Round

Relay MUST run one matching scheduler. Each matching round executes these steps in order:

1. Fetch queued tasks from the database ordered by `priority DESC, id ASC`, limited to `task_matching.batch_size` tasks. Expired queued tasks MUST be skipped and left to the timeout processor as specified in [task_timeout.md](./task_timeout.md).
2. Take the current node scheduling index state as the round's node view.
3. Iterate the fetched tasks in queue order. For each task:
   1. Compute the qualified node set from the index by applying the hard filters defined in [node_selection.md](./node_selection.md), excluding nodes already reserved in this round.
   2. Extract the task's single lowercase `base:` model ID. Auxiliary IDs MUST NOT participate in Relay matching.
   3. Retain only qualified nodes that have the task's base model on disk.
   4. If no base-ready node remains, leave the task queued and continue with the next task. Relay MUST NOT fall back to the qualified node set. The matching round MUST NOT emit `DownloadModel` events; model distribution is owned by the model distribution controller specified in [model_distribution.md](./model_distribution.md).
   5. Compute base-ready candidate weights and select one node by weighted random sampling as defined in [node_selection.md](./node_selection.md).
   6. Reserve the selected node for this task for the remainder of the round.
4. If every fetched task is either matched or blocked and unreserved eligible nodes remain, fetch the next page of queued tasks and continue matching within the same round.
5. Execute `SetTaskStatusStarted` for all matched pairs. Executions MAY run concurrently across pairs. The task start transaction MUST retain its optimistic status guards, trace snapshot capture, and metrics. Task start MUST NOT emit model download events or create `NodeModel` rows; it MUST only update the in-use state of existing node-reported base-model rows.
6. Release the reservation of every pair whose task start failed. The task remains queued and re-enters matching in a later round. The node's index entry MUST be resynced from the database before reuse.

Relay MUST start the next round immediately when the current round matched at least one task, and MUST otherwise wait `task_matching.tick_interval_seconds` before the next round.

### Priority and Reservation Semantics

Within a round, tasks select nodes in queue-priority order. A node reserved by a higher-priority task MUST NOT appear in the candidate set of any lower-priority task in the same round. Node contention between tasks is resolved only by this in-round reservation order; Relay MUST NOT hold a matched pair in a pending window and MUST NOT replace a matched pair with another task before the task start transaction runs.

Blocked tasks MUST NOT block the round: a task with no base-ready candidate is skipped for the round and lower-priority tasks continue matching, so eligible idle nodes are never left unused behind blocked higher-priority tasks.

### Blocked-Task Handling

A task with qualified nodes but zero base-ready nodes stays queued and re-enters matching in later rounds. Its demand is visible to the model distribution controller through the task records defined in [model_distribution.md](./model_distribution.md); the matching scheduler itself MUST NOT select download target nodes, persist download selection state, or emit `DownloadModel` events.

### Sampling Semantics

Node selection remains weighted random sampling per task. Within a round, sampling is without replacement across tasks because reserved nodes are excluded. Across rounds, every unmatched task re-samples independently from the then-current candidate set. The matching scheduler MUST NOT convert selection into deterministic assignment such as highest-weight-first or best-fit pairing.

### Candidate Set Reuse

Within a round, tasks that share the same requirement signature MUST be served from one computed candidate set. Weighted sampling MUST still run per task against that shared set minus nodes reserved earlier in the round.

## Consistency Requirements

The index is advisory. The task start transaction keeps its existing guards: the conditional task status update, the node availability pre-check, and the status-conditional node update. A matching decision that loses a race with a concurrent node state change MUST fail at the database and MUST NOT be forced through.

The matching scheduler, the node scheduling index, and the in-round reservation set are process-local structures. This design requires the single-process Relay deployment that the existing in-memory caches already assume.

## Dispatch Path Exclusivity

The matching scheduler is the only task dispatch path. Relay MUST NOT run per-task dispatch loops that independently query the database for candidate nodes, and MUST NOT query the node table with model preloading as part of per-task dispatch attempts. Candidate node data at dispatch time MUST come from the node scheduling index.

## Configuration

Relay MUST define these settings in every runtime configuration template:

- `task_matching.batch_size`
- `task_matching.tick_interval_seconds`

## Trace and Metrics Visibility

Successful matches MUST record the node selection trace data specified in [task_tracing.md](./task_tracing.md), including the final base-ready candidate pool used by weighted sampling. Node selection metrics (candidate pool size, empty-selection counter) and dispatch metrics (dispatched counter, queue wait histogram) MUST continue to be reported from the matching path.

## Relevant Source Files

| File | Responsibility |
|------|----------------|
| `service/start_task.go` | Dispatch scheduler entry point and timeout processor startup |
| `service/task_matching.go` | Qualification filters, base-model gate, weight calculation, and weighted sampling |
| `service/selecting_prob.go` | Staking score, QoS combination, and max staking cache |
| `service/task_status.go` | Task start transaction and its optimistic guards |
| `service/node.go` | Node status transitions (`nodeStartTask`, `nodeFinishTask`, join, quit, kickout, slash) |
| `service/node_name_policy.go` | Node name policy caches used during candidate filtering |
| `api/v1/nodes/add_model_id.go` | Node-reported on-disk model registration |
| `models/node.go` | Node fields and status-conditional update guard |
