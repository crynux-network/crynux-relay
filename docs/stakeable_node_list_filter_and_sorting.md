# Stakeable Node List Filter And Sorting Specification

This document specifies the design goal and implementation model for filtering and sorting the Portal stakeable node list through Relay.

## Scope

This specification covers:

- stakeable node list filter and sort goals
- Relay-side list snapshot design
- snapshot refresh rules
- public API behavior
- Portal interaction model

## Design Goals

The stakeable node list MUST support filtering by node status, GPU VRAM, GPU name, and node version. The list MUST support one selected descending sort metric at a time. Relay MUST rank running nodes before stopped nodes for every sort metric.

Relay MUST keep list pagination efficient. A list request MUST NOT recalculate every candidate node, join large runtime tables, aggregate vesting records, or cast string amount fields across the full stakeable-node set. Expensive per-node response enrichment MUST be limited to nodes on the requested page.

The response payload for each node MUST remain compatible with the existing delegated staking node response. Snapshot fields are used for list selection, filtering, and sorting only.

## Snapshot Model

Relay maintains `delegated_staking_node_list_snapshots` as the indexed list source for stakeable nodes. One row represents one node with `delegator_share > 0`.

Each snapshot row stores:

- identity and filter fields: node address, blockchain network, status group, GPU name, GPU VRAM, and normalized version
- ranking fields: status rank, operator four-week emission, delegator four-week emission, estimated upcoming operator emission, estimated upcoming delegator emission, operator staking, delegator staking, total staking, delegator count, probability weight, QoS, and GPU VRAM
- maintenance timestamps

The status group MUST be `running` for all non-quit node statuses and `stopped` for quit nodes. `running` MUST have a lower status rank than `stopped`.

Amount sort fields MUST be stored as database-native decimal values so list queries can order by indexed numeric columns without runtime string casts.

## Snapshot Refresh

Relay MUST rebuild the full snapshot list on startup after delegation, vesting, and selecting-probability caches are initialized. Relay MUST refresh the full snapshot list once per hour as the correctness backstop.

Relay MUST refresh a single node snapshot after committed node lifecycle changes and delegated staking mutations that directly affect list visibility, filters, or sort fields:

- node join
- node quit
- delegation stake update
- delegation unstake
- delegation slash
- node delegator share change

When a node no longer has `delegator_share > 0`, Relay MUST remove the node from the snapshot list.

The hourly full refresh covers lower-frequency values such as vesting changes, QoS changes, health changes, and task-status transitions that are not refreshed per event.

## List Query Behavior

`GET /v2/delegated_staking/nodes` MUST query snapshot rows first. Relay MUST apply the selected filters to snapshot columns, count matching rows, order by `status_rank ASC`, the selected sort metric descending, and `node_address ASC`, then apply pagination.

After pagination, Relay MUST load only the corresponding `nodes` rows and build the existing node response for those page addresses.

Supported filters are:

- `status`
- `gpu_vram`
- `gpu_name`
- `version`

Supported sort keys are:

- `operator_emission_4w`
- `delegator_emission_4w`
- `estimated_upcoming_operator_emission`
- `estimated_upcoming_delegator_emission`
- `operator_staking`
- `delegator_staking`
- `total_staking`
- `delegators_num`
- `prob_weight`
- `qos`
- `gpu_vram`

Relay MUST reject invalid filter values and invalid sort keys with validation errors.

## Filter Options

`GET /v2/delegated_staking/nodes/filter_options` MUST return available filter values from the snapshot list. The endpoint MUST return only values that exist on currently stakeable nodes.

The returned option groups are:

- `statuses`
- `gpu_vrams`
- `gpu_names`
- `versions`

## Portal Behavior

Portal MUST fetch filter options from Relay and render the stakeable node list with a filter card and a sort selector. Changing any filter or sort key MUST reset pagination to page 1 and reload the list through Relay.

Portal MUST send selected filters and the selected sort key as query parameters to Relay. Portal MUST display existing node card data from the delegated staking node response and MUST NOT compute list ordering on the client.

