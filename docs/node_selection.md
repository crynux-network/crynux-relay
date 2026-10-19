# Node Selection

This document describes how node selection works in Crynux Relay.

## Overview

Relay selects tasks from the queue according to [task-pricing.md](./task-pricing.md). This document starts after queue selection and specifies how Relay selects an execution node for the selected task.

Node selection runs inside the matching scheduler specified in [task_matching.md](./task_matching.md). Candidate node data and weight inputs are served from the node scheduling index defined there; the filtering, weighting, and sampling semantics in this document are unchanged by that data source.

Node selection is a pipeline:

1. **Qualification Filters** enforce availability, hardware, version, task-specific, and node-name policy requirements.
2. **Model File Gate** retains only qualified nodes whose on-disk model files cover the task's required model files.
3. **Base Weight** computes a base weight per ready candidate from staking and QoS.
4. **In-Memory Locality Weight** boosts ready-candidate weights when the task's requested model is already in memory.
5. **Weighted Sampling** selects the final node using the effective weights.

## Qualification Filters

Relay MUST first apply these hard filters to form the qualified node set:

- **Availability**. Only nodes currently in the `Available` status are eligible for selection.
- **Short-term health**. A node with persistent `health_excluded = true` and effective health below `qos.health_exclude_exit_threshold` MUST be removed from the candidate set. Relay MUST apply this as a hard filter and MUST NOT depend on a zero selection weight. A node whose effective health is equal to or above the exit threshold MAY pass this filter while its persistent flag remains set; the task-start transaction MUST recheck health and clear the flag atomically.
- **Hardware compatibility**. If the task specifies a required GPU, the node must match both that GPU model and the required VRAM exactly. Otherwise, the node must satisfy the task's minimum VRAM requirement.
- **Version compatibility**. For task and node version compatibility rules used by this selection flow, see [task_version.md](./task_version.md).
- **Task-specific exclusions**. `LLM` tasks exclude nodes on `Darwin` when the task does not pin `RequiredGPU`. When `InferenceTask.RequireLinuxCuda` is true, Relay MUST retain only nodes whose GPU name platform suffix is `docker` or `Linux`, and MUST apply this filter whether or not the task pins `RequiredGPU`. Platform suffixes other than `docker` and `Linux`, including `Windows`, `Darwin`, and a missing `+` suffix, MUST be excluded for `RequireLinuxCuda` tasks. See [quantization_platform_support.md](./quantization_platform_support.md).
- **Node name policy**. Relay MAY enforce node-name restrictions using the exact tuple (`GPUName`, `GPUVram`, `NodeVersion`), where `NodeVersion` is the node-reported `major.minor.patch`. When `task.node_name_whitelist_enabled` is true, only tuples in the node-name whitelist are eligible. Eligible tuples MUST also satisfy `task.minimum_node_name_number` using active-count data. Active counts include statuses `Available`, `Busy`, `PendingPause`, and `PendingQuit`, and exclude `Paused` and `Quit`. All GPU names MUST be stored and compared in normalized form: leading and trailing whitespace is removed, and every internal run of whitespace is collapsed into a single space. Relay MUST apply this normalization at every input boundary that accepts a GPU name: node join (`gpu_name`), joined-node capability synchronization (`gpu_name`), task creation (`required_gpu`), and the admin node-name whitelist APIs. When an active joined node changes any member of the tuple through capability synchronization, Relay MUST atomically decrement the persisted count for the old tuple and increment the count for the new tuple, then apply the same transfer to the in-memory count cache after commit. Capability synchronization for `Paused` nodes MUST NOT change active counts.

## Model File Gate

Every task carries required model files derived from its TaskArgs: `RequiredModelFileHashes` holds the exact hashes of the entries with a revision, and `RequiredModelFileSourceHashes` holds the source hashes of the entries without a revision. `InferenceTask.ModelIDs` MUST be ignored by Relay matching.

A qualified node is model-ready only when its on-disk model file inventory contains every exact hash and at least one file of every source hash. When no qualified node is model-ready, Relay MUST leave the task queued and MUST NOT fall back to hardware-only or other qualification-only selection.

A task with no required model files passes the model file gate without an on-disk requirement.

## Base Weight

Base weight is computed from a staking score and a QoS score, then combined using a harmonic mean.

### Staking Score

```
StakingScore = sqrt(staking / maxStaking)
```

Where:
- `staking`: the node score stake. Score stake is the sum of operator stake, non-slashed delegated stake on the node current blockchain network, active unslashed locked vesting for the node address across all vesting types multiplied by `staking_score.locked_emission_coefficient`, and the node relay account balance.
- `maxStaking`: the maximum score stake among all non-quit nodes.

`staking_score.locked_emission_coefficient` is a required config value in `[0, 1]`. It scales only the locked vesting component; the scaled amount is rounded down to an integer wei value. The relay account balance component uses the node address balance from the in-memory relay account cache and is counted in full.

Relay APIs that expose selection probability fields MAY expose this score stake as `staking` for effective-stake displays. Portal netstats MUST label that value as `Effective Stake` and present it as `Operator Stake + Delegated Stake + Locked Emission * 0.4 + Relay Account Balance`. Displayed raw staking token amounts MUST use operator stake plus delegated stake and MUST NOT include vesting amounts or relay account balance.

### QoS Score

QoS is computed as described in [qos.md](./qos.md).

### Harmonic Mean

```
BaseWeight = StakingScore * QoS / (StakingScore + QoS)
```

If either `StakingScore` or `QoS` is `0`, then `BaseWeight` is `0`.

## In-Memory Locality Weight

All candidates that reach weighting already hold the task's required model files on disk. On-disk locality therefore acts only as a hard gate and MUST NOT add a weight component. Relay MUST apply only the `0.3` in-memory locality component to ready candidates:

```
localityWeight = 1 + 0.3 (when the node's in-memory model has the task's RequestedModelCacheHash and its model files cover the task's required model files)
localityWeight = 1   (otherwise)
```

The in-memory model of a node MUST come from the node's authoritative in-memory model report. The cache hash comparison MUST be exact; the node cache hash does not include revisions, so the model file coverage check MUST additionally require every exact required hash and one file of every source required hash among the in-memory model files.

## Final Effective Weight

```
EffectiveWeight = BaseWeight * localityWeight
```

## Weighted Sampling

Nodes are sampled by weighted random selection using `EffectiveWeight`.

## Relevant Source Files

| File | Description |
|------|-------------|
| `service/selecting_prob.go` | Staking score and base weight calculation |
| `service/task_matching.go` | Qualification filters, model file gate, in-memory locality weight, and weighted sampling |
| `service/qos.go` | QoS score calculation (`CalculateQosScore`) |
