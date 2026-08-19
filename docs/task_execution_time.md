# Task Execution Time Coefficients API

This document specifies the public Relay APIs that return SD and LLM execution-time coefficients. Coefficient production is specified only in [task_execution_parameters.md](./task_execution_parameters.md). These APIs MUST return the selected base coefficients. They MUST NOT receive workload values, MUST NOT return a combined `estimated_seconds`, and MUST NOT apply Timeout selection or `timeout_multiplier`.

SDFT LoRA MUST NOT have an execution-time coefficient API.

## Endpoints

Relay MUST expose:

```text
GET /v2/models/sd/execution-time
GET /v2/models/llm/execution-time
```

Both endpoints MUST be public and MUST NOT require authentication.

A request that names a model absent from `loaded_models` MUST still return coefficients. Relay MUST use matching calibration records when they exist and MUST use configured initial parameters when they do not. These endpoints MUST NOT return 404.

## Query Parameters

Both endpoints MUST accept:

| Parameter | Required | Rules |
|-----------|----------|-------|
| `model` | yes | Non-empty. Relay MUST apply `NormalizeModelName`. |
| `dtype` | no | When present and non-empty, Relay MUST store the lowercased value as `RequestedDType`. When absent or empty, `RequestedDType` MUST be `auto`. |
| `quantize_bits` | no | Unsigned integer. When present, Relay MUST use that value as `QuantizeBits`. When absent, `QuantizeBits` MUST be `0`. A negative value MUST return 400. |
| `min_vram` | exclusive | Positive integer GB. |
| `gpu_name` | exclusive with `gpu_vram` | Non-empty GPU name. |
| `gpu_vram` | exclusive with `gpu_name` | Positive integer GB. |

The SD endpoint MUST also accept optional `variant`. When present and non-empty, Relay MUST store the lowercased value as `ModelVariant`. When absent or empty, `ModelVariant` MUST be empty. The LLM endpoint MUST NOT accept `variant`. LLM `ModelVariant` MUST be empty.

A request MUST use exactly one selection mode:

1. `min_vram` without `gpu_name` and without `gpu_vram`.
2. `gpu_name` and `gpu_vram` together, without `min_vram`.

Relay MUST return 400 when:

- `model` is missing or empty.
- both modes are present.
- neither mode is present.
- only `gpu_name` or only `gpu_vram` is present.
- `min_vram` is `0`.
- `gpu_vram` is `0`.
- `gpu_name` is empty after whitespace normalization.
- `quantize_bits` is negative.

`gpu_name` MUST match stored calibration `GPUName` values, including letter case. Relay MUST apply the same whitespace normalization used for node-reported GPU names. After that normalization, the name MUST be compared exactly.

## Parameter Selection

Relay MUST select coefficients from the in-memory execution-parameter cache specified in [task_execution_parameters.md](./task_execution_parameters.md). `min_vram` selection MUST match SD and LLM task creation when `RequiredGPU` is empty. `gpu_name` and `gpu_vram` selection MUST match SD and LLM task creation when `RequiredGPU` and `RequiredGPUVRAM` are set. Relay MUST NOT implement a second selection path and MUST NOT use Timeout selection.

An explicit `dtype` MUST select only records whose `ExecutionDType` equals that value. A requested dtype of `auto` MUST select `auto` records and records that store a reported actual execution dtype.

### `min_vram` mode

Relay MUST build a lookup whose `MinVRAM` equals `min_vram` and whose `RequiredGPU` is empty.

Relay MUST select sampled records with `GPUVram >= min_vram` and a matching model configuration. Relay MUST average the selected records with equal weight. Cumulative successful-sample counts MUST NOT be aggregation weights.

When the model configuration has no sampled records, Relay MUST select the nearest `MinVRAM` interval records among the VRAM-filtered hardware records and MUST average equal-distance records with equal weight. If no candidate exists, Relay MUST use configured initial parameters.

### `gpu_name` and `gpu_vram` mode

Relay MUST build a lookup whose `RequiredGPU` equals the normalized `gpu_name` and whose `RequiredGPUVRAM` equals `gpu_vram`. `MinVRAM` MUST NOT filter hardware records on this path.

Relay MUST first use sampled records whose GPU name equals `gpu_name` and whose VRAM equals `gpu_vram` and whose model configuration matches. When that GPU has no matching model-configuration samples, Relay MUST use other GPU names with the same VRAM and a matching model configuration. When no matching model-configuration sample exists, Relay MUST use the unknown-model nearest `MinVRAM` interval fallback among same-VRAM records, preferring the exact GPU name when that GPU has records. If no candidate exists, Relay MUST use configured initial parameters.

## Response

The response body MUST use the standard Relay v2 response envelope.

SD:

```json
{
  "message": "success",
  "data": {
    "overhead_seconds": 30,
    "seconds_per_sd_pixel_step": 0.00003814697265625
  }
}
```

`overhead_seconds` and `seconds_per_sd_pixel_step` MUST come from the selected calibration records, or from the configured initial SD overhead and initial SD rate when no record is selected.

LLM:

```json
{
  "message": "success",
  "data": {
    "constant_seconds": 30,
    "seconds_per_input_token": 0.0004,
    "seconds_per_output_token": 0.1,
    "model_switch_seconds": 120,
    "seconds_per_image": 10,
    "seconds_per_megapixel": 5
  }
}
```

`constant_seconds`, `seconds_per_output_token`, `model_switch_seconds`, `seconds_per_image`, and `seconds_per_megapixel` MUST use the same units as the in-memory calibration coefficients.

The in-memory cache MUST continue to store LLM text-input cost as `seconds_per_input_byte`. The public LLM response MUST set:

```
seconds_per_input_token = seconds_per_input_byte * 4
```

The factor `4` MUST be a code constant. It MUST NOT be a configuration value. The public response MUST NOT include `seconds_per_input_byte`.

## Downstream Duration

Callers MUST compute duration from the returned coefficients and their own workload.

SD:

```
estimated_seconds =
    overhead_seconds
    + num_images * image_width * image_height * steps * seconds_per_sd_pixel_step
```

LLM:

```
estimated_seconds =
    constant_seconds
    + seconds_per_input_token * input_tokens
    + seconds_per_output_token * output_tokens
    + model_switch_seconds * model_switched
    + seconds_per_image * image_count
    + seconds_per_megapixel * (image_pixels / 1000000)
```

`model_switched` MUST be `1` when the selected node must switch models and `0` otherwise. These APIs MUST NOT accept `input_tokens`, `output_tokens`, `num_images`, width, height, or `steps`.
