# Node Selection

This document describes how node selection works in Crynux Relay.

## Overview

Relay selects tasks from the queue according to [task-pricing.md](./task-pricing.md). This document starts after queue selection and specifies how Relay selects an execution node for the selected task.

Node selection runs inside the matching scheduler specified in [task_matching.md](./task_matching.md). Candidate node data and weight inputs are served from the node scheduling index defined there; the filtering, weighting, and sampling semantics in this document are unchanged by that data source.

Node selection is a pipeline:

1. **Hard Filters** build the candidate set.
2. **Base Weight** computes a base weight per candidate from staking and QoS.
3. **Model Locality Boost** boosts weights based on on-disk and in-memory locality.
4. **Weighted Sampling** selects the final node using the effective weights.

## Hard Filters

The relay first applies hard filters to form the candidate set.

- **Availability**. Only nodes currently in the `Available` status are eligible for selection.
- **Hardware compatibility**. If the task specifies a required GPU, the node must match both that GPU model and the required VRAM exactly. Otherwise, the node must satisfy the task's minimum VRAM requirement.
- **Version compatibility**. For task and node version compatibility rules used by this selection flow, see [task_version.md](./task_version.md).
- **Task-specific exclusions**. `LLM` tasks exclude nodes on `Darwin`.
- **Node name policy**. The relay MAY enforce node-name restrictions using the exact tuple (`GPUName`, `GPUVram`, `NodeVersion`), where `NodeVersion` is the node-reported `major.minor.patch`. When `task.node_name_whitelist_enabled` is true, only tuples in the node-name whitelist are eligible. Eligible tuples MUST also satisfy `task.minimum_node_name_number` using active-count data. Active counts include statuses `Available`, `Busy`, `PendingPause`, and `PendingQuit`, and exclude `Paused` and `Quit`.
- **Local model filter (on-disk)**. Only select nodes that have at least one required model available locally on disk. If no node has any required model available locally, keep all candidates.

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

## Model Locality Boost

The goal of the model locality boost is to prefer nodes that can start the task sooner and with less network and IO overhead. There are two locality layers. On disk locality means the required model is already downloaded and present on the node. In memory locality means the required model is already loaded and in use in GPU memory. On disk locality is weighted higher because avoiding model downloads typically saves far more time and bandwidth than avoiding an extra load step. In memory locality is still valuable, so it adds an extra bonus on top of on disk locality.

```
boost = 1 + diskWeight * (localCnt / total) + memWeight * (inUseCnt / total)
```

In this formula,

 * `diskWeight` is `0.7` and `memWeight` is `0.3`.
 * `localCnt` is the number of required models that are available locally on disk on the node.
 * `inUseCnt` is the number of required models that are already in GPU memory and in use.
 * `total` is the number of required models for the task.

For a task that requires 3 models, the following cases illustrate how on disk and in memory differ.

| In-memory models | On-disk models | boost |
|--:|--:|--:|
| 0 | 1 | 1.233333 |
| 1 | 1 | 1.333333 |
| 0 | 2 | 1.466667 |
| 1 | 2 | 1.566667 |
| 2 | 2 | 1.666667 |
| 0 | 3 | 1.700000 |
| 1 | 3 | 1.800000 |
| 2 | 3 | 1.900000 |
| 3 | 3 | 2.000000 |

For a task that requires 2 models, the per model contribution is larger because `total` is smaller.

| In-memory models | On-disk models | boost |
|--:|--:|--:|
| 0 | 1 | 1.350000 |
| 1 | 1 | 1.500000 |
| 0 | 2 | 1.700000 |
| 1 | 2 | 1.850000 |
| 2 | 2 | 2.000000 |

For a task that requires 1 model, the locality impact is the strongest.

| In-memory models | On-disk models | boost |
|--:|--:|--:|
| 0 | 1 | 1.700000 |
| 1 | 1 | 2.000000 |

## Final Effective Weight

```
EffectiveWeight = BaseWeight * boost
```

## Weighted Sampling

Nodes are sampled by weighted random selection using `EffectiveWeight`.

## Relevant Source Files

| File | Description |
|------|-------------|
| `service/selecting_prob.go` | Staking score and base weight calculation |
| `service/select_nodes.go` | Candidate filtering, model locality boost, and weighted sampling |
| `service/qos.go` | QoS score calculation (`CalculateQosScore`) |
