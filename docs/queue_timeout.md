# Creator Queue Timeout

This document specifies the creator-supplied queue wait budget for normal SD and LLM tasks. Stage deadlines, abort reasons, fee handling, and Node Busy behavior for every task status are specified in [task_timeout.md](./task_timeout.md). Queue priority is specified in [task-pricing.md](./task-pricing.md).

## Purpose

Crynux schedules queued tasks by immutable task priority derived from `task_fee`, estimated execution time, and VRAM weight. That design is flexible pricing: a creator MAY pay a higher fee for earlier dispatch or a lower fee for later dispatch.

Flexible pricing alone does not set how long a task MAY remain queued. Without a per-task queue wait budget, every normal task shares one operator-configured wait. Creators MUST be able to combine:

* a low `task_fee` with a long queue wait, so the task MAY execute during quieter network periods at low cost;
* a high `task_fee` with a short queue wait, so the task either starts quickly or fails quickly with a full refund.

Creator `queue_timeout` supplies that per-task wait budget. It MUST NOT change priority calculation, dispatch order rules, or Relay-owned execution timeout after node assignment.

## Scope

Creator `queue_timeout` applies only while the task status is `TaskQueued`.

After Relay selects a node and starts the task, normal SD and LLM execution timeout remains Relay-owned as specified in [task_timeout.md](./task_timeout.md). Creators MUST NOT use `queue_timeout` to control execution, creator-validation, or result-upload deadlines.

SDFT LoRA MUST NOT accept creator `queue_timeout`. SDFT queue and lifecycle deadlines remain `CreateTime + 3 minutes + Timeout` with creator-supplied execution `timeout` as specified in [task_timeout.md](./task_timeout.md).

## Configuration

`task_pricing` MUST define two positive required values. Code MUST NOT supply fallback defaults.

| Key | Role |
|---|---|
| `default_queue_timeout_seconds` | Queue wait used when create omits `queue_timeout` |
| `max_queue_timeout_seconds` | Inclusive upper bound for every resolved normal-task queue wait |

`max_queue_timeout_seconds` MUST be greater than or equal to `default_queue_timeout_seconds`. Configuration loading MUST fail when either value is missing, zero, or violates that inequality.

## Create Contract

Create input for normal SD and LLM tasks MAY include optional `queue_timeout` as an unsigned integer number of seconds.

When `queue_timeout` is omitted, Relay MUST resolve the queue wait to `default_queue_timeout_seconds`.

When `queue_timeout` is present:

1. Relay MUST reject the request when the value is zero.
2. Relay MUST reject the request when the value is greater than `max_queue_timeout_seconds`.
3. Otherwise Relay MUST use that value as the resolved queue wait.

When the task type is SDFT LoRA and `queue_timeout` is present, Relay MUST reject the request.

Single create and batch create MUST apply the same rules. Batch create MUST validate and resolve `queue_timeout` per item.

## Persistence and Deadline

For each accepted normal SD or LLM task, Relay MUST persist the resolved queue wait seconds on the task row as `queue_timeout`.

Relay MUST set the queue-stage deadline to:

```text
DeadlineAt = CreateTime + resolved_queue_timeout_seconds
```

The persisted resolved value MUST be part of the immutable create identity used for batch idempotency. A retry with a different `queue_timeout` under the same task commitment MUST be a conflict.

Queue priority MUST NOT alter that deadline. Matching MUST skip a queued task whose queue deadline has already passed and MUST leave expiration to the timeout processor specified in [task_timeout.md](./task_timeout.md).

## Expiration

When the queue deadline expires while status remains `TaskQueued`, Relay MUST end the task as `TaskEndAborted` with `TaskAbortTimeout`, MUST refund the task fee in full, MUST NOT create task income, and MUST NOT penalize a node. Complete expiration effects are specified in [task_timeout.md](./task_timeout.md).
