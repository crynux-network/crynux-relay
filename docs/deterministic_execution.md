# Deterministic Execution

## Scope

This document defines the operational determinism required for task validation and explains why honest LLM executions can rarely produce different results despite matching reported environments.

The validation contract does not require every floating-point intermediate value to be universally reproducible across all hardware and software. It requires results to satisfy the comparison rule for their task type:

- LLM validation scores MUST match exactly.
- SD and SD fine-tuning validation scores MUST satisfy the configured per-image perceptual-hash distance threshold.

The complete validation and slashing behavior is specified in [task_validation_and_slashing.md](./task_validation_and_slashing.md).

## Validation Pool Contract

Every member of one validation group MUST receive the same canonical task arguments, including the model, messages or prompt, generation configuration, seed, dtype, and quantization controls.

LLM validation tasks MUST execute in the hardware pool selected by the primary task. The two validation copies MUST use the primary task's immutable execution GPU name and VRAM as `RequiredGPU` and `RequiredGPUVRAM`.

Execution paths with different floating-point operation orders MUST remain in separate validation pools. Classic layer-sharded execution and tensor-parallel execution MUST NOT share a validation pool. Nodes in one tensor-parallel pool MUST use the same GPU model, GPU count, platform, executor marker, and fallback behavior.

Runner compatibility does not establish exact runtime identity. Relay accepts a runner with the same major version and a minor/patch pair that is lexicographically equal to or newer than the task version. A validation pool can therefore contain compatible but non-identical runner versions.

## Current LLM Execution Controls

The LLM executor establishes repeatable execution by applying all of the following controls before generation:

1. It validates one canonical `GPTTaskArgs` object.
2. It calls `transformers.set_seed` with the task seed for every task. Tensor-parallel execution applies the same seed on every rank.
3. It sets `CUDA_LAUNCH_BLOCKING=1`.
4. It sets `CUBLAS_WORKSPACE_CONFIG=:16:8`.
5. It enables PyTorch deterministic algorithms with `warn_only=True`.
6. It enables deterministic cuDNN behavior and disables cuDNN benchmarking.
7. It selects IEEE FP32 precision for general, CUDA matmul, and cuDNN FP32 operations.
8. It prohibits CPU and disk model offload.
9. Tensor-parallel execution sets `NCCL_ALGO=Ring`, `NCCL_PROTO=Simple`, and `NCCL_NVLS_ENABLE=0`.
10. The Worker executes one task at a time in its inference process. It does not combine unrelated requests through continuous or dynamic batching.

These controls remove the common sources of execution variation. They do not constitute a proof that every executed operator is deterministic.

`torch.use_deterministic_algorithms(True, warn_only=True)` allows an operation without a deterministic implementation to continue after emitting a warning. PyTorch deterministic mode also does not detect every source of nondeterminism, including conflicting writes through overlapping tensor memory. Custom model code, fused kernels, and third-party extensions retain their own determinism behavior.

The cuBLAS reproducibility guarantee applies to the same toolkit version and GPU architecture under its documented stream and workspace conditions. `CUBLAS_WORKSPACE_CONFIG=:16:8` establishes the deterministic workspace configuration for concurrent streams when cuBLAS observes the setting before creating the relevant workspace. It does not control kernels outside cuBLAS.

LLM inference executes forward generation only and does not run model backward passes. Nondeterminism that exists only in an operator's backward implementation does not affect task execution.

## Fixed-Shape Serial Execution

Load-dependent batching is a major source of variation in general-purpose LLM serving systems. A request can be grouped with different concurrent requests on different runs, changing tensor shapes, kernel selection, tile sizes, split counts, and floating-point reduction order.

Crynux does not use that serving model. One Worker inference process handles one task at a time, and one canonical task fixes the rendered input and generation configuration for every validation copy. Unrelated network traffic does not change the task's batch composition.

The default generation configuration requests one returned sequence. A task that explicitly requests another fixed number of sequences still sends the same value to every validation copy. The execution shape at a corresponding generation step is therefore derived from the task rather than from concurrent request load.

This architecture removes the common batch-variance mechanism before the deterministic framework controls are applied. It is a primary reason that the current fixed-seed execution path produces matching results in normal operation.

## Tensor-Parallel Execution

Tensor-parallel execution introduces floating-point all-reduce operations that do not exist in classic layer-sharded execution. The additional reductions change floating-point operation order, so a tensor-parallel result does not need to match a classic result.

This cross-backend difference does not imply run-to-run variation inside one tensor-parallel validation pool. The pool fixes the GPU model, GPU count, platform, executor marker, and fallback behavior. The executor fixes the world size selected for the task, uses the same task input and seed on every rank, pins NCCL to the `Ring` algorithm and `Simple` protocol, and disables NVLS. These constraints fix the collective structure and reduction order used by equivalent validation tasks.

Classic and tensor-parallel results MUST remain in separate validation pools. Nodes within one tensor-parallel pool MUST retain the same pool-defining execution constraints.

Production operation after tensor-parallel execution was enabled has not shown a tensor-parallel-specific increase in LLM validation mismatches. This operational evidence applies to the current pool separation and executor controls. It does not establish equivalence between tensor-parallel and classic output.

## What Identical Execution Means

Matching a GPU model name, operating system class, container image, PyTorch version, CUDA version, Node build, model identifier, task arguments, and seed is a strong operational match. It does not prove that two executions used identical:

- model, configuration, tokenizer, and remote-code artifact bytes;
- GPU driver and loaded native-library implementations;
- tensor shapes, layouts, layer placement, CUDA streams, and workspaces;
- fused-kernel and attention implementations;
- random-generator state immediately before every random draw;
- deterministic implementations for every executed operator.

Model IDs are not immutable artifact identities. Model download and loading use a repository ID without binding it to a repository revision or file manifest. Two nodes can therefore hold different repository snapshots under the same model ID.

The statement that two fully identical executions can randomly differ requires precision. If all input bytes, runtime state, operation order, and operator implementations are identical and deterministic, the result is identical. A repeated result can differ only when an execution input or path differs, a nondeterministic operation is allowed, or stochastic decoding consumes a different effective random state.

## Floating-Point Arithmetic

Floating-point addition and multiplication are not associative. Different reduction orders can therefore produce different rounded values.

Non-associativity alone does not create run-to-run randomness. A fixed reduction order produces the same rounded value. It causes different results only when the operation order or implementation changes, including a scheduling-dependent atomic reduction, a different kernel, a different tensor shape, a different parallel topology, or a different batch execution path.

Most model computation consists of matrix multiplication and other operations whose selected implementations are repeatable under the current controls. Residual nondeterministic operations therefore affect only a subset of executions.

## How a Numeric Difference Changes an LLM Result

### Greedy Decoding

Greedy decoding selects the token with the largest logit. The seed does not affect this selection.

Let the two largest logits at one generation step be `l1` and `l2`, with `l1 > l2`. A small numeric perturbation leaves the selected token unchanged while it is smaller than the decision margin `l1 - l2`. It changes the selected token only when it changes the ordering of the leading logits.

Most generation steps have a decision margin larger than the residual numeric variation. Their selected tokens remain unchanged even when intermediate floating-point values are not bitwise identical. A visible mismatch occurs only at a boundary-sensitive step.

An exact logit tie does not create randomness when the same argmax implementation applies the same tie-breaking rule. It creates a cross-execution difference only when the logits or tie-breaking implementation differ.

### Sampling

Sampling selects a token from a probability distribution using a pseudorandom draw. Setting the same seed reproduces the result only when both executions also produce the same probability tensor and consume random draws in the same order.

A small probability change affects the selected token only when it moves a cumulative-probability boundary across the current random draw. Most small probability changes do not cross that boundary, so both executions still select the same token.

### Autoregressive Amplification

The first different token becomes part of the next model input. From that point, the executions no longer have identical inputs, and later logits and tokens can differ substantially.

Autoregressive amplification explains why one boundary crossing can produce completely different final text. It does not cause the first different token.

## Why Honest Mismatches Are Rare

The current execution controls align the task request, seed, GPU pool, executor family, common CUDA library behavior, and model execution path. Serial per-task execution also prevents unrelated requests from changing the batch shape. These controls make the dominant forward operations repeatable.

Residual numeric differences remain invisible to validation when they do not alter a token decision. A final response differs only when an execution difference reaches a boundary-sensitive greedy or sampling step. This two-stage condition explains the observed behavior:

1. An uncontrolled execution difference must first produce a numeric change.
2. That change must then cross a token-selection boundary.

After both conditions occur, autoregressive generation amplifies the first token difference and the final response hash differs. Without both conditions, the executions produce the same serialized response and the same validation score.

Longer outputs contain more token-selection steps and therefore more opportunities to encounter a boundary-sensitive step. Lower-precision execution can increase numeric perturbations and can reduce the separation between representable values, but it does not by itself make execution nondeterministic.

## Validation Interpretation

Relay compares LLM score strings exactly. The Node computes the LLM score as the SHA-256 hash of the complete serialized result file, including assistant output and response metadata. Relay does not compare logits or floating-point tensors.

An exact score match proves that the submitted result files have the same bytes. A score mismatch proves only that the result files differ. The score alone does not identify whether the first difference came from task data, model artifacts, runtime selection, random state, a nondeterministic operator, serialization, or incorrect node behavior.

The validation outcome MUST continue to follow the majority rules in [task_validation_and_slashing.md](./task_validation_and_slashing.md). Explanations of numerical nondeterminism MUST NOT weaken exact LLM score comparison or change the configured image perceptual-hash threshold.

## Execution Mode Requirements

A new execution mode, including a quantized model path, MUST satisfy the existing result-level contract within its validation pool. It does not need to reproduce the output of another dtype, quantization setting, or executor.

One canonical task setting MUST select one execution behavior. An implementation MUST NOT silently ignore the setting or select semantically different execution modes for otherwise equivalent validation tasks.

The loaded-model cache identity MUST include every task setting that changes loaded weights or execution behavior. Different settings MUST NOT reuse an incompatible cached model.

## External References

- [PyTorch Reproducibility](https://docs.pytorch.org/docs/stable/notes/randomness.html)
- [PyTorch Deterministic Algorithms](https://docs.pytorch.org/docs/stable/generated/torch.use_deterministic_algorithms.html)
- [PyTorch Numerical Accuracy](https://docs.pytorch.org/docs/stable/notes/numerical_accuracy.html)
- [NVIDIA cuBLAS Results Reproducibility](https://docs.nvidia.com/cuda/cublas/index.html#results-reproducibility)
- [Hugging Face Generation Strategies](https://huggingface.co/docs/transformers/en/generation_strategies)
- [Understanding and Mitigating Numerical Sources of Nondeterminism in LLM Inference](https://proceedings.neurips.cc/paper_files/paper/2025/file/f80094a824ba5912d4a2de169c404a40-Paper-Conference.pdf)
