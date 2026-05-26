# Relay Account Event, Cache, and DB Flow

This document explains how Relay account balance changes are handled in the current Relay implementation, from event creation to in-memory balance updates and then to database projection.

It covers Task, Deposit, and Withdraw paths.

## High-Level Overview

Relay uses three linked layers for balance handling:

1. `relay_account_events`: append-only ledger events.
2. In-memory cache (`relayAccountCache`): fast runtime balance checks and mutations.
3. `relay_accounts`: persisted balance projection in the database.

The in-memory cache exists to accelerate task creation.

At runtime, Task and Withdraw operations create events and apply balance deltas to in-memory cache in the request lifecycle. A background processor later validates pending events and projects them into `relay_accounts`.

This means the observable flow is:

- business request creates ledger event
- business request updates in-memory cache
- background sync updates `relay_accounts` and marks event status

## Core Data Structures and Statuses

### Relay Account Event Types

`relay_account_events.type` includes:

- `TaskPayment`
- `TaskRefund`
- `TaskIncome`
- `DaoTaskShare`
- `Deposit`
- `Withdraw`
- `WithdrawRefund`
- `WithdrawFeeIncome`
- `VestingCreated`
- `VestingRelease`

### Relay Account Event Status

`relay_account_events.status` lifecycle:

- `Pending`: created by business flow, not yet projected by sync processor.
- `Processed`: validated and projected to `relay_accounts`.
- `Invalid`: failed validation and excluded from projection.

## End-to-End Processing Model

### 1) Business-Layer Event Creation and Cache Mutation

Business operations call service methods that:

1. create a `relay_account_events` row with `Pending` status.
2. return a cache mutation callback (`commitFunc`) that applies in-memory delta.
3. execute that callback inside transaction business flow.

Task, Deposit, and Withdraw use this pattern.

### 2) Background Event Projection to DB

`StartRelayAccountSync` runs periodic loops that:

1. load `Pending` events.
2. validate event structure and reason binding.
3. aggregate deltas per address.
4. apply deltas into `relay_accounts`.
5. write MAC and set event status to `Processed`.
6. set invalid events to `Invalid`.

This processor is the path that projects ledger events into persisted balance table.

### 3) Withdraw Local Processing Gate

The event projection transaction also processes `withdraw_records.local_status` for `Withdraw` events:

1. project the `Withdraw` event and set event status to `Processed`.
2. load pending withdraw records whose `relay_account_event_id` points to processed `Withdraw` event IDs in the same batch.
3. generate record MAC and set local status to `Processed` in the same transaction.

Wallet-facing withdraw APIs use this local status for execution gating.

For withdraw state definitions and the normative relationship between `withdraw_records.local_status` and `relay_account_events.status`, see [deposit_withdraw_and_risk_control.md](./deposit_withdraw_and_risk_control.md).

## Task Path

### Task Charge at Creation

Task creation path:

1. read creator balance from in-memory cache.
2. check sufficient balance.
3. create `TaskPayment` event (`Pending`).
4. apply cache deduction via callback.
5. transaction completes.
6. background processor later applies `TaskPayment` to `relay_accounts`.

### Task Refund and Income Settlement

Task terminal paths create:

- `TaskRefund` for refund flows.
- `TaskIncome` and `DaoTaskShare` for success settlement.

Each path applies immediate in-memory cache delta in business flow, then background processor projects those events to `relay_accounts`.

### Task Related State Changes

Task status transition logic invokes relay account event creation in:

- task creation charge
- end-aborted refund
- end-group-refund refund
- end-success income split

These are tied to task lifecycle transitions in `service/task_status.go`.

## Withdraw Path

### Withdraw Request Creation

Withdraw creation path:

1. create `withdraw_records` row (`Status=Pending`, `LocalStatus=Pending`).
2. create `Withdraw` event (`Pending`) for `amount + withdrawal_fee`.
3. save created event ID into `withdraw_records.relay_account_event_id`.
4. apply cache deduction via callback.
5. transaction completes.
6. background processor projects `Withdraw` event into `relay_accounts`.
7. the same event projection transaction marks `withdraw_records.local_status = Processed` and writes withdraw record MAC.

### Withdraw Fulfill

On fulfill:

1. require `withdraw_records.local_status = Processed`.
2. set withdraw `Status=Success`.
3. if fee is non-zero, create `WithdrawFeeIncome` event.
4. apply in-memory cache increase for fee receiver.
5. background processor later projects `WithdrawFeeIncome` into `relay_accounts`.

### Withdraw Reject

On reject:

1. require `withdraw_records.local_status = Processed`.
2. set withdraw `Status=Failed` and keep local status `Processed`.
3. create `WithdrawRefund` event for `amount + fee`.
4. apply in-memory cache refund to requester.
5. background processor later projects `WithdrawRefund` into `relay_accounts`.

## Deposit Path

### Native Transfer Ingestion

Deposit ingestion path:

1. native token listener scans configured networks and finds successful transfers to `relay_account.deposit_address`.
2. Relay creates a `Deposit` event (`Pending`) with reason format `3-{tx_hash}-{network}`.
3. Relay creates one `deposit_records` row with `local_status = Pending` and stores the created event ID into `deposit_records.relay_account_event_id`.
4. Relay applies in-memory cache increase via callback.
5. transaction completes.
6. background processor validates tx evidence, projects `Deposit` event into `relay_accounts`, and updates linked `deposit_records.local_status` to `Processed` or `Invalid`.

## Vesting Path

### Vesting Creation

Admin vesting creation path:

1. Relay verifies admin token and admin signer signature for each vesting item.
2. Relay creates one `vesting_records` row.
3. Relay creates one `VestingCreated` event (`Pending`) with zero amount and a reason that stores the vesting record ID.
4. Background processor validates reason binding and marks event `Processed`.

`VestingCreated` exists for ordering, replay protection, and wallet-side independent validation; it does not change balance.

### Vesting Release

Release task path:

1. Background task scans active vesting records.
2. For each record, computes `should_released` from schedule and current time.
3. If `should_released > released_amount`, Relay:
   - creates `VestingRelease` event (`Pending`) for `release_amount = should_released - released_amount`
   - updates `vesting_records.released_amount` in the same transaction
   - updates in-memory cache by the same `release_amount`
4. Background processor later projects `VestingRelease` into `relay_accounts`.

Release processing is catch-up and idempotent. Missed runs are recovered by releasing accumulated delta; repeated runs for the same checkpoint do not create duplicate balance credits.

## Consistency Characteristics

Current implementation characteristics are:

- Runtime sufficiency checks use in-memory cache.
- Persisted balances in `relay_accounts` are updated asynchronously from event logs.
- Event log is the canonical append-only change history.
- Task and Withdraw both use business-path cache mutation before event projection to `relay_accounts`.

## Related Source Files

- `service/relay_account.go`
- `service/task_status.go`
- `service/token_listener.go`
- `service/withdraw.go`
- `models/relay_account.go`
- `models/deposit.go`
- `models/withdraw.go`
- `api/v1/relay_account/event_logs.go`
- `api/v1/withdraw/list_withdraw_requests.go`
