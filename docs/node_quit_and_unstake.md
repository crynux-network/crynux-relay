# Node Quit and Unstake Flow

This document specifies the node quit and unstake flow across the node, Relay, and the `NodeStaking` contract.

## Purpose

Node stake MUST be recoverable by the node owner even when Relay is unavailable or the node software cannot be controlled by the owner.

The normal quit path uses both the node and Relay:

1. The node owner starts the quit operation in the node.
2. The node calls `NodeStaking::tryUnstake` when it is still staked on chain.
3. After `tryUnstake` is confirmed, the node calls the Relay node quit API, or Relay observes the `NodeTryUnstaked` chain event.
4. Relay removes the node from local scheduling and sends a `NodeStaking::unstake` transaction on chain.

The on-chain recovery path uses only contract methods:

1. The node owner calls `NodeStaking::tryUnstake`.
2. If Relay is unavailable or does not complete Relay unstake, the node owner waits for `forceUnstakeDelay`.
3. The node owner calls `NodeStaking::forceUnstake` to recover the stake.

`forceUnstake` is a recovery mechanism. It MUST NOT be required during normal Relay operation.

## Contract Staking States

`NodeStaking` stores each node in one of these staking states:

| State | Value | Meaning |
|-------|-------|---------|
| `Unstaked` | `0` | The node has no active stake in the contract. |
| `Staked` | `1` | The node has active stake and may be used by Relay. |
| `PendingUnstaked` | `2` | The node owner has called `tryUnstake`, and the stake can be recovered through Relay unstake or delayed `forceUnstake`. |

`tryUnstake` MUST be callable by the node owner when the node is `Staked`. It MUST move the node to `PendingUnstaked`, record `unstakeTimestamp`, and emit `NodeTryUnstaked`.

`forceUnstake` MUST be callable by the node owner only when the node is `PendingUnstaked` and `unstakeTimestamp + forceUnstakeDelay` has passed. It MUST return the node stake and remove the node from the staking contract.

`unstake(address)` MUST be callable only by Relay. It MUST be able to unstake a node from either `Staked` or `PendingUnstaked`.

`slashStaking(address)` MUST be callable only by Relay. It MUST confiscate the node stake and remove the node from the staking contract.

## Normal Node-Initiated Quit

The node starts normal quit by calling `NodeStaking::tryUnstake` when it is still `Staked`. The node MUST wait for `tryUnstake` to be confirmed before it calls the Relay quit API.

After Relay receives the quit API request, Relay MUST remove the node from scheduling. If the node is executing a task, Relay MUST stop assigning new tasks and finish the quit after the current task resolves.

When Relay completes the quit, Relay MUST send `NodeStaking::unstake` on chain if the node is still `Staked` or `PendingUnstaked`. Relay MUST NOT require the node owner to wait for `forceUnstakeDelay` during normal operation.

## Quit Through Chain Event

The Relay quit API is not the only way for Relay to learn that the node owner wants to quit. `tryUnstake` emits `NodeTryUnstaked`, and Relay MUST treat that event as the same quit request.

If the node does not call the Relay quit API, Relay MUST still remove the node from scheduling after it observes `NodeTryUnstaked`. Relay MUST then send `NodeStaking::unstake` on chain when the node is ready to quit.

If both the Relay quit API and the `NodeTryUnstaked` event are processed, Relay MUST apply the quit only once. The second signal MUST be treated as already handled.

## On-Chain Recovery

If Relay is unavailable, the node owner can still recover the stake without Relay:

1. Call `NodeStaking::tryUnstake`.
2. Wait for `forceUnstakeDelay`.
3. Call `NodeStaking::forceUnstake`.

This recovery path is a fallback. When Relay is operating normally, Relay unstake MUST complete the fund return before `forceUnstake` is needed.

## Relay Kickout and Slashing

Relay kickout does not require the node owner to call `tryUnstake`. When Relay removes a node for a non-slashed reason, Relay MUST send `NodeStaking::unstake` on chain if the node still has active stake.

Slashing has priority over normal unstake. If Relay determines that a node must be slashed while the node is `Staked` or `PendingUnstaked`, Relay MUST send `NodeStaking::slashStaking` instead of `NodeStaking::unstake`.
