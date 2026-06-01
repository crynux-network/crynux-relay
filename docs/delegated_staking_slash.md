# Delegated Staking Slash Specification

This document specifies batched delegated staking slash processing in Relay.

## Scale Assumptions

A single node MAY have a very large number of delegators. A single delegator is not expected to have a very large number of delegations.

Relay and contracts MUST apply this pagination rule:

- Node-side delegator reads MUST be paginated or otherwise bounded.
- Available delegated-staking node reads MUST be paginated or otherwise bounded.
- User-side delegation reads do not require new contract pagination for this change.

## Contract Responsibilities

Contracts MUST NOT own delegated slash orchestration, job progress, retries, recovery, refund workflow, or node-level delegated slash lifecycle.

`NodeStaking` MUST handle only operator stake slash. `NodeStaking.slashStaking` MUST confiscate the operator stake, remove the node staking record, and emit `NodeSlashed`. It MUST NOT call `DelegatedStaking`.

`DelegatedStaking` MUST expose `slashNodeDelegations(address nodeAddress, address[] delegators)` for the Relay admin. The function MUST slash exactly the supplied delegator addresses for the node, remove their staking indexes, emit one `DelegatorSlashed` event per delegator, and transfer the batch total once to the slash receiver.

`DelegatedStaking.setDelegatorShare(0)` MUST only set the node share to `0` and remove the node from the available delegated-staking node set. It MUST NOT clear, refund, slash, or invalidate existing delegations.

## Relay Responsibilities

Relay MUST create or resume a durable delegated slash job after a confirmed `NodeStaking.NodeSlashed` event. The job is keyed by node address and network.

Relay MUST select delegator address batches from the `DelegatedStaking` contract, queue `DelegatedStaking::slashNodeDelegations` transactions, and keep the job open while the contract reports remaining delegators for the node.

Relay MUST process `DelegatedStaking.DelegatorSlashed` events as the source of truth for delegated slash progress. For each event, Relay MUST atomically invalidate the matching delegation row and write one delegated slash audit row.

Relay MUST update the delegation cache only after the database transaction that invalidates the delegation and writes the audit row succeeds.

Relay MUST resume unfinished delegated slash jobs on startup and periodically while running. Recovery MUST NOT queue a new batch when a pending or sent batch transaction already exists for the job.

## Audit Requirements

Relay MUST keep one durable `delegated_staking_slash_records` row per confirmed slashed delegator. This table is a database audit table, not the Relay `events` table.

Relay MUST also emit the Relay event type `DelegatedStakingSlashed` through the generic `events` table for downstream event consumers. The `DelegatedStakingSlashedEvent` Go type is only the payload used to build that generic event. It MUST NOT be used as the database model for the durable slash audit table.

Each `delegated_staking_slash_records` row MUST include:

- node address
- delegator address
- network
- amount
- slash transaction hash
- block number
- log index
- slash job ID when available

The `delegated_staking_slash_records` table MUST enforce uniqueness on `(network, slash_tx_hash, log_index)` and MUST support lookup by `(network, node_address, delegator_address)`.

The authenticated admin API `GET /v2/admin/delegated_slash/audits` MUST support filtering by node address and network, optional filtering by delegator address, and pagination with `page` and `page_size`. The default page size is `30`, and the maximum page size is `100`.

## Reliability Requirements

Delegated slash batch queuing MUST be idempotent for repeated `NodeSlashed` log processing and recovery runs.

Delegated slash audit writes MUST be idempotent for repeated receipt scans. A repeated `(network, slash_tx_hash, log_index)` MUST NOT fail the whole event processing transaction.

The delegated slash job MUST be marked completed only when the contract reports zero remaining delegators for the node.
