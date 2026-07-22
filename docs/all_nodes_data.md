# All Nodes Data

This document specifies the Relay `network_node_data` projection used for public all-node display data.

## Projection Ownership

`network_node_data` is the historical joined-node public snapshot. A row MUST exist for each node address that has joined Relay. The table MUST preserve rows after node quit, kickout, slash, and later joins so Portal and admin exports can display historical joined-node records.

`network_node_data` MUST NOT use soft delete. Relay queries for all-node public data MUST see every historical row in this projection.

The `nodes` table remains the source of truth for node lifecycle, scheduling state, current blockchain network, active membership, busy state, and quit state. Scheduling, staking validation, slash guards, task selection, and active-node counts MUST use `nodes` and chain-derived staking state rather than `network_node_data` row visibility.

## Stored Fields

Each `network_node_data` row SHALL keep the node address and public display fields:

- `address`: node address.
- `network`: the last blockchain network recorded when the node joined.
- `card_model`: GPU model name last recorded for public display.
- `v_ram`: GPU VRAM last recorded for public display.
- `qo_s`: current QoS score for active nodes or the last known QoS score for quit nodes.
- `staking`: current selection staking for active nodes or the last known staking value for quit nodes.
- `health_base`: current health base for active nodes or the last known health base for quit nodes.
- `health_updated_at`: current health update timestamp for active nodes or the last known health update timestamp for quit nodes.

## Join Behavior

When a node joins Relay, Relay MUST upsert the `network_node_data` row by `address`. The upsert MUST write the node's blockchain network, public display fields from the join request, and current Relay-derived state.

If the address has joined before, the new join MUST update the existing row instead of creating a duplicate row. The row's historical identity MUST remain associated with the same node address, and `network` MUST reflect the node's most recent join network.

## Quit Retention

When a node quits, is kicked out, or is slashed, Relay MUST keep the `network_node_data` row. Quit nodes retain their last known public snapshot fields. Relay MUST NOT remove, soft-delete, or hide the row as part of node lifecycle transitions.

## SyncNetwork Refresh

`SyncNetwork` is a netstats projection worker. It MUST refresh `network_node_numbers`, `network_task_numbers`, active-node fields in `network_node_data`, and `network_flops`.

For active nodes, `SyncNetwork` MUST refresh QoS, staking, and health fields from Relay state. For quit nodes, `SyncNetwork` MUST preserve the last known public snapshot fields except for fields explicitly refreshed from non-lifecycle public state.

`SyncNetwork` MUST NOT drive node lifecycle, scheduling, blockchain event processing, or node membership decisions.
