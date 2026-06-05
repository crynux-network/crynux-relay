# Deposit and Withdraw Only Networks

This document defines how Relay supports additional deposit and withdrawal networks that do not run node contracts.

## Network Model

Relay has two configured network groups:

- Node networks.
- Deposit and withdraw only networks.

Node networks MUST support node contracts, native CNX deposits, native CNX withdrawals, and BenefitAddress binding. A node network MUST NOT be repeated in the deposit and withdraw only network config.

Deposit and withdraw only networks MUST support ERC20 CNX deposits, ERC20 CNX withdrawals, and BenefitAddress binding. They MUST NOT require node contracts other than BenefitAddress.

The effective deposit and withdraw network set is the union of node networks and deposit and withdraw only networks. Relay MUST reject configuration when the same network key exists in both groups.

## Token Rules

Node networks MUST use native CNX for deposit and withdrawal. Node network config MUST NOT include an ERC20 token address for relay account funding.

Deposit and withdraw only networks MUST use ERC20 CNX for deposit and withdrawal. Deposit and withdraw only network config MUST include the ERC20 token address.

All CNX amounts MUST use 18 decimals in Relay, Relay Wallet, and Portal.

## Configuration

Each node network config MUST include the existing node contract fields, including BenefitAddress. Each node network config MUST also include relay account funding fields:

- `rps`
- `start_block_num`
- `withdrawal_fee`
- `withdrawal_min`

Each deposit and withdraw only network config MUST include:

- `rps`
- `rpc_endpoint`
- `start_block_num`
- `chain_id`
- `contracts.benefit_address`
- `contracts.token_address`
- `log_block_range`
- `withdrawal_fee`
- `withdrawal_min`

`rps` limits the number of RPC requests Relay sends to the network per second. Relay MUST apply the network `rps` limiter to log queries, receipt reads, transaction reads, block reads, contract calls, and transaction submission for that network.

`log_block_range` limits the maximum number of blocks in one `eth_getLogs` request for deposit and withdraw only networks. Relay MUST split ERC20 deposit log scanning into requests whose inclusive block range length is less than or equal to `log_block_range`.

Base and other public EVM RPC providers enforce provider-specific `eth_getLogs` constraints. Relay config MUST set `log_block_range` to a value supported by the configured RPC provider. For Base providers, known limits include 5 blocks on QuickNode free trial, 10,000 blocks on QuickNode paid plans, 10 blocks on Alchemy free plans, and response-size-limited ranges on Alchemy paid plans. Relay MUST treat `log_block_range` as the authoritative runtime limit for the configured endpoint.

`relay_account.deposit_address` remains a single global deposit address. `withdraw.withdrawal_fee_address` remains a single global withdrawal fee address.

`withdrawal_fee` is network-specific and MUST be read from the effective deposit and withdraw network config selected by the withdrawal request.

## Network Processing Workers

Relay MUST run one processing worker per effective network. Relay MUST NOT create separate node and deposit workers for the same network.

For a node network, the worker MUST scan the chain once and process both:

- Node contract logs.
- Native CNX transfers to `relay_account.deposit_address`.

For a deposit and withdraw only network, the worker MUST NOT scan every transaction in every block. The worker MUST read ERC20 deposits with bounded `eth_getLogs` requests filtered by:

- ERC20 token contract address equal to `token_address`.
- `Transfer(address,address,uint256)` event topic.
- Indexed `to` address equal to `relay_account.deposit_address`.

Each `eth_getLogs` request MUST use a block range no larger than the network `log_block_range`. The worker MUST wait on the network `rps` limiter before each RPC request.

The worker MUST validate each detected deposit before crediting the relay account.

## Processing Cursor

Relay MUST persist one blockchain processing cursor per effective network in the database.

The `BlockchainCursor` model MUST use the `blockchain_cursors` table semantics:

- Each network has an independent cursor.
- If a database row exists for the network, Relay resumes from the stored cursor.
- If no database row exists for the network, Relay initializes the cursor from the network `start_block_num` in config.

Because Relay runs one worker per effective network, a single cursor per network is sufficient. Duplicate network keys across config groups MUST be rejected before workers start.

## Deposit Validation

For node networks, Relay MUST validate native CNX deposits with the existing native transfer rule:

- The transaction recipient MUST equal `relay_account.deposit_address`.
- The transaction value MUST be greater than zero.
- The transaction input MUST be empty.
- The receipt status MUST be successful.

For deposit and withdraw only networks, Relay MUST validate ERC20 CNX deposits with receipt data:

- The receipt status MUST be successful.
- The log address MUST equal `token_address`.
- The log MUST be a `Transfer(address,address,uint256)` event.
- The indexed `to` address MUST equal `relay_account.deposit_address`.
- The amount MUST be greater than zero.
- The credited relay account address MUST be the indexed `from` address.

Relay MUST enforce deposit idempotency by transaction hash and network.

Relay MUST create `Deposit` relay account events with reason format `3-{tx_hash}-{network}` for both native and ERC20 deposits.

## Withdrawal Validation

Relay MUST accept withdrawal requests for node networks and deposit and withdraw only networks. Relay MUST reject withdrawal requests for any other network.

Relay MUST resolve the withdrawal fee from the selected network. Relay MUST set the fee to zero when the requester address equals `dao.task_fee_share_address` or `withdraw.withdrawal_fee_address`.

Relay MUST validate the withdrawal destination against the BenefitAddress contract on the selected withdrawal network:

- If the requester has a non-zero benefit address on that network, the withdrawal destination MUST equal that benefit address.
- If the requester has no benefit address on that network, the withdrawal destination MUST equal the requester operational wallet address.

BenefitAddress state is network-specific. Relay MUST NOT use a benefit address from a different network to validate a withdrawal.

## Relay Wallet Requirements

Relay Wallet MUST use the same effective funding network shape as Relay.

Relay Wallet native network entries MUST omit `token_address` and execute withdrawals with native transfers.

Relay Wallet ERC20 network entries MUST include `token_address` and `contracts.benefit_address`, and execute withdrawals with ERC20 `transfer(destination, amount)`.

For ERC20 withdrawals, Relay Wallet MUST check ERC20 CNX balance for payout sufficiency and native gas balance for transaction execution.

Relay Wallet MUST validate deposit events with the same native and ERC20 rules defined in this document before applying Relay event logs.
