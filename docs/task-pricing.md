# Task Pricing

This document specifies how Relay computes task priority for queued inference task dispatch.

## Scope

Task pricing covers queue ordering before node selection. It does not change task fee charge, refund, or settlement rules. Ledger behavior is specified in [task_fee_charge_and_settlement.md](./task_fee_charge_and_settlement.md).

Relay MUST keep node selection independent from queue pricing. After a task is selected from the queue, Relay MUST select the execution node using [node_selection.md](./node_selection.md).

## Definitions

- Task fee: the `task_fee` value on `InferenceTask`, denominated in the smallest CNX unit.
- Task priority: the stored numeric value used to order queued tasks for dispatch.
- Estimated node seconds: the estimated amount of node execution time consumed by the task.
- VRAM weight: the multiplier that represents the scarcity of the node capacity required by the task.
- Pricing unit: the task-type-specific work unit used to estimate execution time.

## Priority Formula

Relay MUST compute task priority when the task is created. The priority value MUST be immutable for the lifetime of that task.

```
priority = task_fee / (estimated_node_seconds * vram_weight)
```

Relay MUST store the computed value on the task row as `priority`. Relay MUST NOT compute task priority inside the dispatch polling query or inside the per-task dispatch retry loop.

Relay MUST order queued tasks by:

```sql
ORDER BY priority DESC, id ASC
```

Relay MUST NOT order queued dispatch by total task fee alone. Relay MUST NOT order queued dispatch by task ID alone except as the tie breaker after priority.

## Estimated Node Seconds

Relay MUST compute `estimated_node_seconds` from the task type and validated `task_args`.

Relay MUST define `task_pricing.overhead_seconds` in every runtime configuration template. This value covers task argument download, model preparation, waiting for result validation, and result upload overhead. It MUST be greater than `0`.

Relay MUST enforce a positive lower bound on `estimated_node_seconds` before division.

### Stable Diffusion Inference

For `TaskTypeSD`, Relay MUST read these fields from `task_args.task_config`:

- `num_images`
- `image_width`
- `image_height`

When a field is absent, Relay MUST use the schema default:

- `num_images = 6`
- `image_width = 512`
- `image_height = 512`

Relay MUST compute SD pricing units as:

```
sd_units = num_images * image_width * image_height / (512 * 512)
```

Relay MUST compute SD estimated node seconds as:

```
estimated_node_seconds = overhead_seconds + sd_units * seconds_per_sd_unit
```

### LLM Inference

For `TaskTypeLLM`, Relay MUST read `task_args.generation_config.max_new_tokens`.

When `generation_config` is absent, `generation_config` is `null`, or `max_new_tokens` is absent, Relay MUST use the configured `task_pricing.default_llm_max_new_tokens` value.

Relay MUST compute LLM pricing units as:

```
llm_units = max_new_tokens
```

Relay MUST compute LLM estimated node seconds as:

```
estimated_node_seconds = overhead_seconds + llm_units * seconds_per_llm_token
```

### Stable Diffusion Fine-Tune LoRA

For `TaskTypeSDFTLora`, Relay MUST use the task's stored `Timeout` value as the estimated node seconds:

```
estimated_node_seconds = Timeout
```

The timeout value is a self-enforcing upper bound for fine-tune pricing. A creator that understates the timeout causes the task to reach the running timeout deadline before successful completion.

## Calibration

Relay MUST maintain calibrated unit durations for task types whose estimated node seconds are unit-based:

- `seconds_per_sd_unit`
- `seconds_per_llm_token`

Relay MUST update these values using completed tasks that have valid execution timestamps. The measured execution duration is:

```
measured_execution_seconds = ScoreReadyTime - StartTime
```

For each completed SD or LLM task with positive pricing units, Relay MUST compute:

```
measured_unit_seconds = max(measured_execution_seconds - overhead_seconds, 0) / pricing_units
```

Relay MUST update the corresponding calibrated value with an exponentially weighted moving average:

```
new_value = alpha * measured_unit_seconds + (1 - alpha) * old_value
```

Relay MUST define these calibration settings in every runtime configuration template:

- `task_pricing.initial_seconds_per_sd_unit`
- `task_pricing.initial_seconds_per_llm_token`
- `task_pricing.calibration_alpha`
- `task_pricing.default_llm_max_new_tokens`

Calibration updates MUST affect only tasks created after the update. Relay MUST NOT recalculate priority for already created tasks.

## VRAM Weight

Relay MUST compute VRAM demand from task hardware requirements:

1. If `RequiredGPU` is not empty, use `RequiredGPUVRAM`.
2. Otherwise, use `MinVRAM`.

Relay MUST define `task_pricing.base_vram` in every runtime configuration template. `base_vram` MUST be greater than `0`.

Relay MUST compute:

```
vram_weight = max(vram_demand, base_vram) / base_vram
```

Tasks without a VRAM requirement have `vram_demand = 0` and therefore `vram_weight = 1`.

VRAM weight applies only to task queue priority. It MUST NOT alter node candidate filtering, node score calculation, staking score, QoS score, model locality boost, or weighted sampling.

## Dispatch Requirements

Relay MUST keep task dispatch database-backed. The task table MUST have an index that supports the queue polling order:

```sql
(status, priority DESC, id ASC)
```

The matching scheduler MUST fetch queued tasks ordered by `priority DESC, id ASC` and MUST pair them with nodes as specified in [task_matching.md](./task_matching.md).

If a fetched batch contains only tasks that are expired or temporarily undispatchable, Relay MUST continue scanning lower-priority queued tasks before sleeping. Relay MUST NOT leave eligible idle nodes unused only because the highest-priority fetched batch cannot currently start.

Node contention between queued tasks is resolved by the in-round reservation order defined in [task_matching.md](./task_matching.md): within a matching round, higher-priority tasks select nodes first, and a reserved node is excluded from lower-priority tasks' candidate sets in that round.

Relay MUST preserve existing queue timeout behavior. Queue timeout is specified in [task_timeout.md](./task_timeout.md). A low-priority task that stays queued until its queue deadline MUST be aborted by the timeout processor and refunded according to the task fee settlement rules.

## Trace and Metrics Visibility

Task trace output MUST expose the stored `priority`, `estimated_node_seconds`, `vram_weight`, and task-type pricing units for every task.

Queue and dispatch metrics MUST continue to use task type, creator, VRAM tier, queue depth, and queue wait labels according to [monitoring.md](./monitoring.md). Pricing metrics SHOULD expose calibrated unit-duration values and priority distribution buckets.

## Relevant Source Files

| File | Responsibility |
|------|----------------|
| `api/v1/inference_tasks/create_task.go` | Task creation input, task fee, task type, timeout, and hardware requirements |
| `models/task_args.go` | Task argument schema validation |
| `models/inference_task.go` | Task fields, status, timing fields, and execution duration source |
| `service/start_task.go` | Queue polling, matching scheduler entry point, and timeout processor startup |
| `service/select_nodes.go` | Node candidate filtering and weighted sampling after queue selection |
