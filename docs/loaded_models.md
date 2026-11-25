# Loaded Models API

This document specifies the Relay-wide loaded model projection and the public API that exposes it.

## Table Responsibilities

`loaded_models` is the successful-execution projection for models that have completed at least one qualifying terminal task on Relay. Each row keeps the exact-file identity of one successful execution configuration. It is the data source for `GET /v2/loaded-models`, which aggregates rows into public views as specified in [Public View Aggregation](#public-view-aggregation). Rows and `min_vram` values are retained as historical evidence and MUST NOT be deleted when nodes quit or stop holding the model. When `task.model_whitelist_enabled` is `true`, Relay MUST delete `loaded_models` rows whose `model_id` is not whitelisted during whitelist cache load and refresh pruning, as specified in [model_whitelist.md](./model_whitelist.md).

`loaded_models` MUST NOT participate in task matching, model distribution, or create-time HuggingFace validation skip decisions. Those paths use `node_models` as specified in [model_distribution.md](./model_distribution.md) and [node_selection.md](./node_selection.md).

`node_models` is the authoritative on-disk inventory of currently joined nodes. Each row references one exact model file in the `model_files` catalog. `node_in_memory_models` is the authoritative in-memory model of each currently joined node. Create-time Hub skip and revision freezing are specified in [model_distribution.md](./model_distribution.md) and [task_batch_api.md](./task_batch_api.md): new tasks freeze every member to a commit SHA, MAY reuse a commit already held by an `Available` or `Busy` node, and MAY skip Hub resolution only for an already-held exact commit. Create-time validation MUST NOT leave a new task as a revision-less source requirement merely because some revision of that source exists in `node_models`. Node counts on `GET /v2/loaded-models` are derived from these two tables and MAY be `0` while the corresponding `loaded_models` rows remain.

## Loaded Model Definition

A model is a loaded model after at least one Relay task that has execution model files reaches one of these terminal statuses:

- `TaskEndSuccess`
- `TaskEndGroupSuccess`
- `TaskEndGroupRefund`

`TaskEndGroupRefund` MUST count as successful model execution because the task result passed validation group score comparison and the node completed the model execution, even though another validation group task uploaded the final result.

Tasks ending in `TaskEndInvalidated`, `TaskEndAborted`, or any non-terminal status MUST NOT create or update loaded model records. A task without `ExecutionModelFileHashes` MUST NOT create or update loaded model records.

## Projection Table

Relay SHALL store loaded model data in the `loaded_models` table.

Each `loaded_models` row MUST contain:

- `required_model_files_hash`: the SHA-256 of the sorted concatenation of the task's `ExecutionModelFileHashes`, which identifies the exact model file set that was executed.
- `model_cache_hash`: the task's `RequestedModelCacheHash`.
- `model_cache_config`: the canonical JSON text of the task's requested model cache config, which is the exact byte sequence that `model_cache_hash` is computed from (for example, `task_kind: "video"`, `input_mode`, `dtype`, `base_model`, `components`, `loras`, and `quantization_config` for video tasks). The cache config `dtype` MUST be a concrete value (`float16`, `bfloat16`, or `float32`). Relay MUST store it in a non-null `longtext` column and MUST take it from `DeriveTaskModelRequirements(...).RequestedModelCacheConfig`, exposed as `InferenceTask.RequestedModelCacheConfig()`, and MUST NOT assemble it from individual components. The stored text MUST keep the canonical encoding with sorted object keys and no insignificant whitespace. The SHA-256 of the stored text MUST equal `model_cache_hash`.
- `dtype`: the task's concrete dtype from TaskArgs at creation (`float16`, `bfloat16`, or `float32`, stored lowercased).
- `required_model_file_hashes`: the sorted concatenation of the exact member hashes.
- `quantization_config`: the canonical JSON of the complete `quantization_config` of the task's TaskArgs; a task without a quantization configuration stores the canonical JSON `null`.
- `model_id`: the normalized model name of the task, which is the LLM model repository or the SD/video base model name.
- `model_type`: the model category, `sd`, `llm`, or `video`.
- `min_vram`: the smallest `nodes.gpu_vram` value among nodes that have successfully executed a task with that row identity.

`(required_model_files_hash, model_cache_hash)` MUST be unique. `model_type` MUST be derived from the task type: `llm` for `TaskTypeLLM` tasks, `video` for `TaskTypeVideo` tasks, and `sd` for `TaskTypeSD` and `TaskTypeSDFTLora` tasks. `min_vram` MUST be stored in GB, matching the `nodes.gpu_vram` unit recorded at node join.

A row identity is defined only by the exact model files and the model cache hash.

Relay SHALL keep an in-memory pending update cache for successful task executions that have not yet been flushed to `loaded_models`.

## Update Rules

When a task status transition to `TaskEndSuccess`, `TaskEndGroupSuccess`, or `TaskEndGroupRefund` has committed successfully, Relay MUST record the task's row identity, model name, model type, quantization configuration, model cache config, execution model file hashes, and the selected node VRAM in the in-memory pending update cache. Relay MUST NOT record the pending update when the derived model cache config does not hash to the task's `RequestedModelCacheHash`. When `task.model_whitelist_enabled` is `true` and the task `model_id` is not whitelisted, Relay MUST NOT record the pending update. Relay MUST NOT perform `loaded_models` database writes inside the task status transaction.

The pending update cache MUST keep the minimum observed VRAM per row identity:

- If the identity is not present in the pending cache, Relay MUST record it with `min_vram` set to the selected node's `gpu_vram`.
- If the identity is already present in the pending cache, Relay MUST update the pending value only when the selected node's `gpu_vram` is lower than the cached value.

The `min_vram` value for an identity MUST be monotonically non-increasing across successful task executions.

## Background Flush

Relay SHALL run one background loaded model flusher goroutine. The flusher MUST run once per hour.

The flusher MUST periodically take the pending cache and upsert it into `loaded_models`:

- If the identity does not exist in `loaded_models`, Relay MUST create the row with all pending values, including `model_cache_config`.
- If the identity already exists in `loaded_models`, Relay MUST update `min_vram` only when the pending value is lower than the stored value, and MUST keep every other stored column, including `model_cache_config`, unchanged.

If a flush fails, Relay MUST merge the failed pending values back into the in-memory pending update cache.

## Public View Aggregation

The public loaded model view is identified by its public fields: `model_id`, `model_type`, `dtype`, `quantization_config`, and for SD tasks `components` and `lora`, and for video tasks `components` and `loras`. Relay MUST derive those public fields from each member row's stored `model_cache_config` and `quantization_config`. Task matching and scheduling continue to use `RequestedModelCacheHash`, which for video tasks includes `input_mode`.

Relay MUST aggregate `loaded_models` rows into views as follows:

- Rows with different `required_model_files_hash` values that share the same public identity MUST be merged into one view when they differ only in model file revisions.
- For video tasks, rows whose `model_cache_hash` values differ only by `input_mode` in `model_cache_config` MUST be merged into one view when all other public identity fields match.
- Rows whose public identity fields differ MUST NOT be merged.
- A view's `min_vram` MUST be the minimum `min_vram` across its member rows.
- A view's representative `model_id`, `model_type`, `dtype`, `quantization_config`, and task-type-specific public fields MUST be taken from its first member row in `model_id`, `dtype`, row id order that has a complete `model_cache_config`. A later member row whose public identity fields differ from the representative MUST be skipped and logged as an error.

A row has a complete `model_cache_config` only when the value is non-empty valid JSON and its SHA-256 equals the row's `model_cache_hash`. Relay MUST NOT return a view from a row without a complete `model_cache_config`.

When `task.model_whitelist_enabled` is `true`, Relay MUST exclude rows whose `model_id` is not whitelisted before aggregation.

## Node Count Aggregation

The signed AddModelID node API MUST be the authoritative incremental source for on-disk `node_models` rows, subject to model whitelist filtering in [model_whitelist.md](./model_whitelist.md). The signed joined-node capability synchronization MUST be the authoritative full on-disk inventory source after node startup, subject to the same whitelist filtering. The signed node in-memory model report MUST be the authoritative source of `node_in_memory_models`, and MUST be rejected when enforcement is enabled and any reported repository is not whitelisted. Download selection completion is derived from the resulting on-disk inventory as specified in [model_distribution.md](./model_distribution.md).

Task start MUST NOT create or modify `node_models` rows and MUST NOT modify `node_in_memory_models` rows.

For each public view, Relay SHALL derive two node counts. A node covers a member row when its files contain every exact member hash of the row's `required_model_file_hashes`.

- On-disk node count: the number of distinct node addresses whose on-disk model files cover any member row of the view.
- In-memory node count: the number of distinct node addresses whose `current_model_cache_hash` equals the `model_cache_hash` of any member row of the view and whose in-memory model files cover any member row of the view.

A node that covers several member rows MUST be counted once per view. Relay MUST load the node on-disk inventory and in-memory models in bulk and MUST NOT run per-row database queries or SQL pattern matching. Because `node_models` and `node_in_memory_models` rows are deleted when a node quits, the counts cover only currently joined nodes, regardless of node status. A view with no matching nodes MUST report both counts as `0` and its rows MUST remain in `loaded_models`.

Relay SHALL cache the loaded model views, including the counts, in memory with a 1 minute TTL. The cache MUST be refreshed lazily on the first request after expiry, so API traffic produces at most one aggregation per TTL window.

## API Contract

Relay MUST expose loaded models through:

```text
GET /v2/loaded-models
```

The endpoint MUST be public and MUST NOT require authentication.

The response body MUST use the standard Relay v2 response envelope:

```json
{
  "message": "success",
  "data": [
    {
      "model_id": "qwen/qwen3.6-7b",
      "model_type": "llm",
      "dtype": "bfloat16",
      "quantization_config": null,
      "min_vram": 24,
      "in_memory_node_count": 1,
      "on_disk_node_count": 3
    }
  ]
}
```

Every response element MUST include `model_id`, `model_type`, `dtype`, `quantization_config`, `min_vram`, `in_memory_node_count`, and `on_disk_node_count`. The response MUST NOT include `model_cache_hash` or `model_cache_config`.

For `model_type` `sd`, each element MUST also include `components` and `lora`. `components` MUST map each component usage name to an object with `format`, `repo`, and `format_kwargs`, or MUST be `{}` when the task has no components. `lora` MUST be an object with `model`, `weight_file_name`, and `weight`, or `null` when the task has no LoRA.

For `model_type` `video`, each element MUST also include `components` and `loras`. `components` MUST follow the same shape as for SD. `loras` MUST be an array of objects with `model`, `weight_file_name`, `weight`, and `target`, or MUST be `[]` when the task has no LoRA entries.

The endpoint MUST read only persisted `loaded_models` rows. Pending cache entries MUST become visible through the endpoint after the background flusher writes them to the database. The response `data` array MUST contain one element per public view that has a complete `model_cache_config`, ordered by `model_id` ascending, then `dtype` ascending.

`dtype` MUST be the view's concrete stored dtype. `quantization_config` MUST be the stored canonical JSON value. `min_vram` MUST be the view's minimum VRAM across its member rows. `in_memory_node_count` and `on_disk_node_count` MUST be filled from the cached node count aggregation of the same view.
