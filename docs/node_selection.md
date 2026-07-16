# Node Selection

This document describes how node selection works in Crynux Relay.

## Overview

Relay selects tasks from the queue according to [task-pricing.md](./task-pricing.md). This document starts after queue selection and specifies how Relay selects an execution node for the selected task.

Node selection runs inside the matching scheduler specified in [task_matching.md](./task_matching.md). Candidate node data and weight inputs are served from the node scheduling index defined there; the filtering, weighting, and sampling semantics in this document are unchanged by that data source.

Node selection is a pipeline:

1. **Qualification Filters** enforce availability, hardware, version, task-specific, and node-name policy requirements.
2. **Base-Model Gate** retains only qualified nodes that have the task's base model on disk.
3. **Base Weight** computes a base weight per ready candidate from staking and QoS.
4. **In-Memory Locality Weight** boosts ready-candidate weights when the task's base model is already in use.
5. **Weighted Sampling** selects the final node using the effective weights.

## Qualification Filters

Relay MUST first apply these hard filters to form the qualified node set:

- **Availability**. Only nodes currently in the `Available` status are eligible for selection.
- **Hardware compatibility**. If the task specifies a required GPU, the node must match both that GPU model and the required VRAM exactly. Otherwise, the node must satisfy the task's minimum VRAM requirement.
- **Version compatibility**. For task and node version compatibility rules used by this selection flow, see [task_version.md](./task_version.md).
- **Task-specific exclusions**. `LLM` tasks exclude nodes on `Darwin`.
- **Node name policy**. Relay MAY enforce node-name restrictions using the exact tuple (`GPUName`, `GPUVram`, `NodeVersion`), where `NodeVersion` is the node-reported `major.minor.patch`. When `task.node_name_whitelist_enabled` is true, only tuples in the node-name whitelist are eligible. Eligible tuples MUST also satisfy `task.minimum_node_name_number` using active-count data. Active counts include statuses `Available`, `Busy`, `PendingPause`, and `PendingQuit`, and exclude `Paused` and `Quit`. All GPU names MUST be stored and compared in normalized form: leading and trailing whitespace is removed, and every internal run of whitespace is collapsed into a single space. Relay MUST apply this normalization at every input boundary that accepts a GPU name: node join (`gpu_name`), task creation (`required_gpu`), and the admin node-name whitelist APIs.

## Base-Model Gate

Every inference task requires exactly one base model: the single lowercase `base:` entry of `InferenceTask.ModelIDs`. All other entries are auxiliary `lora:` and `controlnet:` models and MUST be ignored by Relay matching.

A qualified node is base-ready only when its on-disk model ID set contains the task's base model ID. When no qualified node is base-ready, Relay MUST leave the task queued and MUST NOT fall back to hardware-only or other qualification-only selection.

A task with no base model ID passes the base-model gate without an on-disk model requirement.

## Base Weight

Base weight is computed from a staking score and a QoS score, then combined using a harmonic mean.

### Staking Score

```
StakingScore = sqrt(staking / maxStaking)
```

Where:
- `staking`: the node score stake. Score stake is the sum of operator stake, non-slashed delegated stake on the node current blockchain network, and active unslashed locked vesting for the node address across all vesting types.
- `maxStaking`: the maximum score stake among all non-quit nodes.

Vesting contributes only to staking score and selection probability. Displayed staking token amounts MUST use operator stake plus delegated stake and MUST NOT include vesting amounts.

### QoS Score

QoS is computed as described in [qos.md](./qos.md).

### Harmonic Mean

```
BaseWeight = StakingScore * QoS / (StakingScore + QoS)
```

If either `StakingScore` or `QoS` is `0`, then `BaseWeight` is `0`.

## In-Memory Locality Weight

All candidates that reach weighting already have the task's base model on disk. On-disk locality therefore acts only as a hard gate and MUST NOT add a weight component. Relay MUST apply only the `0.3` in-memory locality component to ready candidates:

```
localityWeight = 1 + 0.3 (when the task's base model is in the node's in-use model set)
localityWeight = 1   (otherwise)
```

When the task has no base model ID, Relay MUST use a locality weight of `1`.

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
| `service/task_matching.go` | Qualification filters, base-model gate, in-memory locality weight, and weighted sampling |
| `service/qos.go` | QoS score calculation (`CalculateQosScore`) |
