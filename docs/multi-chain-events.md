# Multi-Chain Event Handling

Relay MUST start one block listener for each configured blockchain network. Each listener MUST process only its own network and MUST advance the `block_listeners.network` cursor for that network independently.

Relay node join is address-scoped. One node address MUST be joined to at most one blockchain network where it can receive and execute tasks, and that joined network is the node row's current `network`. Relay MUST reject a join request for a node address when an existing node row for that address is not `Quit`, including when the requested join network differs from the existing node row's network.

## Node Join State

When a node joins Relay, the requested network becomes the node's Relay network. Relay MUST validate the node's operator staking state against that network's `NodeStaking` contract before marking the node available.

Relay MUST read the node's current delegator share and per-delegator delegated staking state from that network's `DelegatedStaking` contract during join. Relay MUST rebuild the delegated staking projection for exactly `(node_address, network)` from the chain response:

- Existing delegation rows for that node and network MUST be marked invalid before current chain delegations are written.
- Non-zero chain delegations MUST be upserted as valid rows with the current chain amount.
- Delegation cache state for that node and network MUST be replaced with the current chain delegations after the join transaction commits.
- The delegator share cache MUST be updated from the chain share after the join transaction commits.
- Selection staking and `NetworkNodeData.staking` MUST use operator staking plus the rebuilt delegated staking total.

If a node quits one network and later joins another network, the new join MUST rebuild current staking state from the new network's contracts. Relay MUST NOT require skipped historical events from the new network to reconstruct the current delegated staking projection.

## Event Projection Guard

Block listener handlers for node-address events MUST mutate Relay projection state only when the event network matches the node row's current Relay network.

This guard applies to operator staking, delegated staking, delegated unstaking, delegator share changes, and delegated staking cleanup after node slashing. Matching-network `NodeSlashed` events MUST clear delegated staking projection after chain confirmation even when the node is already locally marked `Quit`.

Events for unknown nodes or nodes whose current Relay network differs from the event network MUST be skipped and logged. Skipped events MUST NOT create event rows, update node staking, update delegation rows, update delegated staking caches, update delegator share caches, or update selection max-staking.

Delegated staking, delegator share, selection staking, event records, and network statistics MUST derive from the node's current Relay network or from the explicit event network after this guard has matched the node's joined network.
