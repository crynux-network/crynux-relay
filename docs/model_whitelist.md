# Model Whitelist

Relay MAY restrict which normalized model names can be created, dispatched, reported into node inventory, and exposed through the loaded models projection.

## Configuration

`task.model_whitelist_enabled` controls enforcement.

- When `task.model_whitelist_enabled` is `false`, Relay MUST NOT reject creation or abort matching because of the model whitelist, MUST NOT filter node model inventory reports, MUST NOT filter loaded model writes or `GET /v2/loaded-models`, and MUST NOT prune inventory or loaded model rows for whitelist reasons.
- When `task.model_whitelist_enabled` is `true`, Relay MUST allow only model names present in the in-memory whitelist cache for task creation, matching, node on-disk and in-memory model reports, loaded model projection writes, and `GET /v2/loaded-models`.
- When `task.model_whitelist_enabled` is `true` and the whitelist is empty, Relay MUST reject every task creation, MUST abort every queued task that reaches a matching round, MUST accept no on-disk or in-memory model inventory rows, MUST write no loaded model rows, MUST return an empty `GET /v2/loaded-models` list, and MUST prune every non-whitelisted inventory and loaded model row as specified below.

`task.model_whitelist_enabled` is independent of `task.task_whitelist_enabled` and `task.node_name_whitelist_enabled`.

Changing `task.model_whitelist_enabled` requires a configuration change and Relay restart.

## Matching Key

Whitelist keys are normalized HuggingFace repository names after `models.NormalizeModelName`. Matching MUST be exact equality on the normalized name. Relay MUST NOT use SQL `LIKE`, prefix matching, or wildcards.

When `task.model_whitelist_enabled` is `true`, Relay MUST require every repository below to be present in the whitelist:

- LLM: `model` (`InferenceTask.ModelName`)
- SDFT: `model.name` (`InferenceTask.ModelName`)
- SD: `base_model.name` and every `components.*.repo` that is a required model file

An SD task with a whitelisted base model and a non-whitelisted component repository MUST be rejected. LoRA weight references (`lora.model`) are not required model files and MUST NOT be checked by this whitelist.

Admin add and delete inputs MUST be normalized with the same rules before persistence. Empty names after normalization MUST be rejected.

## Cache

Relay MUST load the full `model_whitelists` table into memory during process startup, before HTTP and matching start.

Startup load failure MUST fail process startup.

Create, matching, inventory, and loaded-model checks MUST read only the in-memory cache. They MUST NOT lazy-load from the database on the hot path. Every whitelist membership check MUST use `IsModelWhitelisted` (or an equivalent that normalizes with `models.NormalizeModelName` before exact cache lookup). Relay MUST NOT compare raw repository strings with hand-written case folding or SQL `LIKE`.

Admin add and delete MUST write the database first, then refresh the full in-memory cache from the database.

After a successful startup load or admin refresh, when `task.model_whitelist_enabled` is `true`, Relay MUST prune non-whitelisted inventory and loaded model projection data as specified in [Inventory and Loaded Models](#inventory-and-loaded-models).

## Creation

After `ApplyTaskPricing` sets `ModelName`, and when `task.model_whitelist_enabled` is `true`:

- If any required repository is not in the cache, `CreateTask` MUST return an error that single-task creation maps to a `task_args` validation error with message `Model not allowed`.
- Batch creation MUST treat the same error as `permanent_error` with message `Model not allowed`.

## Matching

In each matching round, after the queue-timeout skip and before candidate-set construction, when `task.model_whitelist_enabled` is `true` and any required repository of the queued task is not in the cache:

- Relay MUST set `AbortReason` to `TaskAbortModelNotAllowed`.
- Relay MUST end the task with `TaskEndAborted` and refund the task fee through the existing abort path.
- Relay MUST NOT select a node for that task.

## Inventory and Loaded Models

When `task.model_whitelist_enabled` is `true`:

- The signed AddModelID report for a non-whitelisted repository MUST succeed without creating a `node_models` row and MUST NOT upsert that model file into the `model_files` catalog for that report.
- Node join and joined-node capability synchronization MUST treat only whitelisted repositories in the reported model file list as the authoritative on-disk inventory. Non-whitelisted reported entries MUST be treated as absent, so existing `node_models` rows for those repositories on that node MUST be deleted by the inventory replacement.
- A non-empty signed in-memory model report that includes any non-whitelisted repository MUST be rejected.
- Successful task execution MUST NOT record a loaded model whose `model_id` is not whitelisted.
- `GET /v2/loaded-models` MUST omit every row whose `model_id` is not whitelisted.

After each successful whitelist cache load or refresh while enforcement is enabled, Relay MUST:

1. Delete every `node_models` row whose catalog repository is not whitelisted.
2. Clear every `node_in_memory_models` row whose model files include a non-whitelisted repository.
3. Delete every `loaded_models` row whose `model_id` is not whitelisted.
4. Refresh the scheduling index of every node whose inventory or in-memory model changed, and invalidate the loaded-models view cache.

Adding a model name to the whitelist MUST NOT restore previously dropped inventory for online nodes. A node that already holds the files on disk MUST report them again through a later capability synchronization or join before those files enter `node_models`.

## Admin API

All routes require AdminAuth.

| Method | Path | Body / Query |
|--------|------|--------------|
| `GET` | `/v2/admin/model_whitelist` | Returns `{ "model_names": [...] }` sorted ascending |
| `POST` | `/v2/admin/model_whitelist` | JSON `{ "model_name": "..." }` |
| `DELETE` | `/v2/admin/model_whitelist` | Query `model_name=` |

`DELETE` uses a query parameter so model names that contain `/` do not require path encoding.
