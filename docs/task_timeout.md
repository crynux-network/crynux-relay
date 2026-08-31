# Task Timeout Flow

This document specifies task-stage deadlines and all deadline-expiration effects. Queue priority is specified in [task-pricing.md](./task-pricing.md). Execution parameter calibration is specified in [task_execution_parameters.md](./task_execution_parameters.md).

## Current-Stage Deadline

Relay MUST assign exactly one active deadline to each non-terminal task according to its current status. A status transition MUST invalidate the previous stage deadline immediately.

| Current status | Waiting for | Deadline | Expiration state and reason | Fee handling | Node health | Node Busy and finish |
|---|---|---|---|---|---|---|
| `TaskQueued` | Relay node assignment | Normal SD/LLM: `CreateTime + queue_timeout_seconds`; SDFT LoRA: `CreateTime + 3 minutes + Timeout` | `TaskEndAborted`, `TaskAbortTimeout` | Full `TaskRefund`; task income MUST NOT be created | No selected node; no penalty | No selected node; `nodeFinishTask` MUST NOT run |
| `TaskStarted`, `TaskParametersUploaded` | Node download, execution, score submission, or error report | `StartTime + Timeout` | `TaskEndAborted`, `TaskAbortTimeout` | Full `TaskRefund`; task income MUST NOT be created | Execution-timeout health penalty MUST apply | Node MUST remain `Busy` until expiration processing calls `nodeFinishTask` |
| `TaskScoreReady`, `TaskErrorReported` | Creator application validation | `ScoreReadyTime + app_validation_timeout_seconds` | `TaskEndAborted`, `TaskAbortCreatorValidationTimeout` | `TaskRefund` MUST NOT be created; the task's full fee MUST be split by successful-task DAO, node operator, and eligible delegator rules | No penalty; no success health boost | Node MUST remain `Busy` until expiration processing calls `nodeFinishTask` |
| `TaskValidated`, `TaskGroupValidated` | Node result upload | `ValidatedTime + result_upload_timeout_seconds` | `TaskEndAborted`, `TaskAbortResultUploadTimeout` | Full `TaskRefund`; task income MUST NOT be created | Result-upload-timeout health penalty MUST apply | Node MUST remain `Busy` until expiration processing calls `nodeFinishTask` |

`queue_timeout_seconds`, `app_validation_timeout_seconds`, and `result_upload_timeout_seconds` MUST be positive required Relay configuration values. Code MUST NOT supply fallback defaults.

Queue priority affects dispatch order only. It MUST NOT alter `CreateTime + queue_timeout_seconds`.

## Execution Timeout

Creators MUST NOT supply the execution Timeout for normal SD or LLM tasks. Relay MUST ignore a creator-supplied value for these task types.

After Relay selects the node and exact GPU variant, Relay MUST calculate normal-task `Timeout` from the frozen model execution configuration and in-memory records defined in [task_execution_parameters.md](./task_execution_parameters.md). Requested `auto` MUST compare complete predictions from `auto` and reported actual dtype records. Unknown-model fallback MUST use the nearest task `MinVRAM` interval, prefer exact-GPU records over other GPU names with the same VRAM, and use the maximum complete prediction among equal-distance records.

For SD:

```
Timeout = ceil(clamp(
    min_execution_timeout_seconds,
    max_execution_timeout_seconds,
    (overhead_seconds + SDUnits * seconds_per_sd_pixel_step)
        * timeout_multiplier
))
```

`overhead_seconds` and `seconds_per_sd_pixel_step` MUST come from the selected in-memory records. Timeout calculation MUST NOT read a configured fixed SD overhead.

For LLM:

```
Timeout = ceil(clamp(
    min_execution_timeout_seconds,
    max_execution_timeout_seconds,
    (
        constant_seconds
        + seconds_per_input_byte * LLMTextInputBytes
        + seconds_per_output_token * LLMMaxNewTokens
        + model_switch_seconds * model_switched
        + seconds_per_image * LLMImageCount
        + seconds_per_megapixel * (LLMImagePixels / 1000000)
    ) * timeout_multiplier
))
```

After selecting the node and before calculating Timeout, Relay MUST compare the node's current in-use base models with the task's required base models exactly once. Relay MUST pass that result into Timeout calculation and persist the same value in `model_swtiched`. The switch term MUST affect dispatch Timeout only and MUST NOT change queue priority.

If any selected record has not completed cold start, Relay MUST exclude its incomplete fitted parameters and use the maximum of the configured initial prediction and every selected ready record's complete prediction before applying `timeout_multiplier`, ceiling, and min/max clamp.

Relay MUST write selected node, `StartTime`, selected execution GPU name, selected execution GPU VRAM, estimated execution completion time, computed `Timeout`, and `TaskStarted` in the same database update. The estimated execution completion time MUST equal `StartTime` plus the exact-GPU execution estimate used as the input to Timeout calculation before applying `timeout_multiplier`, ceiling, or min/max clamp. Parameter lookup and normal Timeout calculation MUST use the in-memory cache and MUST NOT query the calibration database.

The persisted execution GPU and estimated completion time MUST remain immutable after task start. The creator-only batch status API specified in [task_batch_api.md](./task_batch_api.md) MUST return these values without reading the selected node's current mutable record.

Execution Timeout covers only the period from `TaskStarted` until score submission or task-error reporting. It MUST NOT include queue waiting, creator validation, or result upload.

SDFT LoRA MUST retain its creator-supplied Timeout for the complete task lifecycle and MUST NOT enter execution-parameter calibration or the normal SD/LLM staged timeout flow.

## Creator Validation Timeout

`TaskAbortCreatorValidationTimeout` means the creator did not complete the required validation protocol before its deadline. Its fee distribution compensates the node for execution time and the time that the node remained occupied while awaiting validation.

This payment MUST NOT be described as proof that the submitted result was correct. The task MUST remain `TaskEndAborted`. Relay MUST NOT create a validation conclusion, validation rank, or group `Q_long` update.

The same rule MUST apply to both `TaskScoreReady` and `TaskErrorReported`. After expiration, Relay MUST NOT decide whether the submitted score was correct or whether the reported task error was valid.

Relay MUST settle only the expired task's own fee. It MUST create `TaskIncome`, `DaoTaskShare`, and eligible `UserDelegation` events through the successful-task split and MUST ensure their total equals that task fee. Relay MUST NOT perform group QoS-weighted payment allocation for a group that did not complete validation.

Relay MUST NOT apply a node health penalty or result-upload success boost. Relay MUST then call `nodeFinishTask`.

If any validation-group member reaches `TaskEndAborted` with `TaskAbortCreatorValidationTimeout`, Relay MUST permanently reject validation of that group. `ValidateTaskGroup` MUST reject when that reason is present either before writing `TaskID` or again at the start of the final status transaction after locking the three tasks by ID. On either rejection, Relay MUST NOT assign validation ranks, compare scores, change statuses, process fees, update QoS, or process slash effects. Relay MUST NOT remove the expired member and validate the remaining members.

Other `TaskScoreReady` or `TaskErrorReported` members of that group MUST retain their own independent validation deadlines. Each member MUST expire and settle independently if its creator-validation deadline is reached.

## Result Upload Timeout

`TaskAbortResultUploadTimeout` attributes expiration to the selected node after validation made result upload available. Relay MUST refund the creator in full, MUST NOT create task income, MUST apply the result-upload timeout health penalty, and MUST call `nodeFinishTask`.

For execution timeout and result-upload timeout, Relay MUST write the penalized health and set `health_excluded = true` when the new health is below the configured enter threshold. Health mutation, exclusion mutation, task abort, payment refund, and `nodeFinishTask` MUST commit in the same timeout transaction. `nodeFinishTask` MUST restore a `Busy` node to `Available` when only short-term health exclusion applies. It MUST set the node to `Quit` only when the independent long-term QoS permanent-kickout condition is satisfied.

Queue timeout and creator-validation timeout MUST NOT change node health or `health_excluded`.

## Timeout Processor

Relay MUST run one internal timeout processor and MUST check tasks every two seconds. For each non-terminal task, it MUST evaluate only the deadline associated with the current status.

The processor MUST complete expiration by using a conditional status update. Task status, ledger events, node health mutation, and `nodeFinishTask` effects MUST commit atomically.

Each expiration MUST emit `TaskEndAborted` with the actual abort reason, abort issuer, and previous status. Metrics, trace data, fee handling, and QoS handling MUST distinguish `TaskAbortTimeout`, `TaskAbortCreatorValidationTimeout`, and `TaskAbortResultUploadTimeout`.

## Public Abort API

`POST /v1/inference_tasks/:task_id_commitment/abort_reason` MUST allow only the task creator to submit `TaskAbortCreatorCancelled` while the current task status is `TaskQueued`.

The API MUST reject:

- Every abort reason other than `TaskAbortCreatorCancelled`.
- Every signer other than the task creator.
- Every task in `TaskStarted` or a later status.
- Every request that attempts to invoke an internal deadline or protocol abort.

The public API MUST NOT use a deadline to authorize abort. Internal execution timeout, creator-validation timeout, result-upload timeout, and protocol termination MUST bypass the public API and use Relay's internal state-transition service.

Queued creator cancellation MUST use `TaskQueued` as an update condition, refund the task fee in full, and MUST NOT penalize a node. If assignment wins a concurrent `TaskQueued` to `TaskStarted` transition, cancellation MUST reload the task and reject the request.

## Concurrency

Every timeout transition MUST use the current task status as an update condition. A losing timeout, validation, score submission, result upload, or creator-cancel path MUST reload current state and MUST NOT repeat side effects after another path reaches a terminal state.

Exactly one competing path MUST create terminal events, ledger events, node health mutations, and node-finish effects. A terminal success MUST NOT be replaced by an abort. A timeout that wins before score submission MUST cause later score submission to fail status validation.
