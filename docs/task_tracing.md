# Task Tracing

## Target

The Admin API MUST provide a task trace endpoint keyed by `task_id_commitment`. The response MUST be ordered chronologically and MUST include both the timestamp for each recorded step and the derived duration for every phase where a start and end time are available. Durations MUST be explicit fields in the response and MUST NOT require the caller to calculate them from timestamps.

Target endpoint:

`GET /v2/admin/tasks/:task_id_commitment/trace?auth=<token>`

The endpoint MUST assemble the trace from persisted task rows, persisted event rows, current node data, and the in-memory task trace store. Persisted task rows provide task creation, queue entry, task start, score or task error submission, validation completion, result upload completion, Relay result availability, and final task state. Persisted event rows provide ordered lifecycle event IDs and event timestamps. Current node data provides clearly labeled current selected-node context only; it MUST NOT replace a missing start-time node snapshot.

The in-memory task trace store MUST retain only facts that Relay does not already persist. The store MUST be keyed by `task_id_commitment`, MUST add a group lookup by revealed `task_id`, and MUST store phase-specific fields for node selection, start-time node/model snapshot, validation request, validation result summary, upload availability, upload start, and app result fetches. Every record MUST include creation time, update time, and expiration time. The `task.task_tracing_duration_days` configuration controls volatile trace retention. A value of `0` disables volatile trace writes. A non-zero value keeps records in memory for that many days after the last update, and expired records MUST be removed by cleanup.

Relay MUST NOT store full validation proof values in the in-memory trace store. When full VRF proof or public key values are not stored, the response MUST identify the stored representation and MUST report the unavailable full values in `missing_data`.

Target trace data:

- Task creation:
  - Timestamp: task create time.
  - Details: task input parameters, including `task_args`, task type, task version, timeout, VRAM/GPU requirements, task fee, task size, model IDs, creator, nonce, sampling seed when available, and task pricing fields specified in [task-pricing.md](./task-pricing.md).
  - Duration: none for the first trace step.
- Queue entry and waiting:
  - Timestamp: time when the task enters the queue.
  - Details: queued status, queue deadline inputs, and stored queue priority.
- Node selected:
  - Timestamp: time when Relay selects a node for the task before task start.
  - Details: selected node address.
  - Details: final base-ready candidate pool used by weighted random selection, including each recorded candidate's address, card name, staking score, runtime QoS score, and final probability weight after the `0.3` in-memory base-model locality component is applied.
  - Details: every recorded candidate MUST have passed hardware, version, task-specific, and node-name policy filters and MUST have the task's base model ID on disk. Auxiliary model IDs MUST NOT affect the candidate pool.
  - Details: the candidate pool MUST include `candidate_pool_total_count` and `candidate_pool_truncated`. Relay MUST cap the stored candidate list and MUST set `candidate_pool_truncated` to true when the final candidate pool contains more nodes than the stored list.
  - Duration: queue waiting time, calculated as node selected time minus queue entry time.
- Queue abort:
  - Timestamp: task aborted time when the task is aborted before node selection.
  - Details: abort reason and abort issuer when available.
  - Duration: queue lifetime, calculated as queue abort time minus queue entry time.
- Task execution start:
  - Timestamp: task start time.
  - Details: selected node information, including address, card/GPU name, VRAM, version, operator staking, delegated staking summary, QoS score, health score inputs when available, status, delegator share/count, and all other persisted node base information useful for diagnosis.
  - Details: selected node model cache snapshot, including authoritative node-reported models present on disk and base models currently in memory or in use. Task start MUST NOT create missing model rows or emit model downloads.
  - Duration: queue waiting time MUST also be present on this step for easy reading.
- Score submission:
  - Timestamp: score submission time.
  - Details: submitted score and task error information if the node reported an execution error instead of a score.
  - Duration: execution duration, calculated as score submission time minus task start time.
- Validation request and group reveal:
  - Timestamp: time when Relay receives the validation API request from the app.
  - Details: submitted validation parameters, including task ID, task ID commitments, VRF proof, and public key when the trace persistence policy stores full request data.
  - Details: if full VRF proof or public key values are not persisted, the trace MUST record the stored representation, such as a hash or redacted marker, and MUST identify it in `missing_data` when full values are unavailable.
  - Details: whether validation is single-task validation or group validation. For group validation, the trace MUST include the two additional validation tasks after Relay can associate them by the revealed `task_id`.
  - Details: each validation task MUST use the same task input shape as the original task, including creation time, task input parameters, selected node, score or task error, and final status when available.
  - Duration: time from original score submission to validation request time when both timestamps are available.
  - Duration: validation task queue waiting time and execution duration MUST be included for each validation task when its timestamps are available.
  - Scope: Relay does not know which tasks belong to a validation group until the validation request reveals the shared `task_id`. The trace MUST NOT present the two additional tasks as validation tasks before this reveal point.
- Validation result:
  - Timestamp: group-level validation completion time.
  - Details: validation result summary, whether validation passed, number of pass tasks, number of refund tasks, number of invalid tasks, number of aborted tasks, final status of every task in the validation group, each task's selected node, score, QoS score, and abort reason when applicable.
  - Details: each task in the validation group MUST also expose its own validation or terminal status timestamp when available.
  - Duration: validation duration, calculated as group-level validation completion time minus validation request time when validation request time is recorded.
- Result upload availability:
  - Timestamp: time when the result becomes uploadable by the node.
  - Details: upload eligibility status and whether the task is a normal successful task, group validated task, invalidated task with slash evidence, or refund task.
  - Duration: not applicable when upload availability is the same state transition as validation completion.
- Result upload:
  - Timestamp: result upload start time and result upload completion time.
  - Details: uploaded result status, result file metadata when available, checkpoint upload status for fine-tune tasks, and slash evidence result artifact status when applicable.
  - Duration: upload result duration, calculated as upload completion time minus upload start time.
- App result availability:
  - Timestamp: time when Relay makes the result available to the app.
  - Timestamp: time when the app fetches the result if result access tracing is enabled.
  - Details: the trace MUST distinguish `relay_result_available_time` from `app_result_fetched_time`.
  - Duration: time from upload completion to app fetch when app fetch time is recorded; otherwise unavailable with a reason.
- Abort:
  - Timestamp: explicit task aborted time whenever any intermediate step ends with `TaskEndAborted`.
  - Details: abort reason, abort issuer, last status before abort when available, selected node when available, and whether the abort happened while queued, running, waiting for validation, or waiting for upload.
  - Duration: time from the previous lifecycle step to abort, plus total task lifetime from create time to abort.

Relay MUST NOT backfill historical tasks and MUST NOT derive fallback timestamps or fallback durations from unrelated lifecycle fields. The trace implementation only records new task data after task tracing is enabled. Fields that are unavailable because the task predates tracing, tracing is disabled, the in-memory record expired, or the lifecycle step has not occurred MUST be empty in the response and MUST appear in a structured `missing_data` section with the missing field name, affected trace step, and reason.
