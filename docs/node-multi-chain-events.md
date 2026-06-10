# Node Multi-Chain Event Handling

Relay MUST start one blockchain processor for each configured blockchain network. Each processor MUST process only its own network and MUST advance the `blockchain_cursors.network` cursor for that network independently.

Relay node membership is address-scoped. One node address MUST be joined to at most one blockchain network where it can receive and execute tasks, and that joined network is the node row's current `network`. Relay MUST reject a join request for a node address when an existing node row for that address is not `Quit`, including when the requested join network differs from the existing node row's network.

## Node Join Network

The node's Relay network MUST be decided during node join. The join request MUST include `network`, and Relay MUST store that value in the node row before the node is marked available.

Relay MUST treat the node row's `network` as the authoritative network for that node after join. Later node APIs, admin APIs, task selection, staking reads, slashing, vesting stake refresh, delegated staking projection, and node event projection MUST use the node row's `network`. These flows MUST NOT require callers to pass an additional network parameter for a joined node address.

When a node joins Relay, Relay MUST validate the node's operator staking state against the joined network's `NodeStaking` contract before marking the node available. Relay MUST read `staked_balance + staked_credits` for the node address on that network and require it to equal the join request's staking amount.

## Join-Time Staking State

Relay MUST read the node's current delegator share and per-delegator delegated staking state from the joined network's `DelegatedStaking` contract during join. Relay MUST rebuild the delegated staking projection for exactly `(node_address, network)` from the chain response:

- Existing delegation rows for that node and network MUST be marked invalid before current chain delegations are written.
- Non-zero chain delegations MUST be upserted as valid rows with the current chain amount.
- Delegation cache state for that node and network MUST be replaced with the current chain delegations after the join transaction commits.
- The delegator share cache MUST be updated from the chain share after the join transaction commits.
- Selection staking and `NetworkNodeData.staking` MUST use operator staking plus the rebuilt delegated staking total.

If a node quits one network and later joins another network, the new join MUST set the node row's `network` to the new joined network and rebuild current staking state from the new network's contracts. Relay MUST NOT require skipped historical events from the new network to reconstruct the current delegated staking projection.

## Event Projection Guard

Blockchain processor handlers for node-address events MUST mutate Relay projection state only when the event network matches the node row's current Relay network.

This guard applies to operator staking, delegated staking, delegated unstaking, delegator share changes, and delegated staking cleanup after node slashing. Matching-network `NodeSlashed` events MUST clear delegated staking projection after chain confirmation even when the node is already locally marked `Quit`.

Events for unknown nodes or nodes whose current Relay network differs from the event network MUST be skipped and logged. Skipped events MUST NOT create event rows, update node staking, update delegation rows, update delegated staking caches, update delegator share caches, or update selection max-staking.

Delegated staking, delegator share, selection staking, event records, and network statistics MUST derive from the node's current Relay network or from the explicit event network after this guard has matched the node's joined network.
