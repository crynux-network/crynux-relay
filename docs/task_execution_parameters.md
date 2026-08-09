# Task Execution Parameters

This document specifies the execution-time parameters that Relay derives from completed SD and LLM tasks. These parameters estimate execution duration only. They MUST NOT determine result correctness, validation, consensus, fee settlement, or slashing.

## Workload Fields

For SD tasks, Relay MUST compute:

```
sd_units = num_images * image_width * image_height * steps
```

One SD unit is one pixel executed for one step. Relay MUST store this integer as `SDUnits` when the task is created.

For LLM tasks, Relay MUST inspect canonical `messages`, `tools`, and `template_args` at creation. Relay MUST replace each `{"type":"image","base64":"..."}` block with the same block without `base64`, preserve text, roles, tools, template arguments, and image placeholder structure, encode the result as deterministic UTF-8 JSON, and store its byte length as `LLMTextInputBytes`. Relay MUST store the number of image blocks as `LLMImageCount` and the sum of decoded `width * height` values as `LLMImagePixels`.

Relay MUST decode image dimensions for registered JPEG, PNG, GIF, and WebP input. Invalid base64, an unsupported or unreadable image, a non-positive dimension, or `uint64` pixel accumulation overflow MUST reject task creation as an invalid task argument. Relay MUST NOT replace an invalid image workload with zero.

For LLM tasks, Relay MUST also resolve `max_new_tokens` at creation from an explicit `generation_config.max_new_tokens` value or the configured default, and MUST store that integer as `LLMMaxNewTokens`. SD and SDFT LoRA tasks MUST store `LLMMaxNewTokens` as null.

Relay MUST use stored workload fields in every later estimate and calibration update. Relay MUST use the stored `LLMMaxNewTokens` in every later estimate and Timeout calculation. Relay MUST NOT parse mutable `TaskArgs` again for these values.

An LLM calibration sample MUST use the actual `usage.completion_tokens` from the uploaded `0.json`. Relay MUST accept this value only after the SHA-256 hash of the complete uploaded JSON equals the task's validated `Score`. A missing, incorrectly typed, negative, or otherwise invalid completion-token value MUST cause Relay to log a structured error and skip that calibration sample without changing validation, upload, fee, QoS, or terminal-state processing.

## GPU Parameter Identity

Relay MUST maintain one parameter record for each exact `(GPUName, GPUVram)` pair. The record MUST contain:

- SD `seconds_per_sd_pixel_step`.
- LLM `constant_seconds`.
- LLM `seconds_per_input_byte`.
- LLM `seconds_per_output_token`.
- LLM `model_switch_seconds`.
- LLM `seconds_per_image`.
- LLM `seconds_per_megapixel`.
- LLM formula version.
- Independent SD and LLM cumulative successful-sample counts.
- The persisted LLM weighted least-squares matrices.

Relay MUST NOT split parameter records by model ID, model architecture, dtype, quantization, scheduler, or another model setting. This grouping deliberately ignores execution-speed differences between models on the same GPU variant. Relay MUST limit the resulting estimation error to queue priority and execution Timeout. It MUST NOT affect validation, consensus, fee settlement, or slashing.

## Execution GPU Snapshot

After the transaction that moves a task to `TaskStarted` commits, Relay MUST write the selected node's already-loaded `GPUName` and `GPUVram` to an independent in-memory execution GPU snapshot keyed by `TaskIDCommitment`. Relay MUST use the same node data already loaded by matching and MUST NOT add a node, task, or calibration database query for this write.

The execution GPU snapshot store MUST remain enabled when task tracing is disabled. It MUST NOT use `task_tracing_duration_days`.

Calibration MUST use the snapshot that records the GPU variant at execution start. If the snapshot is absent, Relay MUST skip the sample. Relay MUST NOT read the selected node's current row as a replacement. A missing snapshot MUST NOT affect task execution, validation, result upload, fees, QoS, or terminal-state processing.

Relay MUST remove a snapshot after the task can no longer provide a calibration sample. For an LLM `TaskEndGroupRefund`, Relay MUST retain the snapshot until a same-score group result supplies verified completion tokens or the group can no longer supply them. Background cleanup MUST remove snapshots older than the maximum remaining lifecycle after `TaskStarted`:

```
max_execution_timeout_seconds
    + app_validation_timeout_seconds
    + result_upload_timeout_seconds
```

SDFT LoRA MUST NOT write an execution GPU calibration snapshot and MUST NOT update SD or LLM execution parameters.

## Valid Calibration Samples

Actual execution duration MUST be:

```
actual_execution_seconds = ScoreReadyTime - StartTime
```

SD calibration MUST use only these validation-confirmed outcomes:

- A single task entering `TaskValidated`.
- Each group task entering `TaskGroupValidated`.
- Each matching duplicate entering `TaskEndGroupRefund`.

LLM calibration MUST occur only after verified result upload:

- A task entering `TaskEndSuccess` after its uploaded JSON hash matches its validated score.
- A task entering `TaskEndGroupSuccess` after the same verification.
- A `TaskEndGroupRefund` task whose score equals that successful group upload's score. This sample MUST use the refund task's own input bytes, execution duration, and execution GPU snapshot and MUST reuse only the verified completion-token count.

If the group representative does not upload a verified result, `TaskEndGroupRefund` tasks MUST NOT provide LLM samples. A refund task with a different score MUST NOT use the uploaded completion-token count.

`TaskEndInvalidated`, `TaskEndAborted`, `TaskErrorReported`, failed validation, execution timeout, and any task without verifiable LLM completion tokens MUST NOT update execution parameters, sample counts, or LLM fitting matrices.

## Parameter Updates

For SD, Relay MUST subtract configured fixed overhead from actual execution duration, divide the non-negative remainder by `SDUnits`, and update the rate with:

```
new_rate = calibration_alpha * sample_rate + (1 - calibration_alpha) * old_rate
```

For LLM, Relay MUST fit:

```
actual_execution_seconds =
    constant_seconds
    + seconds_per_input_byte * text_input_bytes
    + seconds_per_output_token * actual_completion_tokens
    + model_switch_seconds * model_switched
    + seconds_per_image * image_count
    + seconds_per_megapixel * (image_pixels / 1000000)
```

Relay MUST update the six-dimensional LLM `XᵀX` and `Xᵀy` state by multiplying prior state by `1 - calibration_alpha` and adding the new sample contribution multiplied by `calibration_alpha`. The ridge equation MUST add `regularization * configured_initial_parameters` to `Xᵀy` and `regularization` to the matrix diagonal. Unobserved image or model-switch dimensions MUST therefore remain at their configured positive initial values. All fitted coefficients MUST be non-negative. When the current predicted execution seconds is greater than `0`, a sample's positive residual above the configured maximum residual multiple MUST be truncated before it updates shared parameters. When the current predicted execution seconds is not greater than `0`, Relay MUST NOT truncate that sample. Relay MUST solve for finite coefficients from the updated matrices before committing the sample. If the solve fails or any coefficient is not finite, Relay MUST leave the previous coefficients, matrices, and successful-sample count unchanged.

`calibration_alpha` MUST be greater than `0` and less than `1`. SD EWMA and LLM exponentially weighted least squares MUST use the same configured value.

Every runtime configuration template MUST explicitly define positive SD and LLM initial parameters, fixed overhead, `calibration_warmup_success_samples`, a one-hour calibration flush interval, LLM regularization, and the maximum positive residual multiple. LLM warmup samples MUST be at least `3`. Missing or invalid values MUST prevent Relay startup.

## Initialization and Cold Start

When an exact GPU variant has no valid sample for a task type, Relay MUST initialize its parameters from the simple arithmetic mean of all other GPU names with the same exact `GPUVram` and at least one valid sample for that task type. Every qualifying GPU variant MUST have equal weight. Relay MUST NOT weight this mean by cumulative sample count. If no qualifying record exists, Relay MUST use configured initial parameters.

Inherited parameters MUST NOT increase the target variant's own successful-sample count and MUST NOT populate its own LLM fitting matrices. Cumulative successful-sample counts MUST be used only to determine whether a record has samples and whether the exact GPU variant has completed cold start.

An SD GPU variant completes cold start only when its own successful-sample count reaches `calibration_warmup_success_samples`.

An LLM GPU variant completes cold start only when its own successful-sample count reaches `calibration_warmup_success_samples` and its record uses the current LLM formula version. Full rank across all six dimensions MUST NOT be required.

Before cold start completes, Relay MUST calculate the current task's full predicted duration from configured initial parameters and from every other GPU name with the same exact `GPUVram` that has completed cold start for that task type. Relay MUST use the maximum complete prediction. For LLM, Relay MUST compare complete predictions and MUST NOT combine the maximum of individual coefficients. The target GPU variant's own incomplete fitted parameters MUST NOT participate. If no same-VRAM variant has completed cold start, Relay MUST use the configured initial prediction.

After cold start completes, Relay MUST use the exact GPU variant's own parameters. Relay restart MUST produce the same readiness decision from persisted sample counts and fitting matrices.

## Cache, Persistence, and Metrics

Relay MUST load all persisted GPU execution parameter records once at startup. Runtime reads and updates MUST use the in-memory cache. Task creation, candidate filtering, node selection, dispatch, and execution-time calculation MUST NOT query the calibration table.

When a persisted record has an older LLM formula version, Relay MUST preserve its SD parameter and SD sample count, reset its LLM fitting state and LLM sample count, initialize all six LLM coefficients from required production configuration, assign the current formula version, and mark the record dirty for the normal flush process.

Each valid sample MUST update in-memory parameters, fitting state, successful-sample count, and a dirty version. Relay MUST batch-upsert dirty records at the configured flush interval of one hour and MUST perform one bounded flush during normal shutdown. A failed flush MUST retain dirty records for retry. A concurrent update during a flush MUST remain dirty after that flush completes.

Relay metrics MUST expose base units:

- SD: seconds per pixel-step.
- LLM constant: seconds.
- LLM text input coefficient: seconds per input byte.
- LLM output coefficient: seconds per output token.
- LLM model-switch coefficient: seconds per switch.
- LLM image-count coefficient: seconds per image.
- LLM image-pixel coefficient: seconds per megapixel.

The sample-count metric MUST expose cumulative successful samples by task type and exact GPU variant. Sample counts MUST NOT be used as cross-GPU aggregation weights.

Grafana MUST display the LLM text input and output coefficients as seconds per 5,000 units by multiplying the base metrics by `5000`. The model-switch, image-count, and image-megapixel coefficients MUST be displayed in their base units. Grafana MUST display the SD coefficient as seconds per 512×512 image-step by multiplying the base metric by `512 * 512`. Relay storage, calculation, and `/metrics` output MUST remain in base units.

Queue ordering MUST read these parameters as specified in [task-pricing.md](./task-pricing.md). Execution Timeout calculation MUST read these parameters as specified in [task_timeout.md](./task_timeout.md).
