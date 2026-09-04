# Quality of Service (QoS) Implementation

This document describes the QoS implementation in the Crynux Relay codebase.

## Overview

The QoS (Quality of Service) system is designed to improve network service quality by rewarding nodes that deliver faster execution and reliable availability, while reducing the impact of nodes that frequently fail or time out.

The QoS score is updated continuously as a node executes tasks and is used as an input to network decisions such as task allocation preference and reliability protection mechanisms.

At its core, QoS is intentionally split across two time scales: a long-term performance component derived from task completion speed (`Q_long`), and a short-term reliability component (`H`) that reacts quickly to timeouts but can recover after sustained success.

## Naming Clarification (Code vs Concepts)

In the codebase, "QoS" may refer to either the persisted long-term performance score or the final runtime QoS used for selection. The mapping below is the source of truth:

| Code Symbol | Actual Meaning |
|-------------|----------------|
| `models.Node.QOSScore` | `Q_long`, the persisted long-term performance score (not the final runtime QoS) |
| `CalculateQosScore(qosScore, healthBase, healthUpdatedAt)` parameter `qosScore` | `Q_long` loaded from `models.Node.QOSScore` |
| `CalculateQosScore(...)` return value | final runtime QoS used for selection |

In short: the database field named `QOSScore` stores long-term performance only, while the final runtime QoS is computed by combining normalized `Q_long` with effective health `H`.

## QoS Score Definition

The final runtime QoS score evaluates node quality through two factors that operate at different time scales:

- **Long-term performance factor**: A rolling average of recent validation task scores that captures whether a node is consistently fast.
- **Short-term reliability factor**: A multiplier that reacts immediately to timeout failures, capturing whether a node is currently dependable.

The final QoS score for a node is the product of both factors:

```
QoS = (Q_long / Q_max) * H
```

Where:
- `Q_long`: the node's long-term performance score (rolling average of task scores).
- `Q_max`: the maximum possible task score (10.0).
- `H`: the short-term reliability factor (range 0 to 1).

Relay MUST persist `Q_long = 5.0` when a node joins for the first time. Normalization MUST only divide the persisted value by `Q_max`. A persisted `Q_long` of `0` is a real zero score and MUST normalize to `0`; it MUST NOT be replaced with an initialization default.

## Factor Calculation

This section explains how `Q_long` and `H` are computed.
### Long-term Performance Score (`Q_long`)

`Q_long` measures a node's sustained execution speed across its recent validation tasks. It changes gradually and reflects the node's typical hardware and network quality.

#### Task Grouping (Validation Tasks)

Not all tasks contribute to `Q_long`. Only **grouped tasks** (validation tasks) receive Task Scores that enter the rolling pool. A grouped task is executed by **3 different nodes** simultaneously. Single (non-grouped) tasks do not generate Task Scores for `Q_long` (though they do influence the short-term reliability factor via successful completion or timeout).

#### Task Score

When a grouped task is validated, each of the 3 tasks in the group receives a **Task Score** based on execution speed (SubmissionTime - StartTime).

Tasks within a group are sorted by execution time (fastest first). The fixed score values are:

| Completion Order | Task Score |
|-----------------|------------|
| 1st (fastest)   | 10         |
| 2nd             | 5          |
| 3rd (slowest)   | 2          |

Special cases:
- A task already in `TaskEndAborted` when an otherwise permitted group validation begins MUST receive a score of **0**.
- A validation-group task aborted due to `TaskAbortTimeout` MUST contribute that **0** score to the selected node's rolling long-term QoS average when the same group contains at least one non-aborted task.
- If **all 3 tasks** in a group are aborted, QoS scores are set to NULL (not valid) and are **not included** in any node's rolling average.
- If any group member has `TaskAbortCreatorValidationTimeout`, Relay MUST permanently reject validation of the group. No member of that group MUST receive a validation rank, task QoS score, or `Q_long` update.

#### Rolling Pool Mechanism

The long-term score (`Q_long`) is calculated using an in-memory rolling pool:

- **Pool size**: Configurable via `qos.score_pool_size` (default: 50 tasks)
- The pool is stored per node address in a concurrent-safe map (`NodeQosScorePool`).
- When a new task score arrives, it is appended to the pool. If the pool exceeds the configured size, the oldest entry is removed.
- `Q_long` is the **arithmetic mean** of all scores in the pool.

When a node's pool does not exist in memory, Relay MUST build the pool when the next valid task score arrives. If persisted `QOSScore` is greater than `0`, Relay MUST insert `score_pool_size - 1` copies of that persisted score before appending the new task score. Later task scores MUST replace these initial entries from oldest to newest. For a new node with persisted `QOSScore = 5.0`, the first pool therefore contains `score_pool_size - 1` entries of `5` and one real task score.

When an existing node rejoins with normalized `Q_long` below `qos.rejoin_qos_long_floor`, Relay MUST raise persisted `QOSScore` to `qos.rejoin_qos_long_floor * Q_max` and discard the node's in-memory rolling pool. This rule MUST apply when persisted `QOSScore` is `0`. The next valid task score MUST rebuild the pool with `score_pool_size - 1` copies of the raised score and one real task score, so scores recorded before kickout no longer contribute to `Q_long`. Relay MUST leave the score and pool unchanged when normalized `Q_long` is equal to or above the floor.

### Short-term Reliability Factor (`H`)

The short-term factor (`H`) addresses the need to immediately penalize nodes that start timing out, protecting applications from unreliable nodes.

Each node carries a **health multiplier** `H` (range 0.0 to 1.0, default 1.0).

#### Penalty on Node-Attributed Timeout

Relay MUST apply the health penalty when an assigned task expires during node execution with `TaskAbortTimeout` or during result upload with `TaskAbortResultUploadTimeout`. Relay MUST NOT apply it for a queued `TaskAbortTimeout` or `TaskAbortCreatorValidationTimeout`.

For each node-attributed timeout, Relay MUST update the health multiplier by this two-stage rule based on current effective health:

```
if H_effective >= FirstTimeoutHealthThreshold:
    H_new = H_effective * FirstTimeoutPenaltyFactor
else:
    H_new = H_effective * PenaltyFactor
```

With default config values:
- `FirstTimeoutPenaltyFactor = 0.95`
- `FirstTimeoutHealthThreshold = 0.99`
- `PenaltyFactor = 0.3` (heavy penalty for repeated timeout state)

Default behavior example:
- 1 timeout from full health: `1.00 -> 0.95` (light penalty)
- 2nd consecutive timeout: `0.95 -> 0.285` (heavy penalty begins)

#### Health Exclusion

When a node-attributed timeout produces `H_new < qos.health_exclude_enter_threshold`, Relay MUST set `health_excluded = true` in the same node update that writes `HealthBase` and `HealthUpdatedAt`. Equality with the enter threshold MUST NOT set the flag.

A node with `health_excluded = true` and effective health below `qos.health_exclude_exit_threshold` MUST remain `Available` but MUST be removed from inference-task candidate sets by a hard filter. Relay MUST NOT rely on zero selection weight because zero-weight sampling can still select a node when every candidate has zero weight. This exclusion MUST NOT prevent model download selection.

When effective health reaches or exceeds the exit threshold, the node is eligible for inference matching. Before task start, Relay MUST reload the node and recompute effective health. If health remains below the exit threshold, task start MUST fail and leave the task `Queued` and the node `Available`. If health has reached the exit threshold, the task-start transaction MUST set the node `Busy`, set its current task, and clear `health_excluded` in one node update. Join and rejoin MUST reset `HealthBase` to `1.0` and `health_excluded` to `false`.

Short-term health MUST NOT cause permanent kickout. Permanent kickout MUST use only the long-term `Q_long` threshold after the configured rolling pool contains enough samples.

#### Recovery

The penalty is temporary. Health recovers via two mechanisms:

1. **Passive time-based recovery**: Exponential decay toward 1.0 with the configured time constant.
   ```
   H(t) = H_base + (1 - H_base) * (1 - exp(-elapsed / recovery_tau_minutes))
   ```
2. **Active success-based recovery**: Every successfully completed task adds a boost of `0.15` to H.

Relay MUST apply the active success boost only after validated result upload succeeds. `TaskAbortCreatorValidationTimeout` MUST NOT apply this boost even though its fee uses the successful-task distribution function.

An excluded node cannot complete inference tasks and therefore cannot receive a success boost while exclusion is active. For an exclusion entered at `H_new`, passive recovery reaches the exit threshold after:

```
duration_minutes =
    recovery_tau_minutes
    * ln((1 - H_new) / (1 - health_exclude_exit_threshold))
```

Relay MUST NOT use a cooldown field, timer, or background cleanup to end exclusion. `qos.recovery_tau_minutes` MUST be positive, and code MUST NOT substitute a fallback value.

## QoS Tracing

Relay MUST keep an in-memory `qos_tracing` event list for each node address. The trace list is diagnostic data only and MUST NOT affect QoS calculation, task selection, node status, kickout decisions, or persisted task and node records.

Relay MUST retain only the newest `qos.tracing_max_task_events` trace events for each node address. When a node trace list exceeds this limit, Relay MUST remove the oldest events for that node. Trace events are process-local and MUST be lost after Relay restarts.

Relay MUST record a trace event only for explicit QoS mutation events. Relay MUST NOT record passive time-based health recovery, because passive recovery is calculated at read time and does not mutate QoS state.

Each trace event MUST include:

- `timestamp`
- `node_address`
- `event_type`
- `qos_long_before`
- `qos_long_after`
- `qos_short_before`
- `qos_short_after`
- `qos_before`
- `qos_after`

Task-related trace events MUST include `task_id_commitment`. Validation-group rank events MUST include `task_qos_score` and `validation_rank`. Validation-group aborted trace events MUST include `task_qos_score` with value `0`.

### Trace Event Types

Relay MUST use the following `event_type` values:

| Event Type | Trigger |
|------------|---------|
| `validation_group_rank_1` | A validation-group task receives rank 1 and task QoS score 10, then updates `Q_long`. |
| `validation_group_rank_2` | A validation-group task receives rank 2 and task QoS score 5, then updates `Q_long`. |
| `validation_group_rank_3` | A validation-group task receives rank 3 and task QoS score 2, then updates `Q_long`. |
| `validation_group_aborted` | A validation-group task receives the special abort task QoS score 0, then updates `Q_long`. |
| `task_timeout_penalty` | `TaskAbortTimeout` during node execution or `TaskAbortResultUploadTimeout` applies the short-term health penalty. |
| `task_result_upload_success_boost` | A validated task result upload succeeds and applies the short-term health boost. |
| `validation_group_matched_boost` | A validation-group comparison accepts the node result and applies the short-term health boost. |
| `node_join_health_reset` | Node join or rejoin resets short-term health to full health. |
| `node_rejoin_qos_floor` | Existing node rejoin raises `Q_long` to the configured rejoin floor. |

`TaskAbortCreatorValidationTimeout` MUST NOT produce `validation_group_aborted`, a rank event, `task_timeout_penalty`, or `task_result_upload_success_boost`.

### Trace API

Relay MUST expose node QoS trace events through:

```
GET /v2/node/:address/qos/tracing
```

The API MUST allow access when either authentication method succeeds:

- JWT authentication: the JWT address MUST equal `:address`.
- Signature authentication: the recovered signer address MUST equal `:address`.

The API response `data` object MUST contain `node_address`, `max_task_events`, and `events`. The `events` array MUST contain the retained in-memory trace events for the requested node address.

## Key Constants and Config

| Constant / Config | Value | Description |
|-------------------|-------|-------------|
| `TASK_SCORE_REWARDS` | [10, 5, 2] | Task scores for 1st, 2nd, 3rd place |
| `maxQoSScore` | 10.0 | Fixed normalization denominator for QoS score |
| New node `QOSScore` | 5.0 | Persisted initial long-term score; normalized value is 0.5 |
| `qos.score_pool_size` | 50 | Rolling pool size for node QoS calculation |
| `qos.rejoin_qos_long_floor` | 0.3 | Minimum normalized long-term score applied when an existing node rejoins below the floor |
| `qos.tracing_max_task_events` | 50 | Maximum number of in-memory QoS trace events retained per node |
| `qos.penalty_factor` | 0.3 | Heavy timeout multiplier applied to H after first-timeout condition is no longer met |
| `qos.first_timeout_penalty_factor` | 0.95 | Light timeout multiplier applied when node health is near full |
| `qos.first_timeout_health_threshold` | 0.99 | Health threshold that determines whether timeout uses light or heavy penalty |
| `qos.success_boost` | 0.15 | Additive boost to H on success |
| `qos.recovery_tau_minutes` | 30 | Time constant used for passive health recovery |
| `qos.health_exclude_enter_threshold` | 0.2 | Strict H threshold below which a timeout sets persistent health exclusion |
| `qos.health_exclude_exit_threshold` | 0.8 | H threshold at or above which an excluded node becomes eligible for task start |

Relay configuration MUST satisfy:

- `0 < health_exclude_enter_threshold < health_exclude_exit_threshold <= 1`
- `0 < penalty_factor <= 1`
- `0 < first_timeout_penalty_factor <= 1`
- `0 < first_timeout_health_threshold <= 1`
- `0 <= success_boost <= 1`
- `recovery_tau_minutes > 0`

The Admin node QoS CSV MUST include `health_excluded` from persistent node state and `health_exclusion_active`, calculated as `health_excluded && effective_H < health_exclude_exit_threshold` at export time.

## Relevant Source Files

| File | Description |
|------|-------------|
| `service/qos.go` | Core QoS logic: long-term pool, short-term health (H), combined score calculation (`CalculateQosScore`) |
| `service/qos_tracing.go` | In-memory QoS tracing event store, trace event types, retention, and API read helpers |
| `service/task_status.go` | Task state transitions, QoS updates, health penalty/boost triggers |
| `models/node.go` | Node model with `QOSScore` (long-term), `HealthBase`, `HealthUpdatedAt`, and `HealthExcluded` |
