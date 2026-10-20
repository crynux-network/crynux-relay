# Loaded Models API

This document specifies the Relay-wide loaded model projection and the public API that exposes it.

## Table Responsibilities

`loaded_models` is the successful-execution projection for models that have completed at least one qualifying terminal task on Relay. It is the data source for `GET /v2/loaded-models`. Rows and `min_vram` values are retained as historical evidence and MUST NOT be deleted when nodes quit or stop holding the model.

`loaded_models` MUST NOT participate in task matching, model distribution, or create-time HuggingFace validation skip decisions. Those paths use `node_models` as specified in [model_distribution.md](./model_distribution.md) and [node_selection.md](./node_selection.md).

`node_models` is the authoritative on-disk inventory of currently joined nodes. Each row references one exact model file in the `model_files` catalog. `node_in_memory_models` is the authoritative in-memory model of each currently joined node. Create-time validation MAY skip a Hub check only when `node_models` contains the exact model file hash of the entry that carries a revision, or a file of the source hash of an entry without a revision. Node counts on `GET /v2/loaded-models` are derived from these two tables and MAY be `0` while the corresponding `loaded_models` row remains.

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
- `execution_dtype`: the dtype the execution reported, which is the task's `ExecutionDType`.
- `required_model_file_hashes`: the sorted concatenation of the exact member hashes.
- `quantization_config`: the canonical JSON of the complete `quantization_config` of the task's TaskArgs; a task without a quantization configuration stores the canonical JSON `null`.
- `model_id`: the normalized model name of the task, which is the LLM model repository or the SD base model name.
- `variant`: the SD base-model variant of the task, or an empty string. LLM rows MUST use an empty `variant`.
- `model_type`: the model category, `sd` or `llm`.
- `min_vram`: the smallest `nodes.gpu_vram` value among nodes that have successfully executed a task with that row identity.

`(required_model_files_hash, model_cache_hash, execution_dtype)` MUST be unique. `model_type` MUST be derived from the task type: `llm` for `TaskTypeLLM` tasks, and `sd` for `TaskTypeSD` and `TaskTypeSDFTLora` tasks. `min_vram` MUST be stored in GB, matching the `nodes.gpu_vram` unit recorded at node join.

Relay SHALL keep an in-memory pending update cache for successful task executions that have not yet been flushed to `loaded_models`.

## Update Rules

When a task status transition to `TaskEndSuccess`, `TaskEndGroupSuccess`, or `TaskEndGroupRefund` has committed successfully, Relay MUST record the task's row identity, model name, variant, model type, quantization configuration, execution model file hashes, and the selected node VRAM in the in-memory pending update cache. Relay MUST NOT perform `loaded_models` database writes inside the task status transaction.

The pending update cache MUST keep the minimum observed VRAM per row identity:

- If the identity is not present in the pending cache, Relay MUST record it with `min_vram` set to the selected node's `gpu_vram`.
- If the identity is already present in the pending cache, Relay MUST update the pending value only when the selected node's `gpu_vram` is lower than the cached value.

The `min_vram` value for an identity MUST be monotonically non-increasing across successful task executions.

## Background Flush

Relay SHALL run one background loaded model flusher goroutine. The flusher MUST run once per hour.

The flusher MUST periodically take the pending cache and upsert it into `loaded_models`:

- If the identity does not exist in `loaded_models`, Relay MUST create the row with all pending values.
- If the identity already exists in `loaded_models`, Relay MUST update `min_vram` only when the pending value is lower than the stored value, and MUST keep every other stored column unchanged.

If a flush fails, Relay MUST merge the failed pending values back into the in-memory pending update cache.

## Node Count Aggregation

The signed AddModelID node API MUST be the authoritative incremental source for on-disk `node_models` rows. The signed joined-node capability synchronization MUST be the authoritative full on-disk inventory source after node startup. The signed node in-memory model report MUST be the authoritative source of `node_in_memory_models`. Download selection completion is derived from the resulting on-disk inventory as specified in [model_distribution.md](./model_distribution.md).

Task start MUST NOT create or modify `node_models` rows and MUST NOT modify `node_in_memory_models` rows.

For each `loaded_models` row, Relay SHALL derive two node counts:

- On-disk node count: the number of distinct node addresses whose on-disk model files contain every exact member hash of the row's `required_model_file_hashes`.
- In-memory node count: the number of distinct node addresses whose in-memory model has the row's `execution_dtype` and `model_cache_hash`, and whose in-memory model files contain every exact member hash of the row's `required_model_file_hashes`.

Relay MUST load the node on-disk inventory and in-memory models in bulk and MUST NOT run per-row database queries or SQL pattern matching. Because `node_models` and `node_in_memory_models` rows are deleted when a node quits, the counts cover only currently joined nodes, regardless of node status. A loaded model with no matching nodes MUST report both counts as `0` and MUST remain in `loaded_models`.

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
      "variant": "",
      "model_type": "llm",
      "execution_dtype": "bfloat16",
      "quantization_config": null,
      "min_vram": 24,
      "in_memory_node_count": 1,
      "on_disk_node_count": 3
    },
    {
      "model_id": "crynux-network/sdxl-turbo",
      "variant": "fp16",
      "model_type": "sd",
      "execution_dtype": "float16",
      "quantization_config": null,
      "min_vram": 14,
      "in_memory_node_count": 0,
      "on_disk_node_count": 2
    }
  ]
}
```

The endpoint MUST read only persisted `loaded_models` rows. Pending cache entries MUST become visible through the endpoint after the background flusher writes them to the database. The response `data` array MUST be ordered by `model_id` ascending, then `variant` ascending, then `execution_dtype` ascending, then row id ascending.

`quantization_config` MUST be the stored canonical JSON value. `in_memory_node_count` and `on_disk_node_count` MUST be filled from the cached node count aggregation of the same row.
