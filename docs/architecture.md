# Relay Architecture

This document specifies the high-level architecture and terminology for Crynux Relay.

## System Boundary

Crynux Relay is a single off-chain service that coordinates nodes, tasks, account state, statistics, and blockchain projections across multiple configured blockchain networks. Tasks are scheduled and dispatched by Relay. Tasks are not partitioned by blockchain network.

Relay MUST NOT be described as multiple Relay networks. Relay has one service-level state model. Blockchain-specific state is partitioned by blockchain network keys.

Each configured blockchain network has its own contracts, blockchain processor, cursor, events, deposits, withdrawals, staking state, delegated staking state, and chain-specific projection data. Relay MUST keep blockchain-derived state isolated by network unless a document explicitly defines an aggregate across configured blockchain networks.

## Terminology

Documentation and implementation MUST use these terms consistently:

| Term | Definition |
|------|------------|
| Relay | The single off-chain Relay service and database-backed application. |
| Blockchain network | A configured chain identified by a network key in Relay configuration. |
| Node current blockchain network | The blockchain network stored in the node row's `network` field after join. |

The terms `Relay network`, `current Relay network`, `node Relay network`, and `task network` MUST NOT be used to mean a blockchain network. Documents MUST use `blockchain network` or `node current blockchain network`.

Tasks are scheduled and dispatched by Relay and are not partitioned by blockchain network.

## Node Membership

Node membership is address-scoped within Relay. One node address MUST have at most one current blockchain network for staking, slashing, and blockchain-derived node state. Relay task scheduling is not partitioned by blockchain network.

The node row's `network` field is the node's current blockchain network. Node APIs, admin APIs, task selection, staking reads, slashing, vesting stake refresh, delegated staking projection, and node event projection MUST use that current blockchain network unless a document defines a user-owned historical or inactive state on another blockchain network.

When a node quits one blockchain network and later joins another blockchain network, Relay MUST treat the new join network as the node current blockchain network. Historical user-owned state on the previous blockchain network MUST remain isolated under the previous blockchain network key.

## Blockchain Processors

Relay MUST run one blockchain processor for each configured blockchain network. Each processor MUST process only its own blockchain network and MUST advance the cursor for that blockchain network independently.

An event processed by one blockchain processor MUST be attributed to that processor's blockchain network. Event handling MUST NOT infer an event's blockchain network from the node's current blockchain network.

## State Isolation

Blockchain-derived records MUST include or derive a blockchain network identity when the same address or entity can exist on more than one blockchain network. This applies to staking, delegated staking, deposits, withdrawals, account ledger projections, event cursors, task settlement, statistics, and blockchain transaction records.

Address equality across blockchain networks MUST NOT imply state equality. The same node address, delegator address, client address, transaction hash, or contract address on different blockchain networks MUST be treated as distinct blockchain-scoped state unless a document explicitly defines a cross-network aggregate.

## User-Facing State

Portal and public APIs MUST distinguish the Relay service from blockchain networks. User-facing text MUST describe staking, delegation, deposits, withdrawals, and transactions as belonging to blockchain networks.

User-owned blockchain state can remain visible even when it does not belong to a node's current blockchain network. Such state MUST be labeled according to the owning blockchain network and MUST NOT be described as belonging to a separate Relay network.

Node-scoped APIs MUST use the node current blockchain network by default and MUST NOT require a `network` parameter for ordinary node operations. APIs MAY include an explicit `network` parameter only when the operation or query targets blockchain state that can exist outside the node current blockchain network, such as a user's inactive delegation on a previous blockchain network.
