# Portal Netstats Chart

This document specifies the Relay API implementation logic used by Portal netstats statistics.

## API Overview

| API | Function | Implementation |
|---|---|---|
| [`GET /v1/stats/line_chart/task_count`](#api-task-count) | Return time series of completed task count by task type and period. | Read pre-aggregated `task_counts` rows and sum `total_count` by time bucket. |
| [`GET /v1/stats/line_chart/task_success_rate`](#api-task-success-rate) | Return success-rate time series by task type and period. | Reuse `task_counts`, aggregate `success_count` and `total_count`, then compute `success_count / total_count`. |
| [`GET /v1/stats/histogram/task_execution_time`](#api-task-execution-time) | Return task execution-time histogram bins for elapsed time from `start_time` to `score_ready_time`. | Read pre-aggregated `task_execution_time_counts`, optionally filter by `model_switched`, then sum by `seconds` bin. |
| [`GET /v1/stats/histogram/task_fee`](#api-task-fee) | Return task-fee histogram for recent tasks. | Scan last-hour `inference_tasks` raw rows and build 10 logarithmic fee buckets. |
| [`GET /v1/stats/line_chart/incentive`](#api-incentive-line-chart) | Return incentive time series by day/week/month. | Build fixed intervals, map rows to interval index, and sum `node_incentives.incentive` per interval. |
| [`GET /v2/incentive/nodes`](#api-top-incentivized-nodes) | Return the current-day top 10 incentivized nodes. | Aggregate per-node incentive/task counters over a fixed one-day window and enrich with real-time node fields, scores, and effective stake. |
| [`GET /v2/incentive/delegations`](#api-top-delegations-by-task-fee) | Return the current UTC-day top 10 delegations by task fee. | Read the pre-aggregated `delegation_task_fee_leaderboard_snapshots` rows in rank order. |

## Shared Stats Pipeline

Relay starts background stats workers in `main.go`:

- `StartStatsTaskCount`
- `StartStatsTaskExecutionTimeCount`
- `StartStatsTaskUploadResultTimeCount`
- `StartStatsTaskWaitingTimeCount`

These workers run every 5 minutes and backfill hourly windows from `2025-01-01T00:00:00Z` to current time. APIs under `/v1/stats/*` consume either these pre-aggregated tables or raw task records depending on endpoint.

### Task Lifecycle Timestamps

The following `inference_tasks` timestamps define the task execution timeline and MUST be interpreted consistently across all stats endpoints:

- `create_time`:
  - The task creation time.
  - This timestamp marks when a task enters the relay workflow.
- `start_time`:
  - The task start time.
  - This timestamp marks when execution begins on the selected node.
- `score_ready_time`:
  - The score-ready time.
  - This timestamp marks when execution output is finalized and score data is available.
- `validated_time`:
  - The validation completion time.
  - This timestamp marks when task validation processing is completed.
- `result_uploaded_time`:
  - The result-upload completion time.
  - This timestamp marks when task result upload is completed.

Timestamp ordering for a normally completed task SHALL follow:

`create_time <= start_time <= score_ready_time <= validated_time <= result_uploaded_time`

If any stage has not been reached, the corresponding timestamp MUST remain `NULL` and MUST NOT be synthesized by stats logic.

## Detailed Implementation

<a id="api-task-count"></a>
### API: `GET /v1/stats/line_chart/task_count`

- Handler: `api/v1/stats/task_count.go:GetTaskCountLineChart`
- Inputs:
  - `task_type`: `All|Image|Text`
  - `period`: `Hour|Day|Week`
  - optional `end`, optional `count`
- Window and bucket count:
  - `Hour`: default 24 buckets, 1 hour each
  - `Day`: default 15 buckets, 1 day each
  - `Week`: default 8 buckets, 7 days each
- Data source:
  - `models.TaskCount` (`task_counts`)
- Count semantics:
  - Counts completed tasks only (not submitted tasks).
  - Completed tasks are rows with terminal statuses:
    - `TaskEndSuccess`
    - `TaskEndGroupSuccess`
    - `TaskEndGroupRefund`
    - `TaskEndAborted`
    - `TaskEndInvalidated`
  - Excluded statuses (not completed yet) are:
    - `TaskQueued`
    - `TaskStarted`
    - `TaskParametersUploaded`
    - `TaskErrorReported`
    - `TaskScoreReady`
    - `TaskValidated`
    - `TaskGroupValidated`
- Aggregation:
  - Query rows where `start >= window_start` and `start < window_end`
  - Apply task type filter
  - Sum `total_count` per truncated bucket timestamp
  - Return `{timestamps[], counts[]}` in ascending time order
- Upstream producer:
  - `tasks/stats.go:getTaskCounts`
  - `success_count` includes `TaskEndSuccess`, `TaskEndGroupSuccess`, and `TaskEndGroupRefund`
  - `aborted_count` includes `TaskEndAborted` and `TaskEndInvalidated`
  - `total_count = success_count + aborted_count`

<a id="api-task-success-rate"></a>
### API: `GET /v1/stats/line_chart/task_success_rate`

- Handler: `api/v1/stats/task_success_rate.go:GetTaskSuccessRateLineChart`
- Inputs:
  - `task_type`: `All|Image|Text`
  - `period`: `Hour|Day|Week`
- Window:
  - `Hour`: last 24 hours
  - `Day`: last 15 days
  - `Week`: last 8 weeks
- Data source:
  - `models.TaskCount` (`task_counts`)
- Aggregation:
  - Query rows in window and apply task type filter
    - `Image` maps to `task_type IN (SD, SD_FT_LORA)`
    - `Text` maps to `task_type = LLM`
    - `All` applies no `task_type` filter
  - Sum `success_count` and `total_count` by bucket
  - Compute `success_rate = success_count / total_count` when `total_count > 0`, otherwise `0`
  - Return `{timestamps[], success_rate[]}`
- Upstream producer semantics:
  - `success_count` and `total_count` are inherited from `tasks/stats.go:getTaskCounts`

<a id="api-task-execution-time"></a>
### API: `GET /v1/stats/histogram/task_execution_time`

- Handler: `api/v1/stats/task_execution_time.go:GetTaskExecutionTimeHistogram`
- Inputs:
  - `task_type`: `All|Image|Text`
  - `period`: `Hour|Day|Week`
  - optional `model_switched`: `0|1`
- Data source:
  - `models.TaskExecutionTimeCount` (`task_execution_time_counts`)
- Query window:
  - `Hour`: `start >= now-1h`
  - `Day`: `start >= now-24h`
  - `Week`: `start >= now-7d`
- Aggregation:
  - Read `seconds` and `count` rows in window
  - Apply task type filter and optional `model_switched` filter
  - Drop bins where `seconds >= 300`
  - Group by `seconds` and sum counts
  - Return `{execution_times[], task_count[]}`
- Business semantics:
  - This histogram counts tasks with returned results (`score_ready_time IS NOT NULL`).
  - It represents elapsed time from `start_time` to `score_ready_time`.
  - Tasks in `TaskErrorReported` are included.
  - It is not limited to terminal success tasks only.
- Upstream producer:
  - `tasks/stats.go:getTaskExecutionTimeCount`
  - Bin formula: `floor((score_ready_time - start_time)/5)*5` seconds
  - Aggregated per task type and per `model_swtiched` boolean

<a id="api-task-fee"></a>
### API: `GET /v1/stats/histogram/task_fee`

- Handler: `api/v1/stats/task_fee.go:GetTaskFeeHistogram`
- Inputs:
  - `task_type`: `All|Image|Text`
- Data source:
  - `models.InferenceTask` (`inference_tasks.task_fee`)
- Query window:
  - Last 1 hour by `created_at`
- Aggregation:
  - Filter rows with `task_fee IS NOT NULL` and `task_fee > 0`
  - Parse each `task_fee` as wei base-10 integer in Relay and compute min/max in Go.
  - Build 10 bins with decimal-order step:
    - `bin_size = 10^(digits(max-min)-1)` when `min < max`
    - `bin_size = 10^(digits(min)-1)` when `min == max`
  - Bin start is `floor(min / bin_size) * bin_size`
  - Return `{task_fees[], task_counts[]}` where:
    - `task_fees[]` are wei integer strings (bucket start values)
    - `task_counts[]` are bucket counts
  - If no qualified rows exist in the 1-hour window, return 10 zero buckets
- Note:
  - This endpoint reads raw task rows directly and does not use pre-aggregated stats tables.
  - This endpoint uses in-memory per-process cache keyed by `task_type` with TTL 60 seconds.
  - Performance is primarily determined by row count in the 1-hour window, not total historical table size when the `created_at` filter is index-backed.
  - Current operational reference: around 5,000 tasks per hour is within acceptable range for on-demand computation.

<a id="api-incentive-line-chart"></a>
### API: `GET /v1/stats/line_chart/incentive`

- Handler: `api/v1/stats/incentive.go:GetIncentiveLineChart`
- Inputs:
  - `period`: `Day|Week|Month`
  - optional `end`, optional `count`
- Data source:
  - `models.NodeIncentive` (`node_incentives`)
- Interval construction:
  - `Day`: default 14 daily intervals
  - `Week`: default 8 weekly intervals
  - `Month`: default 12 monthly intervals
- Aggregation:
  - Build interval boundaries first
  - Use a SQL `CASE WHEN` expression to map each row into interval index
  - Sum `incentive` per interval index
  - Fill missing intervals with zero
  - Return `{timestamps[], incentives[]}`

<a id="api-top-incentivized-nodes"></a>
### API: `GET /v2/incentive/nodes`

- Handler: `api/v2/incentive/nodes.go:GetNodeIncentive`
- Inputs:
  - None. The endpoint MUST NOT accept `period`, `size`, or pagination query parameters.
- Data sources:
  - Aggregated stats from `models.NodeIncentive`
  - Real-time node snapshot from `models.NetworkNodeData`
- Window:
  - Fixed one-day window: now minus 24 hours to now
- Aggregation and enrichment:
  - Sum by node address:
    - `incentive`
    - `task_count`
    - `sd_task_count`
    - `llm_task_count`
    - `sd_ft_lora_task_count`
  - Sort by `incentive DESC`, apply a fixed limit of 10 rows
  - Join `network_node_data` by node address
  - Compute QoS and selection probability fields through service helpers
  - Return node rows with incentive totals plus current card, VRAM, effective stake, and score fields
- Effective stake:
  - The response field is `staking`.
  - Portal MUST label this field as `Effective Stake`.
  - Portal MUST explain the field with this formula: `Effective Stake = Operator Stake + Delegated Stake + Locked Emission * 0.4 + Relay Account Balance`.
  - The locked emission component is rounded down to integer wei after the coefficient is applied.

<a id="api-top-delegations-by-task-fee"></a>
### API: `GET /v2/incentive/delegations`

- Handler: `api/v2/incentive/delegations.go:GetDelegationIncentive`
- Inputs:
  - None. The endpoint MUST NOT accept `period`, `size`, or pagination query parameters.
- Data source:
  - `models.DelegationTaskFeeLeaderboardSnapshot` (`delegation_task_fee_leaderboard_snapshots`)
- Response fields per row:
  - `delegator_address`
  - `node_address`
  - `network`
  - `staking_amount`: current delegation stake in wei integer string
  - `task_fee`: current UTC-day delegation task fee earning in wei integer string
  - `delegation_apr_12m`: node-level historical delegation APR ratio
- Read path:
  - The handler MUST read the snapshot table in `leaderboard_rank ASC` order with a fixed limit of 10 rows and MUST NOT scan or aggregate `user_staking_earnings` during the request.
- Snapshot producer:
  - `tasks/delegation_task_fee_leaderboard.go:StartDelegationTaskFeeLeaderboardRefresh` runs `service.RebuildDelegationTaskFeeLeaderboardSnapshots` at startup and every 5 minutes.
  - The rebuild reads current UTC-day `user_staking_earnings` daily rows (`time = current UTC day start`), joins non-slashed `delegations` rows on `(delegator, node, network)` for the staking amount, and left-joins `delegated_staking_node_list_snapshots` on node address for `delegation_apr_12m` (`0` when the node has no snapshot).
  - Rows are ordered by task fee descending, then by delegator address, node address, and network ascending for stable ordering, truncated to 10 rows, assigned ranks `1..10`, and written in one transaction that replaces all previous snapshot rows.
  - The leaderboard is minute-level delayed and MUST NOT be required to reflect task settlements in real time.
