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

If `Q_long` is 0 (e.g., for a new node), it defaults to `5.0` (half of `Q_max`) before applying `H`.

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
- A task in a validation group that reached `TaskEndAborted` before group validation receives a score of **0**.
- A validation-group task aborted due to `TaskAbortTimeout` MUST contribute that **0** score to the selected node's rolling long-term QoS average when the same group contains at least one non-aborted task.
- If **all 3 tasks** in a group are aborted, QoS scores are set to NULL (not valid) and are **not included** in any node's rolling average.

#### Rolling Pool Mechanism

The long-term score (`Q_long`) is calculated using an in-memory rolling pool:

- **Pool size**: Configurable via `qos.score_pool_size` (default: 50 tasks)
- The pool is stored per node address in a concurrent-safe map (`NodeQosScorePool`).
- When a new task score arrives, it is appended to the pool. If the pool exceeds the configured size, the oldest entry is removed.
- `Q_long` is the **arithmetic mean** of all scores in the pool.

When a node's pool does not yet exist in memory (e.g., after a relay restart), the pool is initialized from persisted `QOSScore` in the database (`models.Node.QOSScore`, which stores `Q_long`) to ensure smooth transition.

### Short-term Reliability Factor (`H`)

The short-term factor (`H`) addresses the need to immediately penalize nodes that start timing out, protecting applications from unreliable nodes.

Each node carries a **health multiplier** `H` (range 0.0 to 1.0, default 1.0).

#### Penalty on Timeout

When a task assigned to the node ends with a timeout, the health multiplier is penalized by a two-stage rule based on current effective health:

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

#### Health-Based Kickout

When a node's effective health drops below the **health kickout threshold** (`0.1`), the relay MUST kick the node out when the current task finishes. The node leaves the candidate set because its status becomes `Quit`, not because runtime QoS is clamped to zero. Under default penalty settings, this occurs after the heavy-penalty stage drives health below the threshold.

#### Recovery

The penalty is temporary. Health recovers via two mechanisms:

1. **Passive time-based recovery**: Exponential decay toward 1.0 with a 30-minute time constant.
   ```
   H(t) = H_base + (1 - H_base) * (1 - exp(-elapsed / 30min))
   ```
2. **Active success-based recovery**: Every successfully completed task adds a boost of `0.15` to H.

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
| `task_timeout_penalty` | A task timeout abort applies the short-term health penalty. |
| `task_result_upload_success_boost` | A validated task result upload succeeds and applies the short-term health boost. |
| `validation_group_matched_boost` | A validation-group comparison accepts the node result and applies the short-term health boost. |
| `node_join_health_reset` | Node join or rejoin resets short-term health to full health. |
| `node_rejoin_qos_floor` | Existing node rejoin raises `Q_long` to the configured rejoin floor. |

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
| `qos.score_pool_size` | 50 | Rolling pool size for node QoS calculation |
| `qos.tracing_max_task_events` | 50 | Maximum number of in-memory QoS trace events retained per node |
| `qos.penalty_factor` | 0.3 | Heavy timeout multiplier applied to H after first-timeout condition is no longer met |
| `qos.first_timeout_penalty_factor` | 0.95 | Light timeout multiplier applied when node health is near full |
| `qos.first_timeout_health_threshold` | 0.99 | Health threshold that determines whether timeout uses light or heavy penalty |
| `qos.success_boost` | 0.15 | Additive boost to H on success |
| `qos.recovery_tau_minutes` | 30 | Time constant used for passive health recovery |
| `qos.health_kickout_threshold` | 0.1 | H value below which the node is kicked out when the current task finishes |

## Relevant Source Files

| File | Description |
|------|-------------|
| `service/qos.go` | Core QoS logic: long-term pool, short-term health (H), combined score calculation (`CalculateQosScore`) |
| `service/qos_tracing.go` | In-memory QoS tracing event store, trace event types, retention, and API read helpers |
| `service/task_status.go` | Task state transitions, QoS updates, health penalty/boost triggers |
| `models/node.go` | Node model with `QOSScore` (long-term), `HealthBase`, `HealthUpdatedAt` |
