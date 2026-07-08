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
- Current delegation: a `delegations` row with `slashed = false`. The row represents stake currently locked in the `DelegatedStaking` contract on the row's blockchain network.
- Slashed delegation: a `delegations` row with `slashed = true`. The row is a read-only user-visible record for confiscated delegated stake and MUST NOT participate in staking logic.
- Active delegation: a current delegation whose blockchain network matches the node's current blockchain network.
- Inactive delegation: a current delegation whose blockchain network differs from the node's current blockchain network.
- Total node staking: operator staking plus the sum of current delegated staking for the same node and the node's current blockchain network.

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
| `delegations` | User-visible delegation records keyed by delegator, node, and blockchain network |
| `relay_account_events` type `UserDelegation` | Delegator income ledger entries |
| `node_earnings` | Operator-side and delegator-side earnings of a node |
| `user_staking_earnings` | Earnings of one delegation tuple `(user, node, network)` |
| `user_earnings` | Total delegated staking earnings of one user |
| `vesting_delegation_emission_details` | Issued delegation emission mapped back to `(user, node, network)` details |
| `node_stakings` | Historical operator staking and delegated staking snapshots |
| `node_delegator_counts` | Historical delegator count snapshots |
| `delegated_slash_jobs` | Durable node-level delegated slash progress |
| `delegated_staking_slash_records` | Per-delegator slash outcome audit records |

Relay MUST keep delegation identity scoped by network. The same delegator may delegate to the same node address on different networks, and each network MUST use an independent delegation record.

In `delegations`, `slashed = false` rows MUST represent current locked delegated stake. `slashed = true` rows MUST represent read-only slashed state. A missing row MUST represent no current or user-visible delegation record, including after normal unstake.

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
- the node has at least one non-slashed delegation row on the selected node's current blockchain network

If either condition is false, Relay MUST:

- keep `operator_income = node_income_before_delegation`
- keep `delegator_pool = 0`
- create no `UserDelegation` relay account events

When delegated staking distribution is active, Relay MUST:

1. Sum all non-slashed delegation amounts for the node on the selected node's current blockchain network.
2. Split `delegator_pool` proportionally by stake.
3. Assign any integer remainder to one delegator so the total dispatched amount equals `delegator_pool`.
4. Create one `TaskIncome` event for the operator.
5. Create one `DaoTaskShare` event for the DAO address.
6. Create one `UserDelegation` event for each participating delegator.

## Lifecycle Rules

Relay MUST keep delegated staking state aligned with the `DelegatedStaking` contract.

When a delegation is created or increased, Relay MUST upsert the corresponding `(delegator, node, network)` record, set `slashed = false`, and update the amount and timestamps.

When a delegation is unstaked, Relay MUST delete the corresponding non-slashed delegation row.

When `delegator_share` changes, Relay MUST update `nodes.delegator_share`.

If `delegator_share` becomes `0`, Relay MUST:

- keep existing non-slashed delegation rows until each delegator exits or is slashed on chain
- stop treating the node as a delegated staking node
- keep active delegated staking in total node staking for task selection
- stop distributing task income to delegators of that node

When operator slash is confirmed through `NodeStaking.NodeSlashed`, Relay MUST create or resume the delegated slash job for that confirmed chain event. The job MUST queue bounded `DelegatedStaking::slashNodeDelegations` transactions and MUST NOT mark a delegation slashed until the matching `DelegatedStaking.DelegatorSlashed` event is confirmed. A later confirmed `NodeStaking.NodeSlashed` event for the same node address MUST use a new delegated slash job after previous jobs are completed.

For each confirmed `DelegatorSlashed` event, Relay MUST atomically:

- mark the matching non-slashed delegation row `slashed = true` and update its amount to the slashed amount
- write one `delegated_staking_slash_records` record with node, delegator, network, amount, slash transaction hash, block number, and log index
- emit the Relay `DelegatedStakingSlashed` event
- remove the delegation from the in-memory delegation cache

Relay MUST use `(network, slash_tx_hash, log_index)` as the chain-event idempotency key for delegated slash audit records. A delegated slash job MUST be completed only after the contract reports zero remaining delegated staking records for the node.

When a node quits, Relay MUST remove the node from task selection, but Relay MUST NOT delete or mark delegations slashed only because of the local quit action. Current delegation rows SHALL continue to follow the on-chain delegated staking state.

## Multi-Chain Delegation Handling

Delegation visibility for users MUST follow the on-chain delegated staking state on each network. A delegation MUST remain visible to the delegator while the corresponding stake remains locked in the `DelegatedStaking` contract on its network, even when the node address is no longer joined to that network in Relay.

Relay and Portal MUST distinguish user-visible delegation state from node selection state:

- Active user delegation: the non-slashed delegation row exists and its blockchain network matches the node's current blockchain network.
- Inactive user delegation: the non-slashed delegation row exists and its blockchain network differs from the node's current blockchain network.
- Slashed user delegation: the delegation row has `slashed = true`.
- Removed user delegation: no delegation row exists because the delegation was normally unstaked.

Portal MUST include active, inactive, and slashed user delegations in the delegator's delegated staking list. Portal MUST label each record as `Active`, `Inactive`, or `Slashed`, and MUST show the delegation network and the node's current blockchain network when they differ. Inactive and slashed delegations MUST NOT be presented as contributing to the node's current staking score or task selection weight.

Portal MUST allow a delegator to unstake an inactive delegation from the delegation's own network. After the unstake is confirmed on that network, the delegation MUST no longer appear in the delegator's delegated staking list. Unstaked delegations MUST NOT remain visible as historical delegation records in Portal and MUST NOT be required as current delegation records in Relay.

When a node quits network `A` and later joins network `B`, delegations that remain locked on network `A` MUST remain visible to their delegators as inactive delegations. Those delegations MUST NOT contribute to the node's staking score, task selection weight, node delegated staking total, node delegator count, or task income distribution while the node's current blockchain network is `B`.

When a delegator unstakes from network `A` after the node has joined network `B`, Portal MUST remove that network `A` delegation from the delegator's delegated staking list after the network `A` unstake is confirmed. The node's current blockchain network MUST NOT prevent the user-visible delegation state for network `A` from reflecting the confirmed unstake.

When a node quits network `A` and later joins network `A` again, Relay MUST rebuild the node's network `A` delegated staking state from the network `A` contract. Portal MUST show only delegations that still exist on network `A` after that rebuild as active delegations.

When a delegation is slashed, Relay MUST preserve a user-visible `slashed = true` delegation row for the delegator. The slashed record MUST NOT be counted as an active or inactive delegation, MUST NOT be unstakable, and MUST NOT contribute to staking score, task selection weight, node delegated staking total, node delegator count, or task income distribution.

Examples:

1. A user delegates to node `N` on network `A` while node `N` is joined to network `A`. Portal shows the delegation as active, and the delegation contributes to node `N` on network `A`.
2. Node `N` quits network `A` and joins network `B`. The user's network `A` delegation remains visible in Portal as inactive while the stake remains locked on network `A`. It does not contribute to node `N` on network `B`.
3. The user unstakes the inactive network `A` delegation after node `N` has joined network `B`. After the network `A` unstake is confirmed, Portal no longer shows that delegation.
4. Node `N` later joins network `A` again. Portal shows the user's delegation as active only if the network `A` contract still contains that delegation.
5. A user's delegation is slashed. Portal shows a slashed record for the user, but the record has no unstake action and no staking or task-selection effect.

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

Relay MUST calculate node-level delegated staking APR as a simple annualized APR:

```
node delegation APR = node delegator income over the APR observation window * 365 / sum of daily delegated staking over the APR observation window
```

The APR observation window MUST end at the APR refresh time. Its start time MUST be the later of the trailing 12-month start time and `dao.apr_start_time` when `dao.apr_start_time` is configured. Relay MUST parse `dao.apr_start_time` as RFC3339, convert it to UTC, cut it to the beginning of that UTC date at `00:00:00`, and use that timestamp as the earliest APR observation time. When `dao.apr_start_time` is empty, Relay MUST use only the trailing 12-month start time.

The numerator MUST include both delegated staking task fee income and issued delegation emission income. Delegated staking task fee income MUST use `node_earnings.delegator_earning` daily rows for the node. Issued delegation emission income MUST use `vesting_delegation_emission_details.emission_amount` rows for the node. Relay MUST count the full mapped vesting grant amount, including locked and released portions of the linked aggregate vesting record. Relay MUST NOT use current incomplete-week emission estimates in APR.

The denominator MUST use `node_stakings.delegator_staking` daily rows for the node. Relay MUST calculate one APR for the node delegator pool and MUST NOT average per-delegation APR values. The APR value MUST be `0` when the denominator is `0`. Relay MUST NOT suppress APR only because the observation period contains fewer than 365 daily rows.

Relay MUST refresh delegated staking APR in `delegated_staking_node_list_snapshots` as part of the delegated staking node list snapshot rebuild. The snapshot MUST store the APR value, the number of staking snapshot days used as the denominator observation count, and the APR refresh time. Stakeable node list APIs MUST use the snapshot fields for filtering, sorting, pagination, and response data and MUST NOT aggregate `delegations`, `node_stakings`, `node_earnings`, or `user_staking_earnings` during list requests.

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

`GET /v2/delegated_staking/nodes` and `GET /v2/delegated_staking/nodes/:address` MUST include node-level delegated staking APR fields in each node response. `delegation_apr_12m` MUST be the simple annualized APR ratio, where `1.0` means 100% APR. `apr_observation_days` MUST be the number of daily delegated staking snapshot rows used in the denominator. `delegation_apr_updated_at` MUST be the Unix timestamp for the APR snapshot refresh time. The delegated staking node list API MUST support `sort_by=delegation_apr_12m`.

`GET /v1/delegator/:user_address/delegation` and `GET /v1/delegator/:user_address/delegations` MUST return user-visible delegation records with `status = active`, `status = inactive`, or `status = slashed`. They MUST include the delegation blockchain network and the node current blockchain network. Earnings lookup for these APIs MUST be keyed by `(node_address, network)`.

## Configuration

Each blockchain network configuration MUST provide:

| Key | Description |
|-----|-------------|
| `blockchains.<network>.contracts.delegated_staking` | On-chain `DelegatedStaking` contract address |
| `blockchains.<network>.delegated_staking_slash_batch_size` | Maximum delegator addresses in one delegated slash transaction |
| `blockchains.<network>.delegated_staking_read_page_size` | Page size for contract delegation reads |
| `dao.apr_start_time` | Earliest UTC date included in delegated staking APR observation |

Delegated staking state, earnings, and queries MUST always use the delegation blockchain network as the isolation key. Selection and settlement MUST use the selected node's current blockchain network.
