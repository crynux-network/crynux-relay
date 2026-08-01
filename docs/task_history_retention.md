# Task and Event History Retention

This document specifies automatic retention cleanup for finished inference task rows, node task error reports, on-disk task artifacts, and the Relay `events` stream.

## Configuration

`task.history_retention_days` controls both cleanup paths.

| Value | Behavior |
|-------|----------|
| `0` | Cleanup MUST NOT run. |
| `N` where `N > 0` | Relay MUST delete eligible history older than `N` days. |

Relay MUST evaluate retention on an hourly background tick. Each enabled tick MUST run task history cleanup, then event history cleanup, using the same UTC cutoff `now - N days`.

## Task History Cleanup

Relay MUST delete an `inference_tasks` row only when all of the following hold:

1. `status` is one of `TaskEndInvalidated`, `TaskEndSuccess`, `TaskEndAborted`, `TaskEndGroupRefund`, `TaskEndGroupSuccess`.
2. `updated_at` is older than the retention cutoff.
3. No `pending_slashes` row with `status = pending` references the same `task_id_commitment`.
4. No pending `relay_account_events` of task payment, task income, DAO task share, task refund, or user delegation type parses to the same `task_id_commitment`.

For each deleted task commitment, Relay MUST also delete matching `node_task_errors` rows in the same database transaction, then best-effort remove:

- `data_dir.inference_tasks/<task_id_commitment>/`
- `data_dir.slashed_tasks/<task_id_commitment>/`

Task history cleanup MUST NOT delete `events` rows.

## Event History Cleanup

Event history cleanup is independent of task history cleanup.

Relay MUST hard-delete every `events` row whose `created_at` is older than the retention cutoff, including task events and non-task events such as `NodeJoin`, `NodeQuit`, staking, delegation, `DownloadModel`, `NodeSlashed`, and `NodeKickedOut`.

Event history cleanup MUST NOT delete `inference_tasks` rows.

Relay business state MUST NOT depend on replaying deleted `events` rows. Authoritative state remains in business tables as specified in [relay_event_stream.md](./relay_event_stream.md).

## Retained Data

Retention cleanup MUST NOT delete:

- `relay_account_events`
- `relay_accounts`
- earnings and incentive tables
- pre-aggregated `task_*_counts` and related stats tables

## Operational Effects

After purge, Admin task history, task trace, and node task error queries MUST return empty or not-found results for deleted commitments. Node event watchers that initialize from `GET /v1/events/current_id` MUST continue from the latest remaining cursor and MUST NOT require replay of deleted events.
