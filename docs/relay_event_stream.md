# Relay Event Stream

This document specifies the Relay `events` table, the v1 event APIs, event producers, and node polling behavior.

## Scope

The `events` table is an append-only Relay event stream. Relay uses it to persist task, node lifecycle, staking, delegation, delegated slash, and model download events addressable by node address or task ID commitment.

The `events` table is distinct from `relay_account_events`. `relay_account_events` is the Relay account ledger and MUST NOT be used as the node event stream.

Relay business state MUST NOT depend on replaying `events` rows. Each producer MUST update its authoritative business tables directly. The event stream provides node delivery, external polling, and operational traceability.

## Storage Model

Relay SHALL store each event as one `models.Event` row:

| Field | Requirement |
|-------|-------------|
| `id` | Auto-increment event cursor. Consumers MUST use it as the only ordering cursor. |
| `type` | Event type defined in `models/event.go`. |
| `node_address` | Node address associated with the event. |
| `task_id_commitment` | Task ID commitment for task events; otherwise the event-defined empty value or placeholder. |
| `args` | JSON matching the corresponding payload structure in `models/event.go`. |

Relay MUST write event rows only through `service.emitEvent`. Each payload type MUST implement `models.ToEventType` and convert itself through `ToEvent`.

## API

### `GET /v1/events/current_id`

This endpoint MUST return the latest event ID visible under the optional `event_type`, `node_address`, and `task_id_commitment` filters. It MUST return `0` when no row matches. A node MUST set `node_address` to its own address during watcher initialization.

### `GET /v1/events`

This endpoint MUST return events with `id > start` in ascending `id` order. It SHALL accept:

| Parameter | Requirement |
|-----------|-------------|
| `start` | Required event cursor. |
| `event_type` | Optional event type filter. |
| `node_address` | Optional node address filter. Nodes MUST set it to their own address. |
| `task_id_commitment` | Optional task ID commitment filter. |
| `limit` | Optional maximum row count. |

## Crynux Node Watcher

A node MUST initialize its local cursor with `GET /v1/events/current_id?node_address=<node-address>`, then poll `GET /v1/events?start=<last-event-id>&node_address=<node-address>`.

For each response, the node MUST parse rows by `type` and `args`, dispatch only events with registered handlers, ignore parsed types without handlers, and advance its local cursor to the highest returned event ID.

The node SHALL handle these event types:

| Event type | Node behavior |
|------------|---------------|
| `TaskStarted` | Create an inference task in the local task system. |
| `DownloadModel` | Create a model download task in the local task system. |
| `NodeSlashed` | Set local node state to `slashed`. |
| `NodeKickedOut` | Set local node state to `kicked_out`. |

## DownloadModel Production

Relay MUST produce `DownloadModel` events only from the model distribution controller specified in [model_distribution.md](./model_distribution.md). Neither task matching nor task start produces these events.

Each `DownloadModel` event MUST target one selected qualified node and contain one normalized `base:<name>` model ID that is absent from that node's authoritative on-disk model set. Relay MUST NOT emit `DownloadModel` for non-base IDs.

Each selection attempt emits exactly one event carrying the selected base model ID, committed in the same transaction as the persisted selection record. Relay MUST NOT periodically resend events for a pending selection. A new event for the same node and model is produced only by a new selection attempt after the previous selection expired.

The AddModelID node report MUST remain authoritative for the corresponding on-disk `node_models` row; selection completion is derived from the on-disk inventory as specified in [model_distribution.md](./model_distribution.md).

## Event Producers

| Producer path | Event types |
|---------------|-------------|
| Node lifecycle service | `NodeJoin`, `NodeQuit`, `NodeKickedOut`, `NodeSlashed` |
| Model distribution controller | `DownloadModel` |
| Task status service | `TaskStarted`, `TaskScoreReady`, `TaskErrorReported`, `TaskValidated`, `TaskEndInvalidated`, `TaskEndGroupRefund`, `TaskEndAborted`, `TaskEndSuccess`, `TaskEndGroupSuccess` |
| Node staking chain processor | `NodeStaking` |
| Delegated staking chain processor | `DelegatorStaking`, `DelegatorUnstaking`, `NodeDelegatorShareChanged`, `DelegatedStakingSlashed` |

`NodeSlashed` MUST be emitted when Relay executes the node slash service flow. `DelegatedStakingSlashed` MUST be emitted once for each confirmed `DelegatedStaking.DelegatorSlashed` chain event that marks a delegation row slashed.

## Runtime Consumers

The node has confirmed runtime handlers for `TaskStarted`, `DownloadModel`, `NodeSlashed`, and `NodeKickedOut`. Other event types are persisted for polling and traceability but have no confirmed runtime handler in Relay, node, Portal, or Relay Wallet.

Relay APIs `GET /v1/events/current_id` and `GET /v1/events` are the runtime readers of the `events` table. Relay business services MUST NOT read event rows to reconstruct state.

Crynux Portal MUST use dedicated Relay APIs rather than this stream. Crynux Relay Wallet MUST use `/v1/relay_account/event_logs` for account ledger synchronization and `/v1/withdraw/*` for withdrawal processing.

The `/v1/stats/node_events` endpoint MUST NOT read the `events` table. It builds node selection and release logs from inference tasks and network node data.
