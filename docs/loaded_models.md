# Loaded Models API

This document specifies the Relay-wide loaded model projection and the public API that exposes it.

## Loaded Model Definition

A model is a loaded model after at least one Relay task that includes the model reaches one of these terminal statuses:

- `TaskEndSuccess`
- `TaskEndGroupSuccess`
- `TaskEndGroupRefund`

`TaskEndGroupRefund` MUST count as successful model execution because the task result passed validation group score comparison and the node completed the model execution, even though another validation group task uploaded the final result.

Tasks ending in `TaskEndInvalidated`, `TaskEndAborted`, or any non-terminal status MUST NOT create or update loaded model records.

## Projection Table

Relay SHALL store loaded model data in the `loaded_models` table.

Each `loaded_models` row MUST contain:

- `model_id`: the huggingface model name of a base model, extracted from `InferenceTask.ModelIDs`.
- `model_type`: the model category, `sd` or `llm`.
- `min_vram`: the smallest `nodes.gpu_vram` value among nodes that have successfully executed a task containing the model.

`InferenceTask.ModelIDs` entries use the internal dispatch format `<usage>:<name>` or `<usage>:<name>+<variant>`, where `<usage>` is `base`, `lora`, or `controlnet`. The projection MUST record only `base` entries. The stored `model_id` MUST be the `<name>` part with the `base:` prefix and the `+<variant>` suffix removed, so it can be used directly as a huggingface model identifier. Entries whose `<name>` is a URL MUST NOT be recorded.

`model_type` MUST be derived from the task type: `llm` for `TaskTypeLLM` tasks, and `sd` for `TaskTypeSD` and `TaskTypeSDFTLora` tasks.

`model_id` MUST be unique. `min_vram` MUST be stored in GB, matching the `nodes.gpu_vram` unit recorded at node join.

The migration that creates `loaded_models` MUST create an empty table. The projection MUST be populated only by successful task executions after the table is created.

Relay SHALL keep an in-memory pending update cache for successful task executions that have not yet been flushed to `loaded_models`.

## Update Rules

When a task status transition to `TaskEndSuccess`, `TaskEndGroupSuccess`, or `TaskEndGroupRefund` has committed successfully, Relay MUST record the task's base model huggingface names, the task's model type, and the selected node VRAM in the in-memory pending update cache. Relay MUST NOT perform `loaded_models` database writes inside the task status transaction.

The pending update cache MUST keep the minimum observed VRAM per model ID. For each base model huggingface name extracted from the task's `ModelIDs` list:

- If the model ID is not present in the pending cache, Relay MUST record it with `min_vram` set to the selected node's `gpu_vram`.
- If the model ID is already present in the pending cache, Relay MUST update the pending value only when the selected node's `gpu_vram` is lower than the cached value.

The `min_vram` value for a model MUST be monotonically non-increasing across successful task executions.

## Background Flush

Relay SHALL run one background loaded model flusher goroutine. The flusher MUST run once per hour.

The flusher MUST periodically take the pending cache and upsert it into `loaded_models`:

- If the model ID does not exist in `loaded_models`, Relay MUST create it with `model_type` and `min_vram` set to the pending values.
- If the model ID already exists in `loaded_models`, Relay MUST update `min_vram` only when the pending value is lower than the stored value, and MUST keep the stored `model_type` unchanged.

If a flush fails, Relay MUST merge the failed pending values back into the in-memory pending update cache.

## Node Count Aggregation

Each `node_models` row MUST carry an `hf_model_id` column holding the huggingface base model name derived from the row's dispatch `model_id` with `BaseModelHuggingFaceID`. Rows whose dispatch ID is not a huggingface base model (`lora:` and `controlnet:` entries, and URL-based names) MUST store an empty `hf_model_id`. All `node_models` write paths MUST populate the column at row creation through the `NewNodeModel` constructor.

The `node_models` table MUST have a composite index on `(hf_model_id, node_address)`.

For each huggingface base model, Relay SHALL derive two node counts from `node_models`:

- On-disk node count: the number of distinct node addresses that have at least one `node_models` row with the model's `hf_model_id`.
- In-memory node count: the number of distinct node addresses that have at least one such row with `in_use` set.

Node addresses MUST be counted distinctly because one node may hold several variants of the same base model. The counts MUST be computed by a single grouped aggregation query on `hf_model_id`, and MUST NOT use per-model queries or SQL pattern matching. Because `node_models` rows are deleted when a node quits, the counts cover only currently joined nodes, regardless of node status.

Relay SHALL cache the aggregated counts in memory with a 1 minute TTL. The cache MUST be refreshed lazily on the first request after expiry, so API traffic produces at most one aggregation query per TTL window.

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
      "min_vram": 24,
      "in_memory_node_count": 1,
      "on_disk_node_count": 3
    }
  ]
}
```

The endpoint MUST read only persisted `loaded_models` rows. Pending cache entries MUST become visible through the endpoint after the background flusher writes them to the database. The response `data` array MUST be ordered by `model_id` ascending.

`in_memory_node_count` and `on_disk_node_count` MUST be filled from the cached node count aggregation, matched by `model_id`. A loaded model with no matching aggregation entry MUST report both counts as `0`.
