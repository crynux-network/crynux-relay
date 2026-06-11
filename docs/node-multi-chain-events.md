# Node Multi-Chain Event Handling

Relay MUST start one blockchain processor for each configured blockchain network. Each processor MUST process only its own network and MUST advance the `blockchain_cursors.network` cursor for that network independently.

Relay node membership is address-scoped. One node address MUST be joined to at most one blockchain network where it can receive and execute tasks, and that joined network is the node row's current `network`. Relay MUST reject a join request for a node address when an existing node row for that address is not `Quit`, including when the requested join network differs from the existing node row's network.

Relay MUST also reject a join request for a node address while any delegated slash job for that node address is not completed on any blockchain network.

## Node Join Network

The node's current blockchain network MUST be decided during node join. The join request MUST include `network`, and Relay MUST store that value in the node row before the node is marked available.

Relay MUST treat the node row's `network` as the authoritative network for that node after join. Later node APIs, admin APIs, task selection, staking reads, slashing, vesting stake refresh, delegated staking projection, and node event projection MUST use the node row's `network`. These flows MUST NOT require callers to pass an additional network parameter for a joined node address.

When a node joins Relay, Relay MUST validate the node's operator staking state against the joined network's `NodeStaking` contract before marking the node available. Relay MUST read `staked_balance + staked_credits` for the node address on that network and require it to equal the join request's staking amount.

## Join-Time Staking State

Relay MUST read the node's current delegator share and per-delegator delegated staking state from the joined network's `DelegatedStaking` contract during join. Relay MUST rebuild the delegated staking projection for exactly `(node_address, network)` from the chain response:

- Existing non-slashed delegation rows for that node and network MUST be deleted before current chain delegations are written.
- Existing slashed delegation rows for that node and network MUST remain user-visible read-only records.
- Non-zero chain delegations MUST be upserted as `slashed = false` rows with the current chain amount.
- Delegation cache state for that node and network MUST be replaced with the current chain delegations after the join transaction commits.
- The delegator share cache MUST be updated from the chain share after the join transaction commits.
- Selection staking and `NetworkNodeData.staking` MUST use operator staking plus the rebuilt delegated staking total.

If a node quits one network and later joins another network, the new join MUST set the node row's `network` to the new joined network and rebuild current staking state from the new network's contracts. Relay MUST NOT require skipped historical events from the new network to reconstruct the current delegated staking projection.

## Event Projection Guards

Blockchain processor handlers for node-scoped events MUST mutate Relay node projection state only when the event network matches the node row's current blockchain network.

This guard applies to operator staking, delegator share changes, selection staking, node-level slash orchestration, and node event projection. Matching-network `NodeSlashed` events MUST create or resume delegated slash jobs even when the node is already locally marked `Quit`.

Node-scoped events for unknown nodes or nodes whose current blockchain network differs from the event network MUST be skipped and logged. Skipped node-scoped events MUST NOT create event rows, update node staking, update delegated staking caches, update delegator share caches, or update selection max-staking.

Blockchain processor handlers for user-owned delegated staking state MUST process confirmed `DelegatorStaked` and `DelegatorUnstaked` events on the event blockchain network when the node address is known, even when the node row's current blockchain network differs from the event blockchain network or the node is locally marked `Quit`. These handlers MUST update the user-visible `(delegator_address, node_address, event network)` delegation row and event-network cache. They MUST update selection max-staking only when the event blockchain network equals the node row's current blockchain network.

Delegator share, selection staking, node response staking, and network statistics MUST derive from the node's current blockchain network. User-visible delegation records MUST derive from the delegation row's own blockchain network.
