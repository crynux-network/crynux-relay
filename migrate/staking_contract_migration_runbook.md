# Staking Contract Migration Runbook

This runbook applies to every node blockchain network configured under Relay `blockchains` at migration startup. Networks configured only under `deposit_withdraw_networks` are outside the staking migration scope.

## Migration values

Record and verify one row for each key under `blockchains` before starting:

| Blockchain network key | Old BenefitAddress | Old NodeStaking | Old DelegatedStaking | New BenefitAddress | New NodeStaking | New DelegatedStaking | Deployment block `B` | `start_block_num` equal to `B - 1` |
|---|---|---|---|---|---|---|---:|---:|
| `<NETWORK_KEY>` | `<OLD_BENEFIT_ADDRESS>` | `<OLD_NODE_STAKING_ADDRESS>` | `<OLD_DELEGATED_STAKING_ADDRESS>` | `<NEW_BENEFIT_ADDRESS>` | `<NEW_NODE_STAKING_ADDRESS>` | `<NEW_DELEGATED_STAKING_ADDRESS>` | `<B>` | `<B_MINUS_1>` |

Record the database backup identifier separately.

Do not start the migration while any placeholder remains unresolved.

Governance contract addresses MUST NOT be added to Relay `config.yml`. Relay contract configuration contains only BenefitAddress, NodeStaking, and DelegatedStaking for this migration.

## 1. Verify the new contracts

Complete the new contract deployment and configuration before changing any running service.

1. Verify the three new addresses on each configured node blockchain network.
2. Verify that each address contains deployed bytecode.
3. Verify the NodeStaking BenefitAddress binding.
4. Verify the immutable slash receiver on NodeStaking and DelegatedStaking.
5. Configure the NodeStaking and DelegatedStaking observers.
6. Configure the Relay admin or operator address.
7. Complete the required owner or governance handoff.
8. Record the deployment block `B` and calculate `B - 1` independently for each configured node blockchain network.

Pass condition: every configured address and role matches the approved deployment record, and `B - 1` is recorded.

Stop condition: any address has no bytecode, any binding or role is incorrect, either observer is unset, or the deployment block is not known.

## 2. Prepare and deploy Portal

1. Set `system_networks.<NETWORK_KEY>.contracts` to the three new contract addresses for every migrated blockchain network.
2. Set `system_networks.<NETWORK_KEY>.legacyStakingContracts` to the corresponding three old contract addresses and the required legacy ABI profile for every migrated blockchain network.

3. Build and deploy Portal.
4. Log in with a wallet that has old staking state.
5. Verify that the desktop wallet menu and mobile menu show `Legacy Staking`.
6. Verify that the page reads the old NodeStaking and DelegatedStaking state.
7. Verify that the normal staking pages use the new contract addresses.

Pass condition: the deployed Portal reads old funds only through `legacyStakingContracts`, and all normal staking actions use `contracts`.

Stop condition: the legacy page uses Relay delegation data, a legacy transaction targets a new contract, or a normal staking transaction targets an old contract.

## 3. Stop new heartbeat task creation

Disable the production component that creates heartbeat tasks. Keep Relay running so existing tasks and Relay-created blockchain transactions can finish.

Pass condition: no new heartbeat task is created after the disable time.

Stop condition: heartbeat tasks continue to be created.

## 4. Drain existing tasks

Wait for every existing task to reach a terminal status through the normal Relay flow. Do not modify task rows.

Run:

```sql
SELECT COUNT(*) AS non_terminal_tasks
FROM inference_tasks
WHERE status < 7;

SELECT COUNT(*) AS nodes_with_task_commitments
FROM nodes
WHERE network IN (<BLOCKCHAINS_NETWORK_KEYS>)
  AND current_task_id_commitment IS NOT NULL
  AND current_task_id_commitment <> '';
```

Pass condition: both queries return `0`.

Stop condition: either query returns a nonzero value. Do not clear a task commitment manually.

## 5. Drain Relay-created old staking transactions

Run:

```sql
SELECT id, type, status, tx_hash, to_address
FROM blockchain_transactions
WHERE (network, to_address) IN (
    (<NETWORK_KEY>, <OLD_NODE_STAKING_ADDRESS>),
    (<NETWORK_KEY>, <OLD_DELEGATED_STAKING_ADDRESS>)
  )
  AND status IN (0, 1);
```

Status `0` is Pending, status `1` is Sent, status `2` is Confirmed, and status `3` is Failed.

Pass condition: the query returns no rows.

Stop condition: any Relay-created old staking transaction remains Pending or Sent.

Transactions submitted directly by user or node wallets are not stored in this table and do not block the migration.

## 6. Stop old Relay and create the database backup

1. Stop every old Relay process.
2. Verify that no Relay process can write to the production database.
3. Create a complete database backup.
4. Restore the backup into an isolated database and verify that the restored database opens successfully.
5. Record the backup identifier.

Capture the following pre-migration counts:

```sql
SELECT 'inference_tasks' AS table_name, COUNT(*) AS row_count FROM inference_tasks
UNION ALL SELECT 'blockchain_transactions', COUNT(*) FROM blockchain_transactions
UNION ALL SELECT 'relay_accounts', COUNT(*) FROM relay_accounts
UNION ALL SELECT 'task_fees', COUNT(*) FROM task_fees
UNION ALL SELECT 'user_staking_earnings', COUNT(*) FROM user_staking_earnings
UNION ALL SELECT 'node_earnings', COUNT(*) FROM node_earnings
UNION ALL SELECT 'user_earnings', COUNT(*) FROM user_earnings
UNION ALL SELECT 'vesting_records', COUNT(*) FROM vesting_records
UNION ALL SELECT 'events', COUNT(*) FROM events
UNION ALL SELECT 'network_node_data', COUNT(*) FROM network_node_data
UNION ALL SELECT 'node_stakings', COUNT(*) FROM node_stakings;

SELECT network, last_block_num
FROM blockchain_cursors
WHERE network NOT IN (<BLOCKCHAINS_NETWORK_KEYS>)
ORDER BY network;
```

Pass condition: Relay is stopped, the restore test succeeds, the backup identifier is recorded, all pre-migration counts are saved, and every cursor outside the migration target list is recorded.

Stop condition: Relay is still writing, backup creation fails, or the restore test fails.

## 7. Update Relay configuration

For every `blockchains.<NETWORK_KEY>` entry:

1. Set `contracts.benefit_address` to the new BenefitAddress.
2. Set `contracts.node_staking` to the new NodeStaking.
3. Set `contracts.delegated_staking` to the new DelegatedStaking.
4. Set `start_block_num` to `B - 1`.
5. Keep the network key, chain ID, RPC endpoint, and Relay signer account unchanged.

The `deposit_withdraw_networks` entries and their cursors MUST remain unchanged.

Pass condition: a configuration review confirms all three new addresses and the exact per-network `B - 1` value for every key under `blockchains`.

Stop condition: an old staking address remains in the active Relay contract configuration, or `start_block_num` is not `B - 1`.

## 8. Update Node release configuration

Set the three active contract addresses in every Node release template for each migrated blockchain network:

- `build/docker/config.yml.base`
- `build/macos/config.yml.base`
- `build/windows/config.yml.base`
- the corresponding generated `config.yml.example` files

Old contract addresses must not remain in Node release configuration.

Pass condition: all Node release variants contain the same three new addresses.

Stop condition: any release variant contains an old address or differs from the other variants.

## 9. Start new Relay and apply migration

Start exactly one new Relay instance. Startup applies migration `M20260812` after database initialization and before blockchain clients, transaction processing, task processing, background tasks, and HTTP serving.

The migration:

- obtains its target blockchain network list from the sorted keys under Relay `blockchains`;
- fails when the target list is empty;
- fails when any current node belongs to a blockchain network outside the target list;
- fails when any current node has a nonempty task commitment;
- deletes blockchain-scoped current staking state only for the target blockchain networks;
- preserves blockchain-scoped historical data for every blockchain network outside the target list;
- deletes all node model state, model download selections, and node name counts only after node validation succeeds;
- deletes blockchain cursors only for the target blockchain networks;
- preserves every `deposit_withdraw_networks` cursor that is not also a key under `blockchains`;
- does not read or modify `inference_tasks`;
- does not read or modify `blockchain_transactions`;
- does not create any blockchain transaction;
- executes the reset in one database transaction.

Pass condition: the log contains `DB migrations are done!`, Relay continues startup, and no migration error is logged.

Stop condition: migration or startup fails. Keep Relay stopped. Do not use migration rollback as data recovery; `M20260812` rollback does not recreate deleted current state. Restore the complete pre-migration database backup and old configuration before restarting the old release.

## 10. Verify database state

Run:

```sql
SELECT COUNT(*) AS nodes FROM nodes WHERE network IN (<BLOCKCHAINS_NETWORK_KEYS>);
SELECT COUNT(*) AS delegations FROM delegations WHERE network IN (<BLOCKCHAINS_NETWORK_KEYS>);
SELECT COUNT(*) AS delegated_slash_jobs FROM delegated_slash_jobs WHERE network IN (<BLOCKCHAINS_NETWORK_KEYS>);
SELECT COUNT(*) AS delegated_slash_records FROM delegated_staking_slash_records WHERE network IN (<BLOCKCHAINS_NETWORK_KEYS>);
SELECT COUNT(*) AS node_snapshots FROM delegated_staking_node_list_snapshots WHERE network IN (<BLOCKCHAINS_NETWORK_KEYS>);
SELECT COUNT(*) AS leaderboard_snapshots FROM delegation_task_fee_leaderboard_snapshots WHERE network IN (<BLOCKCHAINS_NETWORK_KEYS>);
SELECT COUNT(*) AS cursors FROM blockchain_cursors WHERE network IN (<BLOCKCHAINS_NETWORK_KEYS>);
SELECT COUNT(*) AS node_models FROM node_models;
SELECT COUNT(*) AS model_download_selections FROM node_model_download_selections;
SELECT COUNT(*) AS node_name_counts FROM node_name_counts;
```

Every query must return `0` before nodes rejoin.

Repeat the preserved-table count query from step 6 and compare every result with the saved pre-migration count.

Repeat the non-target cursor query from step 6 and compare every row with the saved pre-migration result.

Pass condition: all reset tables return `0`, all preserved-table counts are unchanged, every cursor outside the target list is unchanged, and the Relay background refresh recreates snapshots only from new current state.

Stop condition: a reset table contains old current state, a preserved-table count changed, or any cursor outside the target list changed.

## 11. Verify blockchain scanning

Verify that every recreated target blockchain network cursor starts from that network's configured block `B - 1` and that its first scanned block is `B`.

Pass condition: Relay processes each new contract set beginning at that network's block `B` without reading old-contract events.

Stop condition: scanning starts after `B`, scans an old contract, or reports an event decoding error.

## 12. Release Node configuration and reopen staking

1. Publish the Node release containing the new active addresses.
2. Allow nodes to stake and join through the existing Node flow.
3. Verify that the first joined node is created as new Relay current state.
4. Verify that its delegation state is rebuilt from the new DelegatedStaking contract.
5. Allow delegators to stake on the new contract.
6. Keep the Portal Legacy Staking page available for withdrawal from the old contracts.

Pass condition: nodes and delegators create state only on the new contracts, while old withdrawals target only the configured legacy contracts.

Stop condition: any new stake or join targets an old contract, or Relay reconstructs current state from an old contract.
