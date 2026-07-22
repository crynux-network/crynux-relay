# Historical And Estimated Delegation APR Specification

This document specifies how Relay calculates the node-level delegation APR values:

- historical delegation APR: `delegation_apr_12m` and `apr_observation_days`
- estimated next delegation APR: `estimated_next_10k_delegation_apr`, `estimated_next_100k_delegation_apr`, and `estimated_next_1m_delegation_apr`

All APR values are simple annualized APR ratios where `1.0` means 100% APR.

## Historical Delegation APR vs Estimated Next Delegation APR

The two APR families answer different questions and MUST NOT be mixed:

- The historical delegation APR (`delegation_apr_12m`) is backward-looking. It reports the annualized return the node delegator pool actually realized over the APR observation window. Its income input is realized income only: settled delegator task fee and issued delegation emission. It MUST NOT include any estimate for the current incomplete emission week.
- The estimated next delegation APR (`estimated_next_10k/100k/1m_delegation_apr`) is forward-looking. It reports the annualized return a new delegation of the fixed amount placed on the node now is projected to receive. Its income input is the current emission week projection: the estimated upcoming delegator emission plus the delegator task fee accumulated in the current emission week. It MUST NOT use the historical delegation APR observation window income.

A node with a long staking history and little realized emission has a low historical delegation APR and can still have a high estimated next delegation APR. The two value families use independent income inputs, independent time bases, and independent zero conditions.

In both APR families, delegator income is dominated by delegation emission. The task fee amounts are negligible compared to the emission amounts: the task fee functions as the allocation basis that determines each node's share of the weekly emission pool, not as a material income source. An APR value is therefore driven by the emission input, and an APR near `0` indicates missing emission income for the node, not missing task fee income.

The estimated next delegation APR is a steady-state full-week rate expressed as an annualized ratio. Income realized by a new delegation during the remainder of the current emission week is lower than this rate implies, because the delegation participates only from its stake time. The rate applies from the first full emission week after the delegation.

## Historical Delegation APR

`delegation_apr_12m` is the simple annualized APR realized by the node delegator pool over the APR observation window:

```
delegation_apr_12m = node delegator income over the APR observation window * 365 / sum of daily delegated staking over the APR observation window
```

The APR observation window MUST end at the APR refresh time. Its start time MUST be the later of the trailing 12-month start time and `dao.apr_start_time` when `dao.apr_start_time` is configured. Relay MUST parse `dao.apr_start_time` as RFC3339, convert it to UTC, cut it to the beginning of that UTC date at `00:00:00`, and use that timestamp as the earliest APR observation time. When `dao.apr_start_time` is empty, Relay MUST use only the trailing 12-month start time. `dao.apr_start_time` bounds only the historical delegation APR observation window.

The numerator MUST include both delegated staking task fee income and issued delegation emission income. Delegated staking task fee income MUST use `node_earnings.delegator_earning` daily rows for the node. Issued delegation emission income MUST use `vesting_delegation_emission_details.emission_amount` rows for the node. Relay MUST count the full mapped vesting grant amount, including locked and released portions of the linked aggregate vesting record. The historical delegation APR MUST NOT use current incomplete-week emission estimates.

The denominator MUST use `node_stakings.delegator_staking` daily rows for the node. Relay MUST calculate one APR for the node delegator pool and MUST NOT average per-delegation APR values. The historical delegation APR MUST be `0` when the denominator is `0`. Relay MUST NOT suppress the historical delegation APR only because the observation period contains fewer than 365 daily rows.

`apr_observation_days` MUST be the number of daily `node_stakings` snapshot rows used as the historical delegation APR denominator observation count. `apr_observation_days` applies only to the historical delegation APR.

## Estimated Next Delegation APR

`estimated_next_10k_delegation_apr`, `estimated_next_100k_delegation_apr`, and `estimated_next_1m_delegation_apr` estimate the simple annualized APR a new delegation of `10,000 CNX`, `100,000 CNX`, or `1,000,000 CNX` placed on the node now would receive if current node conditions, QoS, task volume, and reward distribution remain stable.

The income basis MUST be the current emission week projection from the current emission estimate snapshot defined in [emission_estimation.md](./emission_estimation.md). The estimated next delegation APR MUST NOT use the historical delegation APR observation window income.

Relay MUST compute the projected weekly delegator income of the node as:

```
projected weekly delegator emission = estimated upcoming delegator emission of the node, converted from whole CNX to wei
current week delegator task fee = node delegator task fee accumulated in the current emission week, in wei
elapsed week time = time from the emission week start to the emission estimate snapshot update time, clamped to a minimum of 1 day and a maximum of 7 days
projected weekly delegator task fee = current week delegator task fee * 7 days / elapsed week time
projected weekly delegator income = projected weekly delegator emission + projected weekly delegator task fee
```

Both inputs MUST be read from the current emission estimate snapshot at APR recalculation time.

The estimated upcoming delegator emission is already a full-week value: it applies the node's task fee share to the full current-week emission pool, so it MUST NOT be scaled by elapsed week time. The current week delegator task fee is a partial-week accumulation, so it MUST be scaled to a full-week equivalent by elapsed week time as defined above.

For each fixed new delegation amount `X`, Relay MUST calculate:

```
new delegation pool share = X / (current delegated staking + X)
node income multiplier = projected node weight share after X / current node weight share
projected annual delegator income = projected weekly delegator income * node income multiplier * 365 / 7
estimated next X delegation APR = projected annual delegator income * new delegation pool share / X
```

The formula reduces to:

```
estimated next X delegation APR = projected weekly delegator income * node income multiplier * 365 / 7 / (current delegated staking + X)
```

`current delegated staking` MUST be the node's active delegated staking on the node current blockchain network at the APR refresh time. `current node weight share` and `projected node weight share after X` MUST use the same staking score and QoS base weight formula used by node selection. The projected node weight share after `X` MUST simulate adding `X` to the node's active delegated staking and MUST NOT mutate Relay's live staking, delegation, or max-staking caches.

The estimated next delegation APR MUST be `0` when the node is quit, the projected weekly delegator income is `0`, the current node weight share is `0`, or the projected node weight share is `0`.

## Snapshot Storage and Refresh

Relay MUST store the historical delegation APR and the estimated next delegation APR values in `delegated_staking_node_list_snapshots` together with `apr_observation_days` and the APR refresh time `delegation_apr_updated_at`.

Relay MUST recalculate all APR values during the full snapshot rebuild and during single-node snapshot refresh defined in [stakeable_node_list_filter_and_sorting.md](./stakeable_node_list_filter_and_sorting.md). Estimated next delegation APR recalculation MUST read the current emission estimate snapshot available at recalculation time.

Stakeable node list APIs MUST serve APR values from the snapshot columns and MUST NOT aggregate `delegations`, `node_stakings`, `node_earnings`, or `user_staking_earnings` during list requests.
