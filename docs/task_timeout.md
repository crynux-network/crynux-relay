# Task Timeout Flow

This document specifies how Relay detects and completes timed-out inference tasks.

## Scope

Relay MUST apply timeout handling to every inference task that has not reached a terminal state.

Terminal task states are:

| Status | Meaning |
|--------|---------|
| `TaskEndSuccess` | A single task completed successfully and its result was uploaded. |
| `TaskEndGroupSuccess` | A validation-group task completed successfully and its result was uploaded. |
| `TaskEndGroupRefund` | A validation-group duplicate task completed and its task fee was refunded. |
| `TaskEndAborted` | The task was aborted. |
| `TaskEndInvalidated` | The task failed validation and triggered node slashing. |

Non-terminal task states are:

| Status | Timeout clock |
|--------|---------------|
| `TaskQueued` | Queue timeout clock |
| `TaskStarted` | Running timeout clock |
| `TaskParametersUploaded` | Running timeout clock |
| `TaskErrorReported` | Running timeout clock |
| `TaskScoreReady` | Running timeout clock |
| `TaskValidated` | Running timeout clock |
| `TaskGroupValidated` | Running timeout clock |

## Timeout Clocks

Each task carries its own `Timeout` value in seconds. Relay MUST use the task's stored `Timeout` value. Relay MUST NOT use a node-level timeout configuration for task timeout decisions.

### Queue Timeout

A queued task has not been assigned to a node. Relay MUST treat a queued task as timed out when:

```
CreateTime + 3 minutes + Timeout <= current time
```

The 3-minute grace period is part of the queue timeout clock. It gives Relay time to select an eligible node after task creation.

Queued task dispatch order is specified in [task-pricing.md](./task-pricing.md). Queue timeout uses the task's create time and timeout value only. Relay MUST NOT extend or shorten the queue timeout deadline based on task priority.

### Running Timeout

A running task has been assigned to a node or is in a post-execution non-terminal state. Relay MUST treat a running task as timed out when:

```
StartTime + Timeout <= current time
```

The running timeout clock applies to `TaskStarted`, `TaskParametersUploaded`, `TaskErrorReported`, `TaskScoreReady`, `TaskValidated`, and `TaskGroupValidated`.

## Timeout Processor

Relay MUST run one task timeout processor as part of `StartTaskProcesser`.

The timeout processor MUST:

1. Periodically scan queued tasks and running tasks.
2. Select queued tasks that have reached the queue timeout deadline.
3. Select running tasks that have reached the running timeout deadline.
4. Complete each selected task through `SetTaskStatusEndAborted` with `TaskAbortTimeout`.
5. Use the configured Relay blockchain account address as the abort issuer.

The queued-task dispatcher MUST NOT abort expired queued tasks directly. It MUST skip expired queued tasks and leave abort completion to the timeout processor.

## Abort Completion

When Relay completes a timed-out task, it MUST set:

| Field | Value |
|-------|-------|
| `Status` | `TaskEndAborted` |
| `AbortReason` | `TaskAbortTimeout` |
| `ValidatedTime` | Current Relay time, when the task does not already have a validated time |

Relay MUST refund the task fee to the creator through the relay account ledger when a timed-out task reaches `TaskEndAborted`.

Relay MUST emit a `TaskEndAborted` event. The event MUST include the task ID commitment, abort issuer, abort reason, and previous task status.

## Node Effects

Queued timeout has no selected node. Relay MUST NOT apply a node health penalty when a task times out before node assignment.

Running timeout has a selected node. When a running task reaches `TaskEndAborted`, Relay MUST finish the node's current task through `nodeFinishTask`.

Relay MUST apply the timeout health penalty to the selected node only when all of these conditions are true:

1. The abort reason is `TaskAbortTimeout`.
2. The task has a selected node.
3. The task has no `ScoreReadyTime`.

If the selected node already submitted a score, Relay MUST NOT apply the timeout health penalty for that task. `ScoreReadyTime` proves that the node completed execution and submitted its result score to Relay, so a later timeout is not attributed to node execution failure. This rule applies to timed-out `TaskScoreReady`, `TaskValidated`, and `TaskGroupValidated` tasks.

After applying any required timeout health penalty, Relay MUST evaluate permanent kickout conditions. If the node's effective health is below `qos.health_kickout_threshold`, Relay MUST kick out the node through the non-slashed quit path. Kickout MUST queue `NodeStaking::unstake` when the node still has active operator staking on its current blockchain network. Timeout kickout MUST NOT queue `NodeStaking::slashStaking`.

## Concurrency

Timeout completion MUST be safe when it races with node or application API calls.

Task status transitions MUST use the current task status as an update condition. If another process changes the task status first, the losing timeout or API path MUST reload task status and retry only when the task still requires completion. If the task is already `TaskEndAborted`, the later abort path MUST be a no-op.

This concurrency rule MUST prevent duplicate task refunds, duplicate node health penalties, duplicate node finish operations, and duplicate task-end events for the same task.

Race outcomes are defined as follows:

| Race | Required outcome |
|------|------------------|
| Timeout processor and manual abort API both abort the task | Exactly one path performs the state transition and side effects. The other path observes the completed status and stops. |
| Timeout processor and score submission race | If timeout wins, score submission is rejected by task status validation. If score submission wins, later timeout completion MUST NOT apply a node health penalty because `ScoreReadyTime` is set. |
| Timeout processor and validation race | Exactly one state transition succeeds. The losing path MUST observe the updated task status and stop or retry according to the current state. |
| Timeout processor and result upload race | Exactly one terminal transition succeeds. A task that reaches a terminal success state MUST NOT be aborted by timeout processing. |
| Timeout processor and node status change race | Node status updates MUST use the current node status as an update condition. A conflicting node status update MUST not be overwritten by timeout processing. |

## Source Files

| File | Responsibility |
|------|----------------|
| `service/start_task.go` | Queue dispatch and timeout processor implementation |
| `service/task_status.go` | Task abort transition, task refund, timeout health penalty, node finish call |
| `service/node.go` | Node task finish, permanent kickout, non-slashed quit path |
| `service/qos.go` | Timeout health penalty and health kickout threshold |
| `api/v1/inference_tasks/abort_task.go` | Manual abort API that uses the same abort completion path |
| `models/inference_task.go` | Task status enum, timeout field, optimistic status update |
| `models/node.go` | Node status enum and optimistic node status update |
