# Quantization Platform Support

This document specifies how Relay detects Linux-CUDA-only quantization loads, stores `RequireLinuxCuda`, and filters matching and model distribution by GPU-name platform suffix. It is a scheduling constraint. It MUST NOT define gpt-task or sd-task execution adapters, and MUST NOT be implemented as a model-ID allowlist.

## Platform Suffixes

Relay MUST parse the platform from a node's GPU name as the substring after the first `+`. Recognized Linux CUDA platforms for `RequireLinuxCuda` are exactly `docker` and `Linux`. `Windows`, `Darwin`, any other suffix, and a missing `+` suffix are not Linux CUDA platforms.

## Supported Quantization Loads by Platform

| Load | Windows CUDA | Linux / Docker CUDA | Darwin (MPS) |
|------|--------------|---------------------|--------------|
| LLM BitsAndBytes / FP8 | Allowed by Relay scheduling | Allowed by Relay scheduling | Subject to gpt-task and installed dependencies; not selected by `RequireLinuxCuda` |
| LLM AWQ / GPTQ / GPTQModel | Excluded by `RequireLinuxCuda` | Allowed by Relay scheduling | Excluded by `RequireLinuxCuda` |
| Image BitsAndBytes / GGUF | Allowed by Relay scheduling | Allowed by Relay scheduling | sd-task MUST NOT platform-block; success depends on dependencies and MPS runtime |
| Image Nunchaku Lite | Excluded by `RequireLinuxCuda` | Allowed by Relay scheduling | Excluded by `RequireLinuxCuda` |

Relay scheduling filters MUST enforce only the rows that set `RequireLinuxCuda`. Loads that are allowed by Relay on a platform MAY still fail at the node when dependencies or kernels are missing.

## RequireLinuxCuda Detection

At task creation, after model-file requirements are derived, Relay MUST set `InferenceTask.RequireLinuxCuda` and MUST NOT change it afterward.

Relay MUST set the flag to true when any of the following holds:

1. TaskArgs `quantization_config.quant_method` (case-insensitive) is `awq`, `gptq`, or `gptqmodel`.
2. For an LLM task, the Hub `config.json` of the primary model repository (at the resolved revision when present) contains `quantization_config.quant_method` equal to `awq`, `gptq`, or `gptqmodel` (case-insensitive).
3. For an SD or SDFT LoRA task, Hub Diffusers metadata of the base model shows Nunchaku Lite: Relay MUST read `model_index.json` and the config of model components it names (at least `transformer` when present), and MUST treat `quantization_config.quant_method` values `nunchaku` and `nunchaku_lite` (case-insensitive) as Linux-CUDA-only. Relay MUST NOT decide from model-ID substrings.

TaskArgs methods `bitsandbytes`, `fp8`, empty, or null MUST NOT set the flag by themselves.

When TaskArgs already requires Linux CUDA, Relay MAY skip Hub detection for the flag. When TaskArgs does not, Relay MUST consult the cache and, on a miss, fetch the light Hub JSON files above. A Hub outage or missing cache entry in that case MUST fail task creation with the same permanent or temporary classification used for Hub model validation.

## Cache

Hub detection results MUST be cached:

1. Process-local TTL cache keyed by runtime kind, repository, and revision.
2. Persistent `model_runtime_constraints` rows keyed by repository and revision, storing `require_linux_cuda`, detection source, and timestamps.

Create MUST read the persistent cache before Hub. A successful Hub detection MUST write both caches. When on-disk inventory already holds the weights and create skips Hub weight or revision validation, Relay MUST still resolve `RequireLinuxCuda` from cache or a light config fetch.

## Matching and Distribution

`RequireLinuxCuda` MUST be part of the task requirement signature used for candidate-set reuse.

Matching MUST drop every node that is not a Linux CUDA platform when the flag is true, including when `RequiredGPU` is set.

Model distribution demand groups MUST include the flag. Only Linux CUDA nodes MAY count toward coverage or be selected as download targets for a demand group with the flag set.
