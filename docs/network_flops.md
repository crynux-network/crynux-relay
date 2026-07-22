# Network FLOPS

## Scope

Network FLOPS is the public total computing power metric exposed by Relay for Portal netstats. It represents the sum of node GPU single-precision computing power in GFLOPS and is returned to clients as TFLOPS.

## Data Flow

Relay MUST calculate Network FLOPS in the `StartSyncNetwork` background task. The background task SHALL run on its normal sync interval and SHALL persist the latest aggregate value to the single `network_flops` row with `id = 1`.

The public API MUST NOT scan node rows or recalculate GPU power during a request. `GetNetworkTFLOPS` SHALL read the precomputed `network_flops` row, divide `GFLOPS` by `1024`, and return the resulting `tflops` value.

The calculation input MUST be the `NetworkNodeData` rows loaded by `syncNodeData`. Each row supplies `CardModel` and `VRam` for GPU matching and estimation.

## Configuration File

Relay MUST load GPU FLOPS data from the JSON file configured by `network_flops.gpu_flops_file`. The file SHALL be loaded during application startup after YAML configuration and logging are initialized and before background tasks start.

The configured JSON file MUST contain:

- `default_gflops`: a positive GFLOPS value used when no model match and no VRAM estimate are available.
- `gpus`: a non-empty list of GPU entries.

Each GPU entry MUST contain:

- `name`: a non-empty GPU model substring.
- `gflops`: a positive GFLOPS value.
- `vram`: an optional positive VRAM size in GB.

Relay MUST fail startup when the configured file cannot be read, cannot be parsed, or violates these validation rules. Updating the JSON file requires a Relay restart before the new data is used.

## GPU Matching

Relay SHALL match a node GPU by comparing the node `CardModel` with each configured GPU `name` using case-insensitive substring matching.

Multi-GPU nodes report `CardModel` as `<N>x <model>` with the summed VRAM. Relay MUST parse a leading `<N>x ` prefix before matching, match the remaining model string, and multiply the matched GFLOPS by `N`. A `CardModel` without this prefix SHALL be counted as a single GPU.

Entries with `vram` set MUST be eligible only when the entry `vram` equals the node `VRam`. Entries without `vram` SHALL be eligible for any node VRAM.

When multiple entries are eligible, Relay SHALL select the entry with the longest configured `name`. If multiple eligible entries have the same `name` length, Relay SHALL select an entry with matching `vram` before an entry without `vram`. This ordering preserves more specific names such as `rtx 4060 ti`, `rtx 4090 d`, and laptop GPU names over shorter base model names.

## VRAM Estimation

Relay SHALL calculate the total GFLOPS in two passes over the loaded node data.

In the first pass, Relay SHALL match all known GPU models. Matched rows SHALL contribute their configured GFLOPS to the final total. Matched rows with positive `VRam` SHALL also be grouped by `VRam`, and each VRAM group SHALL produce a median GFLOPS estimate from its matched samples.

In the second pass, Relay SHALL estimate each unmatched row. For an unmatched row with positive `VRam`, Relay SHALL use the median GFLOPS from the largest sampled VRAM group whose VRAM is less than or equal to the row `VRam`. If no such sampled group exists, Relay SHALL use `default_gflops`.

Relay SHALL log a warning for each unmatched `CardModel` and include the node VRAM and GFLOPS value used for the estimate.
