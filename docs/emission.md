# Emission Specification

This document specifies the Relay emission flow from task fee accounting to vesting creation, vesting release, and emission chart presentation.

## Scope

This specification covers:

- emission week boundaries
- task fee inputs used for emission allocation
- node and delegation emission type split
- vesting record creation and release
- emission chart aggregation and API contracts

## Definitions

- Emission start time: `dao.mainnet_start_time` in Relay configuration.
- Emission week: a seven-day interval anchored to the configured emission start time.
- Task fee: settled task income stored in Relay earning tables and relay account events.
- Node emission: emission assigned to a node operator role.
- Delegation emission: emission assigned to a delegator role.
- Vesting record: signed emission grant stored in `vesting_records`.
- Vesting release: materialized relay account balance credited from a vesting record.

## Emission Week Rules

Relay MUST parse `dao.mainnet_start_time` as RFC3339 and convert it to UTC. Relay MUST cut the parsed UTC time to the beginning of that UTC date at `00:00:00` and use that day-start timestamp as the emission week anchor. Relay MUST NOT normalize the anchor to Monday or to any calendar week boundary.

Emission week numbers are one-based in business descriptions. Emission week 1 is the first seven-day interval starting at the emission week anchor.

```text
week_number = 1: [emission_week_anchor, emission_week_anchor + 7 days)
week_number > 1: [emission_week_anchor + (week_number - 1) * 7 days, emission_week_anchor + week_number * 7 days)
```

Relay implementation uses a zero-based week index internally. Internal week index `0` MUST correspond to business emission week 1, index `1` MUST correspond to business emission week 2, and so on.

The current incomplete emission week MUST be excluded from emission CSV export. A week is complete only after its seven-day interval has fully elapsed.

For example, if `dao.mainnet_start_time = 2026-06-01T12:30:00Z`, the emission week anchor is `2026-06-01T00:00:00Z`, and the first emission week is `[2026-06-01T00:00:00Z, 2026-06-08T00:00:00Z)`. On `2026-06-09`, the week starting `2026-06-08T00:00:00Z` is still incomplete and MUST NOT be included in emission CSV export.

## Task Fee Inputs

Relay task settlement MUST split task payment into DAO income, node operator income, and optional delegated staking income according to `task_fee_charge_and_settlement.md` and `delegated_staking.md`.

Emission allocation uses task fee income that has already been persisted in earning tables:

- Node operator task fee MUST be read from `node_earnings.operator_earning` and grouped by `node_address`.
- Delegator task fee MUST be read from `user_staking_earnings.earning` and grouped by `(user_address, node_address, network)`.
- Rows with non-positive task fee MUST be excluded.
- Node and delegation rows MUST remain separate even when the same address appears in both roles.

## Weekly Emission Allocation

The admin endpoint `GET /v2/admin/emission/task_fee_csv` MUST export allocation data for the previous complete emission week only.

Relay MUST compute the previous complete emission week from `dao.mainnet_start_time` and the current UTC time. If no complete emission week exists, Relay MUST reject the export.

The CSV MUST include task fee participants from the selected week. Relay MUST compute the node emission pool from the tokenomics weekly emission schedule:

- Year 1 MUST use 70 percent of weekly emission as the node emission pool.
- Years 2 through 20 MUST use 80 percent of weekly emission as the node emission pool.
- Year selection MUST use the zero-based internal emission week index divided by 52.
- Emission weeks outside Years 1 through 20 MUST be rejected.

Relay MUST allocate the node emission pool across all node and delegation participants in proportion to each row's task fee. Relay MUST allocate emission in integer CNX units:

```text
row_emission = floor(row_task_fee * node_emission_pool / total_task_fee)
```

The CSV MUST include `task fee` and `emission` as CNX amounts with exactly six decimal places. Emission values MUST represent integer CNX amounts formatted with six fractional zero digits. Relay MUST omit participant rows whose allocated emission is less than `1` CNX. Omitted sub-1-CNX allocations MUST remain in the `remainder` row.

The CSV MUST include `start_time` as the Unix timestamp for the vesting start time. `start_time` MUST equal the selected emission week's exclusive end boundary, which is the UTC `00:00:00` timestamp at `emission_week_anchor + (week_index + 1) * 7 days`. Every CSV row, including the `remainder` row, MUST contain the same `start_time` value.

The CSV MUST contain `address`, `type`, `task fee`, `emission`, `start_time`, `node_address`, and `network` columns. The CSV MUST NOT contain a `user_address` column. The `address` column is the vesting recipient address. The CSV MUST use `type = node` for node operator rows and `type = delegation` for delegator rows. Node rows MUST leave `node_address` and `network` empty. Delegation rows MUST include the delegated staking node address in `node_address` and the blockchain network in `network`; the delegator wallet address MUST be read from `address`. Relay MUST append a `remainder` row containing any integer CNX amount left after floor division. The remainder row is not a vesting recipient.

## Vesting Creation

Emission grants MUST be created through `POST /v2/admin/vesting`. Each item MUST include:

- `address`
- `total_amount`
- `start_time`
- `duration_days`
- `type`
- `admin_signature`
- `delegation_details`, required for `type = delegation` items and forbidden for other types

The `type` field MUST be one of:

- `node`
- `delegation`
- `other`

Relay MUST validate the admin signature against the configured `admin.vesting_signer_address`. The signed payload MUST include address, amount, start time, duration, and `type`. Relay MUST reject records whose signed payload does not match the submitted fields.

Relay MUST store created records in `vesting_records` with `released_amount = 0` and `status = active`. Relay MUST create a `VestingCreated` relay account event with zero amount for each created record. `VestingCreated` MUST anchor the signed vesting schedule and MUST NOT change relay account balance.

The `(type, address, start_time)` tuple MUST identify a vesting item and MUST remain unique.

For `type = delegation`, the admin submitter MUST create one signed aggregate vesting item per wallet-level group and attach the original delegation CSV rows as `delegation_details`. Relay MUST reject `type = delegation` items with an empty `delegation_details` list. Each delegation detail MUST contain `node_address`, `network`, `task_fee`, `emission_amount`, and `start_time`; it MUST NOT contain `user_address`. Relay MUST derive the detail user address from the aggregate item `address`. Relay MUST validate that every detail has non-empty `node_address`, non-empty `network`, positive `task_fee`, positive `emission_amount`, and the same `start_time` as the aggregate item. Relay MUST reject duplicate delegation details with the same `(address, node_address, network, start_time)` tuple. Relay MUST reject the aggregate item unless the sum of `delegation_details.emission_amount` equals `total_amount`.

Relay MUST create the aggregate `vesting_records` row and all linked `vesting_delegation_emission_details` rows in one transaction. The detail table MUST store delegation emission attribution by `vesting_record_id`, `user_address`, `node_address`, `network`, `task_fee`, `emission_amount`, and `start_time`. The `(user_address, node_address, network, start_time)` tuple MUST remain unique. The vesting admin signature MUST continue to cover only the aggregate vesting record fields and MUST NOT include detail rows.

Vesting records for a node address MAY be marked with `slashed = true` when that node address is slashed, regardless of vesting type. The `status` field MUST continue to represent only the release lifecycle. The `slashed` field MUST represent slash eligibility and release eligibility.

## Vesting Release

Relay MUST run vesting release processing periodically. For each active unslashed vesting record, Relay MUST compute the amount that should have been released by the schedule:

```text
should_released = floor(total_amount * elapsed_days / duration_days)
```

If `elapsed_days >= duration_days`, `should_released` MUST equal `total_amount`. If current time is before `start_time`, `should_released` MUST be zero.

When `should_released > released_amount`, Relay MUST:

1. Create a `VestingRelease` relay account event for `should_released - released_amount`.
2. Update `vesting_records.released_amount` to `should_released`.
3. Mark the record completed when `released_amount = total_amount`.
4. Apply the same balance delta to the in-memory relay account cache.

The `VestingRelease` event and the vesting record update MUST be committed in one transaction. Release processing MUST be catch-up and idempotent; missed runs MUST release accumulated delta, and repeated runs at the same checkpoint MUST NOT create duplicate credits.

Slashed vesting records MUST NOT release and MUST NOT contribute to relay account locked amount calculations. Slashing MUST NOT change `released_amount`, `start_time`, `duration_days`, `total_amount`, or `type`.

The admin endpoint `POST /v2/admin/vesting/restore` MUST restore slashed vesting records for the submitted `node_address` by setting `slashed = false` across all vesting types. Restore MUST keep the original schedule and `released_amount`. After restore, the next release run MUST release the catch-up delta when the schedule requires `should_released > released_amount`.

## Emission Chart Aggregation

Emission chart APIs MUST aggregate from `vesting_records.total_amount`. Chart data represents emission grants by vesting start week, not released vesting balance. Slashed vesting records MUST remain included in emission chart totals because slashing does not remove the historical grant.

Each vesting record MUST be assigned to the emission week containing `vesting_records.start_time`, using the exact `emission_week_anchor + n * 7 days` week boundaries. Vesting records before the emission week anchor MUST be excluded from emission chart buckets.

Chart APIs MUST include the current emission week start bucket because vesting records are displayed by submitted `start_time`, not by completed task fee week. For a requested `weeks` value, Relay MUST return exactly `weeks` timestamps and exactly `weeks` amount values per returned series after validating the requested range. Missing data MUST be represented as zero.

The default chart range MUST be 24 weeks. The maximum accepted chart range MUST be 260 weeks.

If fewer than the requested number of chart buckets exist since the emission week anchor, Relay MUST still return the requested number of buckets ending at the current emission week start bucket. Buckets before the first emission week MUST contain zero.

## Chart API Contracts

The relay account chart endpoint is:

```text
GET /v2/relay_account/:address/emission/chart?weeks=24
```

The endpoint MUST require JWT authentication and MUST reject address mismatch. The response data MUST contain:

- `timestamps`: emission week start Unix timestamps
- `node_emission_income`: node-type vesting totals per week
- `delegation_emission_income`: delegation-type vesting totals per week

The relay account vesting list endpoint MUST include slashed records and MUST expose the `slashed` boolean field. Clients MUST display a slashed record as inactive before applying the numeric release status label.

The stakeable node chart endpoint is:

```text
GET /v2/delegated_staking/nodes/:address/emission/chart?weeks=24
```

The endpoint MUST be accessible only for a node that exists and has active delegated staking visibility. Emission chart aggregation is not scoped by network. The response data MUST contain:

- `timestamps`: emission week start Unix timestamps
- `node_emission_income`: node-type vesting totals per week

Chart aggregation MUST use only the canonical vesting types. The relay account chart MUST include only `type = node` and `type = delegation`. The stakeable node chart MUST include only `type = node`. Chart aggregation MUST NOT include `type = other`, `type = delegator`, or any other non-canonical type.

## Relationship Summary

Task settlement creates task fee income. Weekly emission allocation reads the settled task fee income for the previous complete emission week and computes emission amounts for node operators and delegation details. Admin vesting creation turns those emission amounts into signed vesting records and stores delegation detail mappings for delegation emission records. Vesting release materializes those records into relay account balance over time. Emission charts read the original vesting grant amounts, split by vesting type and aligned to emission weeks.
