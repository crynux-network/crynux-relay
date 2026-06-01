# Delegated Staking Specification

This document specifies delegated staking in Crynux Relay.

## Scope

This specification covers:

- delegated staking business requirements
- relay-side delegated staking data model
- node selection impact from delegated stake
- task income distribution to delegators
- delegated staking lifecycle rules
- delegated staking statistics and APIs

## Definitions

- Operator staking: the stake owned by the node operator and stored in `nodes.stake_amount`.
- Delegated staking: the stake provided by third-party delegators through the `DelegatedStaking` contract.
- Delegator share: the integer percentage stored in `nodes.delegator_share` that defines how much of node-side task income is redistributed to delegators.
- Active delegation: a `delegations` row with `valid = true`.
- Total node staking: operator staking plus the sum of all active delegated staking for the same node and network.

## Business Requirements

Relay MUST satisfy these delegated staking requirements:

- A user MUST be able to delegate stake to a node without becoming the node operator.
- Delegation state MUST be tracked per `(delegator_address, node_address, network)`.
- Relay MUST treat the `DelegatedStaking` contract on each network as the source of truth for delegation amount and delegator share.
- Task selection MUST use total node staking, not operator staking alone.
- Task settlement MUST support splitting a node's income between the operator and its delegators.
- Relay MUST expose delegated staking data through public query APIs and statistics APIs.
- Delegated staking data MUST remain isolated by network.

## Overview

Delegated staking allows users to add stake to a node and share part of that node's task income. Relay mirrors the delegated staking state from the on-chain `DelegatedStaking` contract into its own storage, uses delegated stake together with operator stake when evaluating nodes, and redistributes part of node income to delegators when a task is settled. All delegation state, earnings, and queries are scoped by network.

## Relay State

Relay MUST store delegated staking in these places:

| Storage | Purpose |
|---------|---------|
| `nodes.delegator_share` | Current delegator income percentage of the node |
| `delegations` | Current delegation rows keyed by delegator, node, and network |
| `relay_account_events` type `UserDelegation` | Delegator income ledger entries |
| `node_earnings` | Operator-side and delegator-side earnings of a node |
| `user_staking_earnings` | Earnings of one delegation tuple `(user, node, network)` |
| `user_earnings` | Total delegated staking earnings of one user |
| `node_stakings` | Historical operator staking and delegated staking snapshots |
| `node_delegator_counts` | Historical delegator count snapshots |
| `delegated_slash_jobs` | Durable node-level delegated slash progress |
| `delegated_staking_slash_records` | Per-delegator slash outcome audit records |

Relay MUST keep delegation identity scoped by network. The same delegator may delegate to the same node address on different networks, and each network MUST use an independent delegation record.

## Selection Rules

Relay MUST define the staking input to selection as:

```
total staking = operator staking + delegated staking
```

Relay MUST use total staking in:

- node selection scoring
- node probability weight calculation
- node response payloads
- node staking statistics

Relay MUST treat a node as a delegated staking node when `delegator_share > 0`.

Quit nodes MUST NOT receive new tasks even if delegation records still exist.

## Settlement Rules

Delegated staking distribution happens inside the node-side share after the DAO cut.

For each settled task payment unit `payment`, Relay MUST compute:

```
dao_income = floor(payment * dao_percent / 100)
node_income_before_delegation = payment - dao_income
delegator_pool = floor(node_income_before_delegation * delegator_share / 100)
operator_income = node_income_before_delegation - delegator_pool
```

Relay MUST apply delegated staking distribution only when both conditions are true:

- `delegator_share > 0`
- the node has at least one active delegation on the task network

If either condition is false, Relay MUST:

- keep `operator_income = node_income_before_delegation`
- keep `delegator_pool = 0`
- create no `UserDelegation` relay account events

When delegated staking distribution is active, Relay MUST:

1. Sum all active delegation amounts for the node on the task network.
2. Split `delegator_pool` proportionally by stake.
3. Assign any integer remainder to one delegator so the total dispatched amount equals `delegator_pool`.
4. Create one `TaskIncome` event for the operator.
5. Create one `DaoTaskShare` event for the DAO address.
6. Create one `UserDelegation` event for each participating delegator.

## Lifecycle Rules

Relay MUST keep delegated staking state aligned with the `DelegatedStaking` contract.

When a delegation is created or increased, Relay MUST update the corresponding `(delegator, node, network)` record and mark it active.

When a delegation is unstaked, Relay MUST mark the corresponding delegation record inactive.

When `delegator_share` changes, Relay MUST update `nodes.delegator_share`.

If `delegator_share` becomes `0`, Relay MUST:

- keep existing delegation records active until each delegator exits or is slashed on chain
- stop treating the node as a delegated staking node
- keep active delegated staking in total node staking for task selection
- stop distributing task income to delegators of that node

When operator slash is confirmed through `NodeStaking.NodeSlashed`, Relay MUST create or resume a delegated slash job for the node and network. The job MUST queue bounded `DelegatedStaking::slashNodeDelegations` transactions and MUST NOT mark a delegation inactive until the matching `DelegatedStaking.DelegatorSlashed` event is confirmed.

For each confirmed `DelegatorSlashed` event, Relay MUST atomically:

- mark the matching delegation inactive
- write one `delegated_staking_slash_records` record with node, delegator, network, amount, slash transaction hash, block number, and log index
- emit the Relay `DelegatedStakingSlashed` event
- remove the delegation from the in-memory delegation cache

Relay MUST use `(network, slash_tx_hash, log_index)` as the chain-event idempotency key for delegated slash audit records. A delegated slash job MUST be completed only after the contract reports zero remaining delegated staking records for the node.

When a node quits, Relay MUST remove the node from task selection, but Relay MUST NOT invalidate delegations only because of the local quit action. Delegation validity SHALL continue to follow the on-chain delegated staking state.

## Earnings and Statistics

Relay MUST persist delegated staking earnings in these forms:

- `node_earnings.operator_earning`: the operator-retained portion after delegated staking distribution
- `node_earnings.delegator_earning`: the total amount redirected to delegators
- `user_staking_earnings`: per-delegation earnings for `(user, node, network)`
- `user_earnings`: per-user delegated staking earnings across all nodes

Relay MUST maintain both:

- daily rows
- total rows

Relay MUST also maintain historical snapshots for:

- operator staking
- delegated staking
- delegator count

## API Requirements

Relay MUST expose delegated staking through these public APIs:

| API | Contract |
|-----|----------|
| `GET /v1/delegator/:user_address` | Delegator summary across all configured networks |
| `GET /v1/delegator/:user_address/delegation` | One delegation identified by user, node, and network |
| `GET /v1/delegator/:user_address/delegations` | Delegation list with pagination and optional network filter |
| `GET /v2/delegated_staking/nodes` | Delegated staking node list |
| `GET /v2/delegated_staking/nodes/:address` | Delegated staking node details |
| `GET /v2/delegated_staking/nodes/:address/delegations` | Delegation list for one delegated staking node and network |
| `GET /v2/admin/delegated_slash/audits` | Authenticated paginated delegated slash audit lookup |
| `GET /v1/client/:address/income/stats` | Split of operator income and delegated staking income for one client |

Relay MUST expose delegated staking statistics through these APIs:

- `GET /v1/stats/line_chart/node/:address/earnings`
- `GET /v1/stats/line_chart/node/:address/staking`
- `GET /v1/stats/line_chart/node/:address/scores`
- `GET /v1/stats/line_chart/node/:address/delegator_num`
- `GET /v1/stats/line_chart/delegator/:address/earnings`
- `GET /v1/stats/line_chart/delegation/:user_address/:node_address/earnings`

The delegated-staking-only node APIs and statistics APIs MUST return `404` when `delegator_share = 0`.

## Configuration

Each blockchain network configuration MUST provide:

| Key | Description |
|-----|-------------|
| `blockchains.<network>.contracts.delegated_staking` | On-chain `DelegatedStaking` contract address |
| `blockchains.<network>.delegated_staking_slash_batch_size` | Maximum delegator addresses in one delegated slash transaction |
| `blockchains.<network>.delegated_staking_read_page_size` | Page size for contract delegation reads |

Delegated staking state, earnings, and queries MUST always use the task or event network as the isolation key.
