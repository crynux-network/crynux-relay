# Task Batch APIs

This document specifies the Relay APIs that task creators use to submit and advance multiple tasks with bounded HTTP request counts.

## Common Contract

The batch create, status, validation, and cancellation endpoints MUST accept only a bounded number of items. Each endpoint MUST define a required positive maximum item count in Relay configuration and MUST reject an oversized request before processing any item.

Every request MUST contain items for one creator and MUST carry one signature over the complete ordered request body and timestamp. Relay MUST validate request authentication before processing any item. An item that does not belong to the authenticated creator MUST use the same externally visible representation as an absent item when ownership disclosure would otherwise be possible.

Relay MUST preserve request item order in every batch response. Duplicate item identities within one request MUST be rejected before processing. A whole-request authentication, decoding, size, or database-availability failure MUST return a whole-request error. Item-specific outcomes MUST be returned in the successful batch response and MUST NOT change the HTTP status of other items.

The batch APIs MUST NOT replace or change the behavior of the existing single-task APIs. In particular, the existing task-fetch API used by an assigned node MUST continue returning task arguments and conditionally recording `delivered_time`.

## Batch Creation

`POST /v1/inference_tasks/batch` MUST be the signed batch creation endpoint. Its items MUST contain the complete normal-task creation input, including `task_id_commitment`. The endpoint MUST accept ordinary SD and LLM tasks. SDFT checkpoint upload MUST remain on its existing multipart single-task endpoint.

Relay MUST normalize, validate, price, and create each item independently through the existing single-task financial boundary. Each item MUST use its own database transaction for:

- task row creation;
- total-task counter increment;
- one `TaskPayment` event;
- the corresponding creator Relay Account debit.

A failure for one item MUST NOT roll back or prevent successful items. Relay MUST return one of these outcomes for each item:

- `created`, including sequence, sampling seed, and initial status;
- `already_exists`, when the same creator previously created the same immutable task input under that commitment;
- `commitment_conflict`, when the commitment already identifies different immutable task input;
- `permanent_error`, for an input that cannot succeed when retried unchanged;
- `temporary_error`, when the item can be retried after a transient failure.

Creation MUST be idempotent by task commitment and immutable normalized task input. A client MUST treat `already_exists` as a successful creation outcome equivalent to `created`. Relay MUST NOT create another task row or another `TaskPayment` event for either an identical retry or a conflicting retry.

Relay Account cache mutation MUST become visible only after the corresponding database transaction commits. Concurrent item transactions for one creator MUST serialize balance checks and debits so that their total successful charges never exceed the available balance. A database rollback or commit failure MUST NOT change the cached balance.

## Batch Status

`POST /v1/inference_tasks/batch/status` MUST be the signed creator-only batch status endpoint. It MUST use one exact bulk query over the requested commitments and MUST NOT invoke the existing node task-fetch handler.

The endpoint MUST be strictly read-only. It MUST NOT change `delivered_time`, task status, task trace state, node state, Relay Account state, or any other persisted or in-memory state.

Each item MUST return only:

- task commitment;
- found state;
- task status;
- abort reason;
- task error;
- sequence;
- sampling seed;
- selected execution GPU name and GPU VRAM when available;
- estimated execution completion time when available;
- result availability.

The endpoint MUST NOT return task arguments, nonce, score, task fee, or model identifiers. An absent task and a task owned by another creator MUST both return `found = false`.

The selected execution GPU name and GPU VRAM MUST be the immutable assignment captured when the task starts. Relay MUST NOT derive them from the node's current mutable record. LLM VSS processing uses this pair to restrict the two additional members to the same GPU variant as the primary task.

The estimated execution completion time MUST equal task start time plus the exact-GPU execution estimate that Relay used as the input to execution-timeout calculation. It MUST exclude the timeout multiplier, timeout minimum, timeout maximum, and every later stage deadline.

`result availability` MUST be derived from persisted task state without filesystem access. It MUST be true only when Relay has reached `TaskEndSuccess` or `TaskEndGroupSuccess`. The whole-task result endpoint MUST perform the authoritative file-presence checks before download.

## Batch Validation

`POST /v1/inference_tasks/batch/validate` MUST be the signed batch validation endpoint. Each validation unit MUST contain either:

- one commitment, one task ID, one VRF proof, and one public key; or
- exactly three commitments, one shared task ID, one shared VRF proof, and one public key.

Every validation unit MUST retain the transaction, task-locking, node-state, fee, QoS, health, and slashing behavior of the existing single or group validation operation. Relay MUST process units independently and MUST NOT claim atomic behavior across units.

Relay MUST verify the task commitments, task ID, VRF proof, public key, creator, and current task states before persisting the revealed task ID or changing any validation state. Invalid proof or input MUST leave every task, node, account, event, and cache value unchanged.

Relay MUST return one of these outcomes for each unit:

- `validated`;
- `already_applied`, when the same validation has already completed;
- `permanent_error`, when the unit cannot succeed when retried unchanged;
- `temporary_error`, when the unit can be retried after a transient failure.

Retrying an already completed unit MUST NOT repeat fee, QoS, health, node-finish, event, or slashing effects.

## Batch Cancellation

`POST /v1/inference_tasks/batch/abort` MUST be the signed batch cancellation endpoint containing task commitments and `TaskAbortCreatorCancelled`. Each item MUST retain the existing creator-only and `TaskQueued` state requirement.

Relay MUST process each cancellation independently. A successful cancellation MUST retain the existing atomic task status, refund event, Relay Account, and task event behavior. Relay MUST return one of these outcomes for each item:

- `cancelled`;
- `already_cancelled`;
- `not_cancellable`;
- `not_found`;
- `permanent_error`;
- `temporary_error`.

A failure or rejection for one item MUST NOT roll back another cancellation. Repeated cancellation MUST NOT create another refund or repeat any terminal side effect.

## Task Result Download

`GET /v1/inference_tasks/:task_id_commitment/results` MUST replace the indexed result-download endpoint for ordinary tasks. The existing `GET /v1/inference_tasks/:task_id_commitment/results/:index` endpoint MUST be removed. SDFT checkpoints MUST remain on their existing endpoint.

Before sending response headers, Relay MUST:

- verify creator ownership;
- verify `TaskEndSuccess` or `TaskEndGroupSuccess`;
- require `TaskSize = 1` for an LLM task and open `0.json`;
- derive every expected image index from `TaskSize` for an SD task;
- validate every result path;
- open every expected file;
- reject the request if any expected file is absent;
- enforce a configured maximum aggregate uncompressed size.

For an LLM task, Relay MUST stream the single `0.json` file as the response with `Content-Type: application/json`.

For an SD task, Relay MUST stream one ZIP archive containing all result images. Archive entries MUST use only `<index>.png`, in ascending index order from `0` through `TaskSize - 1`. Relay MUST construct entry names from validated numeric indexes and MUST NOT derive an entry path from client input or a filesystem path.

Relay MUST NOT combine results from different tasks and MUST NOT buffer the complete JSON file or image archive in memory.

## Limits and Metrics

Relay MUST configure separate maximum item counts for create, status, validation, and cancellation batches, plus a maximum uncompressed byte size for one SD result archive. A client MUST submit request batches no greater than the corresponding Relay limits and MUST split larger work before submission.

Relay MUST record request count, item count, request duration, per-item outcome, response bytes, and result-download bytes separately for each batch endpoint. Metrics MUST distinguish whole-request failure from item failure.
