# Task Fee Charge and Settlement Specification

This document specifies how task fee amount is charged, refunded, and distributed in relay account ledger.

## Scope

This specification covers:

- task creation charge behavior
- task refund behavior
- successful task settlement split
- rounding and remainder allocation
- ledger event requirements

## Definitions

- Task fee: the `task_fee` value on `InferenceTask`.
- Relay account: per-address off-chain balance in `relay_accounts`.
- Relay account event: immutable ledger entry in `relay_account_events`.
- Node: the selected node address that executes the task.

## Required Event Types

Task fee lifecycle MUST use these relay account event types:

- `TaskPayment`
- `TaskRefund`
- `TaskIncome`
- `DaoTaskShare`

Relay account event type values MUST follow this compatibility contract:

- `0 = TaskIncome`
- `1 = DaoTaskShare`
- `2 = WithdrawFeeIncome`
- `3 = Deposit`
- relay-account-only extensions MUST use values starting at `4`

Relay MUST reuse historical `task_fee_events` by table rename to `relay_account_events`. Relay MUST NOT import `task_quota_events` rows into relay account event history.

## Relay Wallet Event Application Contract

Relay Wallet synchronization MUST fetch relay account events from `GET /v1/relay_account/event_logs` as a contiguous ID stream and MUST preserve checkpoint continuity for every received ID.

Relay Wallet balance application contract SHALL be:

- apply `TaskIncome`
- apply `DaoTaskShare`
- apply `WithdrawFeeIncome`
- apply `Deposit`
- apply `TaskPayment`
- apply `TaskRefund`
- skip `Withdraw`
- skip `WithdrawRefund`

Relay event log rows include a `payload` field as a JSON-encoded string. For `Deposit`, payload encodes `tx_hash` and `network`. Vesting payload requirements are specified in `deposit_withdraw_and_risk_control.md`. For other non-deposit event types, payload is `{}`.

For skipped event types, Relay Wallet MUST still verify integrity and MUST still advance checkpoint to keep event-order alignment with withdrawal synchronization watermark.

## Charge Rules

When task is created, Relay MUST:

1. Validate creator relay account balance is greater than or equal to `task_fee`.
2. Create one `TaskPayment` event for creator address.
3. Decrease creator relay account balance by `task_fee`.

If balance is insufficient, task creation MUST fail and no task fee event may be persisted.

## Refund Rules

When task reaches a refunding terminal state, Relay MUST:

1. Create one `TaskRefund` event for creator address.
2. Increase creator relay account balance by refund amount.

Refund amount MUST equal the task fee amount for the corresponding task commitment.

## Settlement and Distribution Rules

When task settlement is successful, Relay MUST split payment into node and DAO income.

For each settled payment unit `payment`:

1. Compute DAO income:
   `dao_income = floor(payment * dao_percent / 100)`
2. Compute node income:
   `node_income = payment - dao_income`
3. Create `TaskIncome` event for node address with `node_income`.
4. Create `DaoTaskShare` event for DAO address with `dao_income`.
5. Increase balances for both addresses by their event amounts.

## Group Settlement and Rounding

For grouped task validation settlement:

1. Compute per-task payment by QoS-weighted integer division.
2. Track all division remainders across valid tasks.
3. Add total remainder to the last valid task payment.

This policy MUST preserve total distributed amount equal to the total payable amount.

## Consistency Guarantees

Relay MUST keep these invariants:

- Total charge minus total refund plus total income equals net balance delta per address.
- Event ordering by ID is monotonic and deterministic.
- Processed events must not be re-applied.

## API Visibility Requirements

Task fee-related user visibility MUST include:

- payment records from `TaskPayment`
- income records from `TaskIncome`
- share records from `DaoTaskShare`
- refund records from `TaskRefund`

Client API `/v1/client/:address/task_fee` MUST expose processed records for these types.
