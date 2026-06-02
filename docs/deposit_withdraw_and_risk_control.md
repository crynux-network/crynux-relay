# Deposit, Withdraw and Risk Control

This document defines the relay account funding, charging, income distribution, and withdrawal flow across Relay and Relay Wallet.

## Two-Service Architecture

The system has two isolated services:

- Relay: public-facing coordination service.
- Relay Wallet: key-holding execution service with no inbound control plane from Relay.

Relay Wallet MUST pull work from Relay. Relay MUST NOT push withdrawal execution commands directly to Relay Wallet.

## Relay Account Ledger

Relay uses a single ledger model:

- `relay_accounts`: per-address current balance.
- `relay_account_events`: append-only balance change log.

For the detailed processing pipeline from event creation to in-memory cache update and DB projection, see [relay_account_event_cache_flow.md](./relay_account_event_cache_flow.md).

Relay account events MUST be the only source used by Relay Wallet to reconstruct balances.

### Relay Account Event Types

| Type | Name | Balance Effect |
|------|------|----------------|
| 0 | `TaskIncome` | Increase |
| 1 | `DaoTaskShare` | Increase |
| 2 | `WithdrawFeeIncome` | Increase |
| 3 | `Deposit` | Increase |
| 4 | `TaskPayment` | Decrease |
| 5 | `TaskRefund` | Increase |
| 6 | `Withdraw` | Decrease |
| 7 | `WithdrawRefund` | Increase |
| 8 | `UserDelegation` | Increase |
| 9 | `VestingCreated` | No balance change |
| 10 | `VestingRelease` | Increase |

`TaskIncome` is for node operator settlement income after delegated staking distribution. `DaoTaskShare` is for DAO-side settlement share. `UserDelegation` is for delegator-side settlement income.
`VestingCreated` anchors a signed vesting schedule. `VestingRelease` materializes released vesting amount into relay account balance.

`relay_account_events` MUST contain the complete relay account balance history for all relay account event types defined in this document.

`task_quota_events` MUST NOT be treated as relay account history and MUST NOT affect relay account balances.

## Deposit

There is no API to initiate a deposit.

A deposit happens when a user sends a native token transfer to `relay_account.deposit_address` on a supported network.

Relay MUST:

1. Detect successful native transfers to the configured deposit address.
2. Enforce idempotency per transaction hash and network.
3. Create a `relay_account_events` record of type `Deposit` with reason format `3-{tx_hash}-{network}`.
4. Create a `deposit_records` record for client query with `local_status = Pending`, and store the linked relay account event ID in `deposit_records.relay_account_event_id`.
5. Apply the sender balance delta to in-memory cache in the business path and rely on background event projection to persist final balance in `relay_accounts`.
6. Update `deposit_records.local_status` to `Processed` or `Invalid` when background event projection finalizes the linked deposit event.

Client deposit-list API MUST return only rows with `deposit_records.local_status = Processed`.

## Task Charge, Refund and Income

### Task Charge

On task creation, Relay MUST:

1. Validate creator relay account balance is sufficient for `task_fee`.
2. Create a `TaskPayment` relay account event for the creator.
3. Decrease creator relay account balance by `task_fee`.

### Task Refund

If task reaches refund or abort terminal paths, Relay MUST:

1. Create a `TaskRefund` event for the creator.
2. Increase creator relay account balance by the refunded amount.

### Task Income Distribution

On successful settlement, Relay MUST:

1. Split task fee by DAO ratio.
2. Create one `DaoTaskShare` event for the DAO address.
3. Create one `TaskIncome` event for the node operator address with the operator-retained amount.
4. When delegated staking is enabled and the node has active delegations on the task network, create `UserDelegation` events for delegators by proportional stake split of the configured delegator pool.
5. Increase all recipient balances by their split amounts.

## Withdraw

A withdrawal converts relay account balance to on-chain native tokens.

### Relay-Side Flow

On `POST /v1/client/:address/withdraw`, Relay MUST:

1. Validate JWT, signature, amount, and benefit address.
2. Compute withdrawal fee based on configured policy. Set `withdrawal_fee` to zero when requester address equals `dao.task_fee_share_address` or `withdraw.withdrawal_fee_address`.
3. Create a `withdraw_records` row with `Status = Pending`.
4. Create a `Withdraw` relay account event for `amount + withdrawal_fee`.
5. Store the created relay account event ID into `withdraw_records.relay_account_event_id`.
6. Decrease requester relay account balance by `amount + withdrawal_fee`.

When wallet fulfills a withdrawal, Relay MUST:

1. Mark withdrawal `Status = Success`.
2. Create `WithdrawFeeIncome` event to fee collection address when fee is non-zero.
3. Increase fee collection address relay account balance by fee amount.

When wallet rejects a withdrawal, Relay MUST:

1. Mark withdrawal `Status = Failed`.
2. Create `WithdrawRefund` event for requester.
3. Increase requester relay account balance by `amount + withdrawal_fee`.

### Withdraw Local Status and Event Status Relationship

This flow uses three status fields with different business purposes:

- `withdraw_records.status`: user-facing withdrawal outcome (`Pending`, `Success`, `Failed`).
- `withdraw_records.local_status`: Relay-side execution gate for wallet handoff (`Pending`, `Processed`, `Invalid`).
- `relay_account_events.status`: background event processing state (`Pending`, `Processed`, `Invalid`).

Withdraw processing SHALL be interpreted as one continuous pipeline:

1. Client submits a withdraw request.
   - Relay creates one `withdraw_records` row.
   - `withdraw_records.status = Pending` means user-visible processing is not finished.
   - `withdraw_records.local_status = Pending` means the in-memory cache update is already applied, but DB projection is not finally confirmed yet and must wait for background syncer confirmation.
   - Relay creates one `Withdraw` event with `relay_account_events.status = Pending`.
   - Relay stores event ID in `withdraw_records.relay_account_event_id`.
   - Relay applies runtime in-memory cache deduction immediately.

2. Background sync handles pending events.
   - Background sync reads `relay_account_events.status = Pending`.
   - It validates event binding and projects balance change into `relay_accounts`.
   - It sets `Withdraw` event status to `Processed`.
   - In the same transaction, Relay promotes linked `withdraw_records.local_status = Processed` and writes withdraw record MAC.
   - Meaning: Relay-side accounting and local readiness are done, and wallet execution is now allowed.

3. Wallet executes and reports result.
   - Before fulfill/reject, Relay MUST require `withdraw_records.local_status = Processed`.
   - `withdraw_records.status` remains user-facing:
     - Fulfill sets `status = Success`.
     - Reject sets `status = Failed`.
   - On reject, Relay keeps `local_status = Processed` and creates compensating refund event flow.

4. Outcome settlement stays event-driven.
   - Fulfill with non-zero fee creates `WithdrawFeeIncome` event (`Pending`), updates runtime cache, then background sync projects to `relay_accounts` and marks event `Processed`.
   - Reject creates `WithdrawRefund` event (`Pending`), updates runtime cache, then background sync projects to `relay_accounts` and marks event `Processed`.

This design solves three different requirements at the same time:

- user-facing progress and result tracking (`withdraw_records.status`)
- wallet execution safety gate (`withdraw_records.local_status`)
- auditable and replayable accounting pipeline (`relay_account_events.status` + `relay_accounts`)

## Vesting

Relay MUST support vesting through `relay_account_events` and `vesting_records`:

1. `VestingCreated` events MUST be created only by admin APIs protected by admin token and admin signer signature verification.
2. Every vesting creation item MUST be signed by the configured `admin.vesting_signer_address`.
3. `VestingCreated` reason MUST contain the vesting record ID. Relay MUST build the wallet-facing creation payload from the corresponding `vesting_records` row.
4. Release processing MUST be catch-up and idempotent. For each vesting record, Relay computes:
   - `should_released = min(total_amount, total_amount * elapsed_days / duration_days)`
   - `release_amount = should_released - released_amount`
5. Relay MUST emit `VestingRelease` only when `release_amount > 0`.
6. Updating `vesting_records.released_amount` and creating `VestingRelease` MUST be in one transaction.
7. `VestingRelease` reason MUST uniquely identify the release checkpoint to prevent duplicate credits on retries.

Spendable balance is driven by materialized ledger events. The locked vesting query is schedule-based and SHOULD report `total_amount - should_released` at request time.

## Relay Wallet Synchronization Rules

Relay Wallet MUST synchronize in event-order:

1. Sync `relay_account_events` in ascending ID order through `GET /v1/relay_account/event_logs`.
2. Verify signatures and integrity constraints before apply.
3. Apply each event to local account table using event type balance effect.
4. Sync withdrawal records and only execute records whose `relay_account_event_id` is not greater than the last applied relay account event ID.

Relay Wallet MUST reject or halt on ordering or integrity violations.

Relay account events MUST be retained as a complete ledger for audit, including `Withdraw` and `WithdrawRefund` events.

Relay Wallet should skip applying `Withdraw` and `WithdrawRefund` to its local balance if withdrawal deduction and rollback are handled by withdrawal processing flow.

Relay event logs MUST include a `payload` field as a JSON-encoded string. For `Deposit` events, `payload` MUST encode `tx_hash` and `network` derived from event reason. For `VestingCreated` events, `payload` MUST encode the signed vesting schedule and set `released_amount` to `0`. For `VestingRelease` events, `payload` MUST encode only `vesting_id`; Relay Wallet MUST derive release state from its local vesting record. For other event types, `payload` MUST be `{}`.

When Relay Wallet skips applying an event type, it MUST still:

1. Verify event integrity.
2. Keep event-order continuity.
3. Advance sync checkpoint by event ID.

`withdraw_records.relay_account_event_id` MUST be treated as the event anchor used to bind each withdrawal request to its `Withdraw` ledger event.

## Integrity and Authentication

- Client withdrawal requests MUST carry valid JWT and user signature.
- Wallet-facing APIs MUST use wallet signature authentication.
- Relay MUST generate MAC for processed ledger and withdrawal records.
- Relay Wallet MUST verify MAC before acceptance.

## Risk Control

### Relay Controls

- Relay MUST validate sufficient relay account balance before task charge and withdrawal charge.
- Relay MUST validate benefit address from on-chain source before withdrawal acceptance.
- Relay MUST enforce idempotency on deposit events.

### Relay Wallet Controls

- Relay Wallet MUST enforce batch-level risk limits on synced relay account events.
- Relay Wallet MUST treat each withdrawal request debit as `amount + withdrawal_fee`, matching the Relay-side `Withdraw` ledger event amount.
- Relay Wallet MUST process withdrawal records serially. A withdrawal record MUST complete chain execution, Relay fulfill/reject callback, and local finalization before the next withdrawal record starts processing.
- Relay Wallet MUST re-check local balance before marking a fulfilled withdrawal as deducted.

Serial withdrawal processing prevents multiple wallet-side withdrawal records from queuing or monitoring chain transfers against the same local balance before any confirmed transfer has been deducted. This control preserves a single local balance value without introducing reserved-balance state.

## Data Models

### Relay `withdraw_records`

| Field | Type | Description |
|-------|------|-------------|
| `address` | string | User address |
| `benefit_address` | string | Destination on-chain address |
| `amount` | BigInt | Requested withdraw amount |
| `network` | string | Target blockchain network |
| `status` | enum | `Pending`, `Success`, `Failed` |
| `local_status` | enum | `Pending`, `Processed`, `Invalid` |
| `relay_account_event_id` | uint | Ledger ordering anchor |
| `tx_hash` | string nullable | Fulfillment transaction hash |
| `withdrawal_fee` | BigInt | Fee amount |
| `mac` | string | Integrity tag |

### Relay `deposit_records`

| Field | Type | Description |
|-------|------|-------------|
| `address` | string | Depositor address |
| `amount` | BigInt | Deposit amount |
| `network` | string | Source network |
| `tx_hash` | string | Deposit transaction hash |
| `relay_account_event_id` | uint | Ledger ordering anchor |
| `local_status` | enum | `Pending`, `Processed`, `Invalid` |

## Configuration

### Relay `relay_account`

| Key | Type | Description |
|-----|------|-------------|
| `deposit_address` | string | Native token deposit target |

### Relay `withdraw`

| Key | Type | Description |
|-----|------|-------------|
| `relay_wallet_address` | string | Wallet service signer address |
| `min_withdrawal_amount` | uint64 | Minimum withdraw amount |
| `withdrawal_fee` | uint64 | Withdraw fee. This fee is waived for `dao.task_fee_share_address` and `withdrawal_fee_address` |
| `withdrawal_fee_address` | string | Relay operator fee income address |

## API Endpoints

### Client APIs

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/client/:address/withdraw` | Create withdrawal request |
| `GET` | `/v1/client/:address/withdraw/list` | Query withdrawals |
| `GET` | `/v1/client/:address/deposit/list` | Query deposits |
| `GET` | `/v1/client/:address/task_fee` | Query task fee records |
| `GET` | `/v2/relay_account/:address/vesting/locked` | Query locked vesting amount |
| `GET` | `/v2/relay_account/:address/vesting/list` | Query vesting records with pagination |

### Wallet APIs

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/relay_account/event_logs` | Query relay account events |
| `GET` | `/v1/withdraw/list` | Query pending withdrawals |
| `POST` | `/v1/withdraw/:id/fulfill` | Mark fulfilled |
| `POST` | `/v1/withdraw/:id/reject` | Mark rejected |
