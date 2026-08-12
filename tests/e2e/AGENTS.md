## Build the Docker image for E2E testing

- Goal: Build a local relay image for e2e execution.

- Workflow:
  1. Ensure the current working directory is the repository root.
  2. Build the image from `build/crynux_relay.Dockerfile` with tag `crynux_relay:e2e`.
  3. Verify `crynux_relay:e2e` exists locally.

## Prepare the mount folder for the Docker image

- Goal: Prepare relay host-side files in a Docker mount workspace for local e2e runs.

- Workspace rule:
  - Choose one mount workspace root directory and use it consistently in volume mappings.
  - Reuse existing files in that workspace when possible.

- Required relay structure:

```text
<mount-root>/
  config/
    config.yml
    secrets/
  data/
    logs/
```

- Config preparation:
  - Copy `tests/e2e/config.e2e.yml` to `<mount-root>/config/config.yml`.
  - Keep all runtime files under the same `<mount-root>`.

- Directory creation rule:
  - Do not pre-create task data directories.
  - The application creates task directories under `data/inference_tasks` and `data/slashed_tasks` when needed.

## Prepare the database

- Goal: Prepare one database instance for relay e2e execution.

- Requirement:
  - `<mount-root>/config/config.yml` must use a valid `db.connection` that points to the prepared database.

## Prepare private keys

- Goal: Prepare all key material required by relay e2e execution.

- Required secret files:

```text
<mount-root>/config/secrets/blockchain_system_private_key.txt
<mount-root>/config/secrets/jwt_secret_key.txt
<mount-root>/config/secrets/mac_secret_key.txt
```

- Secret file requirements:
  - Each file must contain a single line.
  - `blockchain_system_private_key.txt` must contain a private key with the `0x` prefix.
  - `jwt_secret_key.txt` and `mac_secret_key.txt` are random strings.
  - No trailing whitespace is allowed.


## Prepare accounts

- Required account fields in `<mount-root>/config/config.yml`:
  - `blockchains.dymension.account.address`: signer address for dymension operations. It must match `blockchain_system_private_key.txt`.
  - `blockchains.near.account.address`: signer address for near operations. It must match `blockchain_system_private_key.txt`.
  - `withdraw.relay_wallet_address`: authorization address for Relay Wallet calls to Relay Wallet APIs on Relay. It must equal the address derived from the key `relay.api.private_key_file` in the Relay Wallet's config file.
  - `withdraw.withdrawal_fee_address`: relay operator fee income address for withdrawal fees.
  - `dao.task_fee_share_address`: task fee share recipient address and one of the withdrawal fee waiver addresses.
  - `relay_account.deposit_address`: on-chain deposit target address for relay account top-ups.
