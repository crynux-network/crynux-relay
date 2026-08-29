# Task Fee Charge and Settlement Specification

This document specifies how task fee amount is charged, refunded, and distributed in relay account ledger.

## Scope

This specification covers:

- task creation charge behavior
- task refund behavior
- successful task settlement split
- rounding and remainder allocation
- ledger event requirements

Task fee also participates in queue priority calculation. Queue ordering and task priority rules are specified in [task-pricing.md](./task-pricing.md). This document specifies only relay account ledger effects.

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
- `UserDelegation`

Relay account event type values MUST follow this compatibility contract:

- `0 = TaskIncome`
- `1 = DaoTaskShare`
- `2 = WithdrawFeeIncome`
- `3 = Deposit`
- `8 = UserDelegation`
- relay-account-only extensions MUST use values starting at `4`

Relay MUST reuse historical `task_fee_events` by table rename to `relay_account_events`. Relay MUST NOT import `task_quota_events` rows into relay account event history.

## Relay Wallet Event Application Contract

Relay Wallet synchronization MUST fetch relay account events from `GET /v1/relay_account/event_logs` as a contiguous ID stream and MUST preserve checkpoint continuity for every received ID.

Relay Wallet balance application contract SHALL be:

- apply `TaskIncome`
- apply `DaoTaskShare`
- apply `WithdrawFeeIncome`
- apply `Deposit`
- apply `UserDelegation`
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

If balance is insufficient, task creation MUST fail and Relay MUST NOT persist a task fee event.

For batch creation, each item MUST retain this charge and transaction boundary independently. One item failure MUST NOT roll back another item that committed successfully. Retrying the same task commitment and immutable normalized creation input MUST return the existing task without creating another `TaskPayment` event or applying another balance debit. The complete batch creation contract is specified in [task_batch_api.md](./task_batch_api.md).

## Refund Rules

When task reaches a refunding terminal state, Relay MUST:

1. Create one `TaskRefund` event for creator address.
2. Increase creator relay account balance by refund amount.

Refund amount MUST equal the task fee amount for the corresponding task commitment.

## Terminal-State Ledger Contract

Relay MUST enforce the following ledger-event contract by terminal task state:

| Terminal state | `TaskRefund` | `TaskIncome`, `DaoTaskShare`, `UserDelegation` |
|---|---|---|
| `TaskEndSuccess` | MUST NOT exist | MUST represent the successful-task payment split |
| `TaskEndGroupSuccess` | MUST NOT exist for the representative task fee | MUST represent the validation-group QoS-weighted payment split |
| `TaskEndGroupRefund` | MUST refund the duplicate task's own fee | MUST represent that node's QoS-weighted execution income when the group representative later uploads successfully |
| `TaskEndInvalidated` | MUST NOT exist | MUST NOT exist for the invalidated task |
| `TaskEndAborted` with `TaskAbortCreatorValidationTimeout` | MUST NOT exist | MUST represent the aborted task's full fee split |
| `TaskEndAborted` with any other abort reason | MUST refund the full task fee | MUST NOT exist |

The permitted `TaskIncome`, `DaoTaskShare`, and `UserDelegation` rows MUST follow the split and rounding rules in this document. Relay account event consistency validation MUST reject every event combination prohibited by this table.

`TaskAbortCreatorValidationTimeout` MUST apply when a task expires in either `TaskScoreReady` or `TaskErrorReported`. The fee split compensates the node for execution and creator-validation waiting time after the creator failed to complete the protocol. It MUST NOT indicate that Relay verified the result or error report as correct. The task MUST remain `TaskEndAborted`.

For `TaskAbortCreatorValidationTimeout`, Relay MUST settle only that task's own fee and MUST NOT use validation-group QoS-weighted allocation. Relay MUST NOT assign a validation rank, update group `Q_long`, apply a result-upload health boost, or apply a node health penalty.

`TaskAbortTimeout`, `TaskAbortResultUploadTimeout`, `TaskAbortCreatorCancelled`, `TaskAbortModelDownloadFailed`, `TaskAbortIncorrectResult`, `TaskAbortTaskFeeTooLow`, `TaskAbortGroupTimeout`, and `TaskAbortErrorReported` MUST use the `TaskEndAborted` full-refund rule and MUST NOT create task income.

Task statistics MUST classify `TaskAbortCreatorValidationTimeout` as aborted. Income statistics MUST include its actual ledger income events and MUST NOT infer income eligibility only from a success terminal state.

## Settlement and Distribution Rules

When task settlement is successful, Relay MUST split payment into DAO income, node operator income, and optional delegated staking income.

For each settled payment unit `payment`:

1. Compute DAO income:
   `dao_income = floor(payment * dao_percent / 100)`
2. Compute node-side income before delegation:
   `node_income_before_delegation = payment - dao_income`
3. If the selected node has `delegator_share > 0` and at least one non-slashed delegation row on the selected node's current blockchain network, compute delegator pool:
   `delegator_pool = floor(node_income_before_delegation * delegator_share / 100)`
4. Compute operator income:
   `operator_income = node_income_before_delegation - delegator_pool`
5. Create `TaskIncome` event for node address with `operator_income`.
6. Create `DaoTaskShare` event for DAO address with `dao_income`.
7. When delegated staking distribution is active, create one `UserDelegation` event per participating delegator by proportional split over non-slashed delegation amounts on the selected node's current blockchain network.
8. Increase balances for all recipient addresses by their event amounts.

If delegated staking distribution is inactive, `delegator_pool` MUST be `0` and Relay MUST NOT create a `UserDelegation` event.

## Group Settlement and Rounding

For grouped task validation settlement:

1. Compute per-task payment by QoS-weighted integer division.
2. Track all division remainders across valid tasks.
3. Add total remainder to the last valid task payment.

This policy MUST preserve total distributed amount equal to the total payable amount.

If any group member reaches `TaskEndAborted` with `TaskAbortCreatorValidationTimeout`, Relay MUST permanently reject group validation. Every remaining group member MUST retain its independent creator-validation deadline and MUST settle independently if that deadline expires.

## Consistency Guarantees

Relay MUST keep these invariants:

- Total charge minus total refund plus total income equals net balance delta per address.
- Event ordering by ID is monotonic and deterministic.
- Processed events must not be re-applied.

## API Visibility Requirements

Relay account API `GET /v1/relay_account/:address/task_fee` MUST expose processed records only for `TaskPayment`, `TaskIncome`, and `TaskRefund` of the authenticated address.

Delegator-side `UserDelegation` income MUST be exposed through these APIs:

- `GET /v1/delegator/:user_address`
- `GET /v1/delegator/:user_address/delegation`
- `GET /v1/delegator/:user_address/delegations`
- `GET /v1/stats/line_chart/delegator/:address/earnings`
- `GET /v1/stats/line_chart/delegation/:user_address/:node_address/earnings`
- `GET /v1/client/:address/income/stats`
