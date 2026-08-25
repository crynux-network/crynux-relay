# Task Pricing

This document specifies queue priority calculation and dispatch ordering. Execution parameter production is specified only in [task_execution_parameters.md](./task_execution_parameters.md). Deadline calculation and timeout handling are specified only in [task_timeout.md](./task_timeout.md).

## Parameter Selection

At task creation, Relay MUST determine `VRAMDemand`:

1. If `RequiredGPU` is set, `VRAMDemand` MUST equal `RequiredGPUVRAM`.
2. Otherwise, `VRAMDemand` MUST equal `MinVRAM`.

For a task with `RequiredGPU`, Relay MUST select records for `(RequiredGPU, RequiredGPUVRAM)` and the frozen model execution configuration from the in-memory cache. Relay MUST NOT create or read a task-pricing aggregate key for this task. Selection of `auto` dtype and unknown-model interval fallback MUST follow [task_execution_parameters.md](./task_execution_parameters.md).

For a task without `RequiredGPU`, Relay MUST use an in-memory aggregate key `(TaskType, VRAMDemand, ModelName, ModelVariant, RequestedDType, QuantizeBits)`. On first use, Relay MUST select sampled records with `GPUVram >= VRAMDemand` and the frozen model execution configuration. Requested `auto` MUST include `auto` and reported actual dtype records. When the model configuration has no records, Relay MUST select the nearest `MinVRAM` interval records. Relay MUST initialize aggregate coefficients as the simple arithmetic mean of selected records with equal weight. Cumulative successful-sample counts MUST NOT be aggregation weights. If no compatible sampled record exists, Relay MUST use configured initial parameters.

An initialized aggregate-key read MUST have O(1) complexity. Task creation MUST NOT query the calibration database or inspect the current live-node candidate set.

When an exact GPU parameter record changes, Relay MUST immediately recompute every initialized aggregate key of the same task type whose `VRAMDemand <= GPUVram`. Each recomputation MUST select all currently compatible sampled GPU records and calculate a new simple mean. Relay MUST NOT update only the key associated with the sample-producing task. Keys with `VRAMDemand > GPUVram` MUST remain unchanged.

Relay MUST NOT persist aggregate keys. After restart, Relay MUST recreate each aggregate key on first use from the restored exact GPU parameter cache.

## Estimated Node Seconds

For SD, Relay MUST compute:

```
estimated_node_seconds =
    overhead_seconds
    + SDUnits * seconds_per_sd_pixel_step
```

`overhead_seconds` and `seconds_per_sd_pixel_step` MUST come from the selected or aggregated calibration records specified above. Task creation MUST NOT read a configured fixed SD overhead.

For LLM, Relay MUST compute:

```
estimated_node_seconds =
    constant_seconds
    + seconds_per_input_byte * LLMTextInputBytes
    + seconds_per_output_token * LLMMaxNewTokens
    + seconds_per_image * LLMImageCount
    + seconds_per_megapixel * (LLMImagePixels / 1000000)
```

Task creation MUST set the model-switch term to zero because the selected node is not known. Model switching MUST NOT change the frozen creation estimate or queue priority.

At task creation, if `generation_config.max_new_tokens` is absent, Relay MUST store the configured default into `LLMMaxNewTokens`. Relay MUST assume generation can reach `LLMMaxNewTokens` and MUST NOT reduce output work by a historical early-stop ratio.

For SDFT LoRA, Relay MUST use the creator-supplied stored `Timeout` as `estimated_node_seconds`. SDFT LoRA MUST NOT use the SD or LLM execution parameter cache.

Relay MUST enforce a positive lower bound on `estimated_node_seconds` before division.

## VRAM Weight and Priority

Relay MUST define a positive configured `base_vram` and compute:

```
vram_weight = max(VRAMDemand, base_vram) / base_vram
```

Relay MUST compute:

```
priority = task_fee / (estimated_node_seconds * vram_weight)
```

Relay MUST store `SDUnits` or all of `LLMTextInputBytes`, `LLMImageCount`, `LLMImagePixels`, and `LLMMaxNewTokens`, together with `estimated_node_seconds`, `vram_weight`, and `priority` on task creation. These values MUST remain unchanged for the task lifetime. Later execution-parameter updates and the model-switch decision at dispatch MUST NOT recalculate existing task priority.

VRAM weight MUST affect queue priority only. It MUST NOT alter candidate filtering, node score, staking score, QoS, model locality, or weighted node sampling.

## Dispatch Order

The task table MUST have an index supporting:

```sql
(status, priority DESC, id ASC)
```

The matching scheduler MUST fetch queued tasks in:

```sql
ORDER BY priority DESC, id ASC
```

Relay MUST NOT order queued tasks by task fee alone. Task ID MUST be used only as the tie breaker after priority.

Within a matching round, higher-priority tasks MUST select nodes first. A node reserved by a higher-priority task MUST be excluded from lower-priority candidate sets in that round, as specified in [task_matching.md](./task_matching.md).

If the current fetched batch contains only expired or temporarily undispatchable tasks, Relay MUST continue scanning lower-priority queued tasks before sleeping. Relay MUST NOT leave an eligible node idle solely because a higher-priority task cannot start.

Priority changes dispatch order only. It MUST NOT extend or shorten the queue deadline defined in [task_timeout.md](./task_timeout.md).

## Queued Task Priority Snapshot

Relay MUST maintain one in-memory snapshot of the current queued-task priority range for public callers.

The snapshot MUST cover every `TaskQueued` task. It MUST NOT filter by task type. Soft-deleted rows MUST be excluded by the ordinary GORM soft-delete scope.

Relay MUST refresh the snapshot on startup before the HTTP server begins listening. After startup, Relay MUST refresh the snapshot every `task_pricing.queued_task_priority_snapshot_interval_seconds` seconds. Every runtime configuration template MUST set that value to `300`. Configuration loading MUST fail when the value is missing or not greater than zero.

Each refresh MUST:

1. Count the current queued tasks.
2. When the count is zero, store `queued_task_count = 0` and store null for the three priority fields.
3. When the count is greater than zero, read only the highest, median, and lowest priority values from the existing `(status, priority DESC, id ASC)` index ordering. Relay MUST NOT load every queued priority into memory.
4. Replace the in-memory snapshot atomically after the refresh succeeds.
5. Set `as_of` to the UTC Unix second of the successful refresh.

The median MUST use zero-based index `count / 2` in the descending priority order. For the even-length queue `[100, 80, 60, 40]`, the median MUST be `60`. The selected median MUST be the priority of one real queued task.

When a refresh fails, Relay MUST keep the previous successful snapshot and MUST NOT change `as_of`.

Relay MUST expose:

```text
GET /v2/tasks/queued/priority
```

The endpoint MUST be public and MUST NOT require authentication. The handler MUST return only the in-memory snapshot. It MUST NOT query the database.

The response body MUST use the standard Relay v2 response envelope:

```json
{
  "message": "success",
  "data": {
    "as_of": 1787623200,
    "queued_task_count": 4,
    "highest_priority_gwei": "57",
    "median_priority_gwei": "33",
    "lowest_priority_gwei": "25"
  }
}
```

Relay MUST keep the stored task `priority` in wei per weighted node second. The API MUST convert each selected priority to Gwei by integer division by `10^9` and MUST discard any remainder below one Gwei. The three priority fields MUST be JSON strings of non-negative integers. When `queued_task_count` is `0`, the three priority fields MUST be `null`.

## Trace and Metrics

Task trace output MUST expose stored `priority`, `estimated_node_seconds`, `vram_weight`, `VRAMDemand`, and the stored task workload field.

Relay MUST expose initialized aggregate execution parameters and task-priority distributions in the base units and labels specified in [monitoring.md](./monitoring.md).
