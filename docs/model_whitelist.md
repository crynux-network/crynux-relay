# Model Whitelist

Relay MAY restrict which normalized model names can be created and dispatched.

## Configuration

`task.model_whitelist_enabled` controls enforcement.

- When `task.model_whitelist_enabled` is `false`, Relay MUST NOT reject creation or abort matching because of the model whitelist.
- When `task.model_whitelist_enabled` is `true`, Relay MUST allow only model names present in the in-memory whitelist cache.
- When `task.model_whitelist_enabled` is `true` and the whitelist is empty, Relay MUST reject every task creation and MUST abort every queued task that reaches a matching round.

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

Create and matching checks MUST read only the in-memory cache. They MUST NOT lazy-load from the database on the hot path.

Admin add and delete MUST write the database first, then refresh the full in-memory cache from the database.

## Creation

After `ApplyTaskPricing` sets `ModelName`, and when `task.model_whitelist_enabled` is `true`:

- If any required repository is not in the cache, `CreateTask` MUST return an error that single-task creation maps to a `task_args` validation error with message `Model not allowed`.
- Batch creation MUST treat the same error as `permanent_error` with message `Model not allowed`.

## Matching

In each matching round, after the queue-timeout skip and before candidate-set construction, when `task.model_whitelist_enabled` is `true` and any required repository of the queued task is not in the cache:

- Relay MUST set `AbortReason` to `TaskAbortModelNotAllowed`.
- Relay MUST end the task with `TaskEndAborted` and refund the task fee through the existing abort path.
- Relay MUST NOT select a node for that task.

## Admin API

All routes require AdminAuth.

| Method | Path | Body / Query |
|--------|------|--------------|
| `GET` | `/v2/admin/model_whitelist` | Returns `{ "model_names": [...] }` sorted ascending |
| `POST` | `/v2/admin/model_whitelist` | JSON `{ "model_name": "..." }` |
| `DELETE` | `/v2/admin/model_whitelist` | Query `model_name=` |

`DELETE` uses a query parameter so model names that contain `/` do not require path encoding.
