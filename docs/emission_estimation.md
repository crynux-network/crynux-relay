# Emission Estimation Specification

This document specifies Relay estimated upcoming emission for the current incomplete emission week.

## Scope

Relay MUST expose estimated upcoming emission for:

- a node operator
- all delegations under one node
- all delegations owned by one staker
- one staker delegation on one node and one blockchain network

Estimated emission is informational. It MUST NOT create vesting records, release balances, or change task fee settlement.

## Emission Week

Relay MUST use the current incomplete emission week anchored by `dao.mainnet_start_time`. The emission week start and end MUST use the same seven-day boundary rules as `emission.md`.

Relay MUST include `emission_week_start`, `emission_week_end`, and `estimate_updated_at` in API responses that expose an estimate. These fields MUST be Unix timestamps in seconds.

## Task Fee Inputs

Relay MUST read current-week task fee from persisted earning tables:

- `node_earnings.operator_earning` grouped by `node_address` for node operator estimates.
- `node_earnings.delegator_earning` grouped by `node_address` for all delegations under one node.
- `user_earnings.earning` grouped by `user_address` for all delegations owned by one staker.
- `user_staking_earnings.earning` grouped by `user_address`, `node_address`, and `network` for one delegation on one blockchain network.

The total task fee denominator MUST be:

```text
total_task_fee = sum(node_earnings.operator_earning) + sum(user_earnings.earning)
```

Rows with non-positive task fee MUST NOT contribute to scope task fee or total task fee.

## Calculation

Relay MUST estimate emission by applying the current week node emission pool to the current-week task fee share:

```text
estimated_upcoming_emission = floor(scope_task_fee * current_week_node_emission_pool / total_task_fee)
```

If `total_task_fee = 0` or `scope_task_fee = 0`, Relay MUST return zero estimated emission.

The current week node emission pool MUST use the tokenomics weekly emission schedule and node allocation percentage defined in `emission.md`.

## Snapshot Refresh

Relay MUST maintain one in-memory current emission estimate snapshot. The snapshot MUST contain:

- current week total task fee
- operator task fee by node
- delegation task fee by node
- delegation task fee by staker
- delegation task fee by staker, node, and blockchain network
- current emission week start and end
- snapshot update time

Relay MUST build the snapshot with database aggregation queries during refresh. API handlers MUST read from the snapshot and MUST NOT aggregate earning tables per request.

Relay MUST refresh the snapshot on startup and every 4 hours after startup.

## Delegation Status

Single-delegation estimates MUST be available for active, inactive, and slashed delegation records.

An active delegation estimate MAY grow while task fee is distributed to the delegation during the current emission week. An inactive or slashed delegation estimate MUST reflect only task fee already distributed to that delegation during the current emission week.

If a node changes its current blockchain network away from a delegation's blockchain network, the delegation estimate MUST remain scoped to the delegation's stored blockchain network.
