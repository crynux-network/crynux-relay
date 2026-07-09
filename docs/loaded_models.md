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

- `model_id`: the normalized model identifier from `InferenceTask.ModelIDs`.
- `min_vram`: the smallest `nodes.gpu_vram` value among nodes that have successfully executed a task containing the model.

`model_id` MUST be unique. `min_vram` MUST be stored in GB, matching the `nodes.gpu_vram` unit recorded at node join.

Relay SHALL keep an in-memory pending update cache for successful task executions that have not yet been flushed to `loaded_models`.

## Update Rules

When a task status transition to `TaskEndSuccess`, `TaskEndGroupSuccess`, or `TaskEndGroupRefund` has committed successfully, Relay MUST record the task's model IDs and selected node VRAM in the in-memory pending update cache. Relay MUST NOT perform `loaded_models` database writes inside the task status transaction.

The pending update cache MUST keep the minimum observed VRAM per model ID. For each model ID in the task's `ModelIDs` list:

- If the model ID is not present in the pending cache, Relay MUST record it with `min_vram` set to the selected node's `gpu_vram`.
- If the model ID is already present in the pending cache, Relay MUST update the pending value only when the selected node's `gpu_vram` is lower than the cached value.

The `min_vram` value for a model MUST be monotonically non-increasing across successful task executions.

## Background Flush

Relay SHALL run one background loaded model flusher goroutine.

The flusher MUST periodically take the pending cache and upsert it into `loaded_models`:

- If the model ID does not exist in `loaded_models`, Relay MUST create it with `min_vram` set to the pending value.
- If the model ID already exists in `loaded_models`, Relay MUST update `min_vram` only when the pending value is lower than the stored value.

If a flush fails, Relay MUST merge the failed pending values back into the in-memory pending update cache.

## Historical Backfill

The migration that creates `loaded_models` MUST backfill existing successful task executions from `inference_tasks`.

The backfill MUST include tasks with status `TaskEndSuccess`, `TaskEndGroupSuccess`, or `TaskEndGroupRefund` and a non-empty `selected_node`. The backfill MUST join `inference_tasks.selected_node` to `nodes.address`, split the stored `model_ids` value, and write the minimum `nodes.gpu_vram` per model ID.

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
      "min_vram": 24
    }
  ]
}
```

The endpoint MUST merge persisted `loaded_models` rows with the in-memory pending update cache before building the response. The response `data` array MUST be ordered by `model_id` ascending.
