# Task Validation and Node Slashing

This document describes the task validation and node slashing implementation in the Crynux Relay codebase.

## Overview

The relay implements a Verifiable Secret Sampling (VSS) consensus protocol to ensure that nodes honestly execute AI tasks. A small percentage of tasks are randomly selected for validation, where the same task is sent to 3 independent nodes and their results are cross-compared. Nodes that submit incorrect results are slashed: their staked tokens are confiscated via a blockchain smart contract call.

The design relies on three cryptographic building blocks:

- **Task ID Commitment**: hides the real task identity until validation time, preventing nodes from knowing whether their task is part of a validation group.
- **VRF (Verifiable Random Function)**: determines, in a publicly verifiable but initially secret way, whether a task will be validated.
- **Perceptual hashing / SHA-256**: provides the similarity metric used to compare results across nodes.

## Task ID Commitment Scheme

When a client creates a task, it does not reveal the real `TaskID`. Instead it provides:

| Field | Description |
|-------|-------------|
| `TaskIDCommitment` | `Keccak256(TaskID \|\| Nonce)` |
| `Nonce` | A random value chosen by the creator |

The real `TaskID` stays hidden until the creator calls the validation endpoint. At that point the relay verifies:

```
Keccak256(TaskID || Nonce) == TaskIDCommitment
```

This ensures that nodes cannot tell whether two concurrent tasks share the same `TaskID` (and therefore belong to the same validation group) until validation is explicitly triggered.

## VRF-Based Task Sampling

### Sampling Seed Generation

When a task is created via the `CreateTask` API, the relay generates a cryptographically random 32-byte `SamplingSeed` and stores it alongside the task. This seed is returned to the client and later used as the VRF input.

### Validation Decision

At validation time, the task creator provides a VRF proof computed over the `SamplingSeed` using their private key. The relay:

1. Parses the creator's public key and verifies that the corresponding Ethereum address matches the task creator.
2. Calls `ecvrf.Secp256k1Sha256Tai.Verify` to validate the VRF proof and obtain the VRF output (`beta`).
3. Determines whether the task is selected for validation:

```
needValidation = (beta mod 100) == 0
```

This gives a **1% sampling rate**: roughly 1 in 100 tasks is selected for cross-validation.

### Single vs Grouped Tasks

| Scenario | Task Count | VRF Result | Validation Type |
|----------|-----------|------------|-----------------|
| Not selected | 1 | `beta mod 100 != 0` | Single task validation |
| Selected | 3 | `beta mod 100 == 0` | Group validation (cross-comparison) |

## Task Lifecycle

The full state machine for an inference task:

```
TaskQueued
  → TaskStarted              (node selected, task dispatched)
    → TaskScoreReady          (node submitted result hash)
    → TaskErrorReported       (node reported execution error)
  → TaskEndAborted            (timeout before node selection)

TaskScoreReady / TaskErrorReported
  → TaskValidated             (single task, VRF confirms no validation needed)
  → TaskGroupValidated        (group task, result matches majority)
  → TaskEndInvalidated        (group task, result does not match majority → SLASH)
  → TaskEndGroupRefund        (group task, result matches but task fee refunded)
  → TaskEndAborted            (group task, no majority found)

TaskValidated / TaskGroupValidated
  → TaskEndSuccess            (single task, result uploaded to client)
  → TaskEndGroupSuccess       (group task, result uploaded to client)
```

### Key Timestamps

| Field | Meaning |
|-------|---------|
| `CreateTime` | Task creation time |
| `StartTime` | Node began execution |
| `ScoreReadyTime` | Node submitted the result score/hash |
| `ValidatedTime` | Relay completed validation |
| `ResultUploadedTime` | Result file delivered to client |

## Score Submission

After executing a task, the node submits a **score** (result fingerprint) rather than the full result:

- **SD / SD Fine-tune LoRA tasks**: The score is a perceptual hash (pHash) of the generated image(s). Each pHash is an 8-byte block; multiple images produce concatenated blocks.
- **LLM tasks**: The score is the SHA-256 hash of the full text response.

The score is submitted via the `SubmitScore` API, which transitions the task to `TaskScoreReady`.

## Validation Logic

### Single Task Validation (`ValidateSingleTask`)

For tasks where the VRF confirms no validation is needed (single task):

1. Verify the `TaskID` against the stored `TaskIDCommitment`.
2. Verify the VRF proof to confirm the task was correctly classified as non-grouped.
3. If the task status is `TaskScoreReady` → transition to `TaskValidated`.
4. If the task status is `TaskErrorReported` → abort with reason `TaskAbortIncorrectResult`.

### Group Task Validation (`ValidateTaskGroup`)

For tasks selected for validation (group of 3 tasks sharing the same real `TaskID`):

1. Verify all 3 `TaskIDCommitment` values against the revealed `TaskID`.
2. Verify the VRF proof to confirm the task was correctly classified as grouped.
3. Sort non-aborted tasks by execution time (fastest first) and assign QoS scores: 1st = 10, 2nd = 5, 3rd = 2. Tasks already in `TaskEndAborted` receive 0.
4. Compare results pairwise to determine the majority.

### Result Comparison

The comparison method depends on task type:

| Task Type | Method | Match Condition |
|-----------|--------|-----------------|
| SD / SD Fine-tune LoRA | Hamming distance on pHash blocks | Distance < `DistanceThreshold` for every 8-byte block |
| LLM | Exact string comparison | Score strings are identical |

The `DistanceThreshold` is configured via `task.distance_threshold` in the application config.

### Group Validation Outcomes

Given 3 finished tasks (A, B, C), the relay compares all pairs and assigns terminal states:

| Matching Pattern | A | B | C |
|-----------------|---|---|---|
| All 3 match (A=B, A=C, B=C) | `GroupValidated` | `GroupRefund` | `GroupRefund` |
| A=B only (C differs) | `GroupValidated` | `GroupRefund` | **`EndInvalidated`** |
| A=C only (B differs) | `GroupValidated` | **`EndInvalidated`** | `GroupRefund` |
| B=C only (A differs) | **`EndInvalidated`** | `GroupValidated` | `GroupRefund` |
| None match | `EndAborted` | `EndAborted` | `EndAborted` |
| All 3 aborted before scoring | QoS scores set to NULL, no long-term QoS update | | |

When only 2 of 3 tasks finished (the third was aborted before scoring):
- If the 2 finished tasks match → first gets `GroupValidated`, second gets `GroupRefund`
- If they do not match → both get `EndAborted`

Long-term QoS scoring for tasks already in `TaskEndAborted` follows these rules:
- If the group contains at least one non-aborted task, each task aborted due to `TaskAbortTimeout` MUST contribute a Task QoS score of `0` to its selected node's long-term QoS rolling average.
- If all 3 tasks in the group are already aborted, all 3 Task QoS scores MUST be treated as NULL and MUST NOT update any node's long-term QoS rolling average.

A task reaching `EndInvalidated` triggers the **node slash** for its assigned node through `SlashNode`.

### Payment Distribution in Groups

When a validation group completes, the task fee is distributed among validated nodes proportionally to their QoS scores:

```
payment_i = task_fee_i * qos_score_i / total_qos_score
```

Where `total_qos_score` is the sum of QoS scores across all valid tasks in the group. Remainder from integer division is added to the last valid task's payment.

Tasks in `GroupRefund` status have their task fee refunded to the creator since the task was a duplicate used purely for validation.

## Node Slashing

### When Slashing Occurs

A node is slashed when its submitted result does not match the majority in a validation group. Specifically, the task transitions to `TaskEndInvalidated`, which calls `SlashNode` with the offending task ID commitment.

The authenticated admin API `POST /v2/admin/nodes/slash` also calls `SlashNode`. Admin-triggered slash uses the node row's current network and does not have an offending task ID commitment, so the emitted `NodeSlashed` Relay event MUST use `0x` as the task ID commitment placeholder.

### Slash Execution Flow

1. **Node status** is set to `NodeStatusQuit`.
2. **All cached models** associated with the node are deleted from the database.
3. Active node vesting records for the node address are marked with `slashed = true`.
4. A **`NodeStaking::slashStaking`** blockchain transaction is queued when the node has active operator staking on its current blockchain network. This calls the `slashStaking` method on the `NodeStaking` smart contract and confiscates only the operator stake.
5. Two Relay events are emitted in order: `NodeQuit` with the blockchain transaction ID, then `NodeSlashed` with the offending task ID commitment or the admin slash placeholder.
6. After the `NodeStaking.NodeSlashed` chain event is confirmed, Relay MUST mark active node vesting records as slashed as an idempotent backstop and MUST create or resume the delegated slash job for that confirmed chain event.
7. The delegated slash job MUST queue bounded `DelegatedStaking::slashNodeDelegations` transactions. Each transaction MUST include no more than `blockchains.<network>.delegated_staking_slash_batch_size` delegator addresses.
8. Relay MUST process confirmed `DelegatedStaking.DelegatorSlashed` events as the source of truth for delegated slash progress. Relay MUST write one audit row per slashed delegator, mark only confirmed non-slashed delegation rows `slashed = true`, remove them from active delegation caches, and emit one generic `DelegatedStakingSlashed` Relay event per confirmed slashed delegator.
9. Relay MUST complete the delegated slash job only when the `DelegatedStaking` contract reports zero remaining delegations for the node.

Each confirmed `NodeStaking.NodeSlashed` chain event MUST have a distinct delegated slash job. Reprocessing the same chain event MUST resume the existing job for that event. A later slash of the same node address after a completed delegated slash job MUST create a new delegated slash job.

The node address MUST NOT join another blockchain network while any delegated slash job for that node address is not completed.

### Normal Quit, Recovery Quit, and Slashed Quit

| Scenario | Node-owner chain action | Relay smart contract call | Token outcome |
|----------|-------------------------|---------------------------|---------------|
| Normal node quit | `NodeStaking::tryUnstake` before Relay quit API | `NodeStaking::unstake` | Tokens returned to node |
| On-chain recovery quit | `NodeStaking::tryUnstake`, then `NodeStaking::forceUnstake` if Relay does not complete unstake | `NodeStaking::unstake` when Relay is available | Tokens returned to node |
| Slashed quit | None required | `NodeStaking::slashStaking` | Tokens confiscated |

Local quit completion is handled by `SetNodeStatusQuit`, differentiated by the `slashed` boolean parameter. The complete node quit, Relay admin unstake, on-chain recovery, and kickout flow is specified in `node_quit_and_unstake.md`.

## Task Timeout and Abort

Tasks can be aborted for several reasons:

| Abort Reason | Description |
|-------------|-------------|
| `TaskAbortTimeout` | Task exceeded its deadline (creation time + 3 minutes + configured timeout) |
| `TaskAbortModelDownloadFailed` | Model download failed on the node |
| `TaskAbortIncorrectResult` | Result failed validation |
| `TaskAbortTaskFeeTooLow` | Task fee was too low to attract eligible nodes |

`TaskAbortTaskFeeTooLow` is not assigned by any automatic relay task processing path in current implementation. It appears only when a caller explicitly submits `POST /v1/inference_tasks/:task_id_commitment/abort_reason` with `abort_reason = TaskAbortTaskFeeTooLow`.

When a task is aborted:
- The task fee is refunded to the creator.
- If the abort reason is `TaskAbortTimeout` and the node never submitted a score, a **health penalty** is applied to the node's short-term reliability factor. If the updated health is below `qos.health_kickout_threshold`, the relay MUST kick the node out when the current task finishes.

## Error Reporting

Nodes can report execution errors (e.g., invalid task parameters) via the `ReportTaskError` API. This transitions the task to `TaskErrorReported`. During group validation, if one node reports an error while the other two submit matching results, the error-reporting node is treated as having submitted an incorrect result and is invalidated (slashed).

## Configuration

| Config Key | Description |
|-----------|-------------|
| `task.stake_amount` | Required stake amount for joining the network (in ether) |
| `task.distance_threshold` | Maximum Hamming distance per 8-byte pHash block for SD result comparison |
| `qos.score_pool_size` | Number of task scores in the rolling QoS pool (default: 50) |
| `qos.kickout_threshold` | QoS score below which a node is permanently kicked out |
| `qos.health_kickout_threshold` | Health threshold below which a timeouted node is kicked out when the task finishes |

## Relevant Source Files

| File | Description |
|------|-------------|
| `service/validate_task.go` | Core validation logic: VRF verification, task ID commitment check, group result comparison |
| `service/task_status.go` | Task state transitions, slash trigger (`SetTaskStatusEndInvalidated`), abort handling |
| `service/node.go` | Node lifecycle: `SlashNode`, `nodeFinishTask`, `SetNodeStatusQuit` |
| `service/qos.go` | QoS scoring, health penalty/boost, permanent kickout check |
| `service/start_task.go` | Task queue processing and node dispatch |
| `service/select_nodes.go` | Node selection for task assignment (weighted by QoS and staking) |
| `blockchain/nodeStaking.go` | Blockchain interactions: `SlashStaking`, `QueueSlashStaking`, `Unstake`, `QueueUnstake` |
| `docs/node_quit_and_unstake.md` | Node quit, Relay admin unstake, on-chain recovery unstake, kickout, and slash precedence |
| `blockchain/task.go` | Perceptual hash and SHA-256 hash computation for result scoring |
| `models/inference_task.go` | Task model, status enum, abort reason enum |
| `models/node.go` | Node model with staking, health, and QoS fields |
| `models/event.go` | Event types: `NodeSlashed`, `NodeKickedOut`, `TaskEndInvalidated`, etc. |
| `utils/vrf.go` | VRF validation sampling decision (`VrfNeedValidation`) |
| `utils/hamming.go` | Hamming distance calculation for pHash comparison |
| `utils/commitment.go` | Task ID commitment utility |
| `api/v1/inference_tasks/validate_task.go` | Validation API endpoint |
| `api/v1/inference_tasks/submit_score.go` | Score submission API endpoint |
| `api/v1/inference_tasks/report_task_error.go` | Error reporting API endpoint |
| `api/v1/inference_tasks/create_task.go` | Task creation API endpoint |
| `config/app_config.go` | Configuration struct with task and QoS settings |
