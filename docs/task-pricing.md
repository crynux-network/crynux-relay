# Task Pricing

This document specifies queue priority calculation and dispatch ordering. Execution parameter production is specified only in [task_execution_parameters.md](./task_execution_parameters.md). Deadline calculation and timeout handling are specified only in [task_timeout.md](./task_timeout.md).

## Parameter Selection

At task creation, Relay MUST determine `VRAMDemand`:

1. If `RequiredGPU` is set, `VRAMDemand` MUST equal `RequiredGPUVRAM`.
2. Otherwise, `VRAMDemand` MUST equal `MinVRAM`.

For a task with `RequiredGPU`, Relay MUST read the exact `(RequiredGPU, RequiredGPUVRAM)` execution parameters from the in-memory GPU parameter cache. Relay MUST NOT create or read a task-pricing aggregate key for this task. If the exact variant is not initialized, Relay MUST initialize it using the same-VRAM initialization rules in [task_execution_parameters.md](./task_execution_parameters.md).

For a task without `RequiredGPU`, Relay MUST use an in-memory aggregate key `(TaskType, VRAMDemand)`. On first use of a key, Relay MUST select every GPU parameter record with `GPUVram >= VRAMDemand` and at least one valid sample for the task type. Relay MUST initialize the aggregate coefficients as the simple arithmetic mean of those compatible records. Every compatible GPU variant MUST have equal weight, and cumulative successful-sample counts MUST NOT be aggregation weights. If no compatible sampled variant exists, Relay MUST use configured initial parameters.

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

For LLM, Relay MUST compute:

```
estimated_node_seconds =
    constant_seconds
    + seconds_per_input_byte * LLMInputBytes
    + seconds_per_output_token * LLMMaxNewTokens
```

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

Relay MUST store `SDUnits` or `LLMInputBytes` and `LLMMaxNewTokens`, `estimated_node_seconds`, `vram_weight`, and `priority` on task creation. These values MUST remain unchanged for the task lifetime. Later execution-parameter updates MUST NOT recalculate existing task priority.

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

## Trace and Metrics

Task trace output MUST expose stored `priority`, `estimated_node_seconds`, `vram_weight`, `VRAMDemand`, and the stored task workload field.

Relay MUST expose initialized aggregate execution parameters and task-priority distributions in the base units and labels specified in [monitoring.md](./monitoring.md).
