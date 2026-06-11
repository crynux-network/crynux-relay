# Relay Event Stream

This document specifies the Relay `events` table, the v1 event APIs, the event producers, and the currently confirmed runtime consumers.

## Scope

The `events` table is an append-only Relay event stream. Relay uses this table to persist task, node lifecycle, staking, delegation, and delegated slash events that are addressable by node address or task ID commitment. The stream records Relay-side state changes for external polling and operational troubleshooting.

The `events` table is distinct from `relay_account_events`. `events` records task, node, staking, and delegation event stream rows. `relay_account_events` is the Relay account ledger and MUST NOT be used as this event stream.

Relay business state MUST NOT depend on replaying rows from `events`. Relay task, node, staking, delegation, delegated slash, statistics, and account flows update their own tables directly in their service paths. The `events` table is written as part of those paths and is read by the v1 event APIs.

## Storage Model

Relay stores each event as one `models.Event` row with these fields:

| Field | Requirement |
|-------|-------------|
| `id` | Auto-increment event cursor. Consumers MUST treat it as the only ordering cursor. |
| `type` | Event type string. Producers MUST set it to one of the Relay event types defined in `models/event.go`. |
| `node_address` | Node address associated with the event. Consumers MAY use this field to read an address-scoped stream. |
| `task_id_commitment` | Task ID commitment associated with the event when the event belongs to a task. Events without a task use an empty value unless the event type defines a placeholder. |
| `args` | JSON string containing the event-specific payload. The payload MUST match the corresponding event type structure in `models/event.go`. |

Relay MUST write `events` rows only through `service.emitEvent`. Each event payload type MUST implement `models.ToEventType` and convert itself into a `models.Event` row through `ToEvent`.

## API

Relay exposes the event stream through the v1 event API.

### `GET /v1/events/current_id`

This endpoint returns the latest event ID visible under the submitted filters.

Supported query parameters:

| Parameter | Requirement |
|-----------|-------------|
| `event_type` | Optional event type filter. |
| `node_address` | Optional node address filter. Nodes MUST set this to their own node address during watcher initialization. |
| `task_id_commitment` | Optional task ID commitment filter. |

If no row matches the filters, Relay MUST return `0`.

### `GET /v1/events`

This endpoint returns events after a cursor.

Supported query parameters:

| Parameter | Requirement |
|-----------|-------------|
| `start` | Required cursor. Relay MUST return rows with `id > start`. |
| `event_type` | Optional event type filter. |
| `node_address` | Optional node address filter. Nodes MUST set this to their own node address. |
| `task_id_commitment` | Optional task ID commitment filter. |
| `limit` | Optional maximum number of rows. |

Relay MUST return matching events ordered by ascending `id`.

## Crynux Node Watcher

A node MUST initialize its local event cursor by calling `GET /v1/events/current_id?node_address=<node-address>`. This prevents the node from replaying historical events that existed before its watcher started.

After initialization, the node MUST poll `GET /v1/events?start=<last-event-id>&node_address=<node-address>`. For each response:

1. The node MUST parse each returned row using the row's `type` and JSON `args`.
2. The node MUST dispatch parsed events only to handlers registered for that event type.
3. The node MUST ignore parsed event types that have no registered handler.
4. The node MUST update its local cursor to the highest returned event ID after the batch is fetched.

The node watcher currently registers handlers for these event types:

| Event type | Node behavior |
|------------|---------------|
| `TaskStarted` | Creates an inference task in the local task system. |
| `DownloadModel` | Creates a model download task in the local task system. |
| `NodeSlashed` | Sets local node state to `slashed`. |
| `NodeKickedOut` | Sets local node state to `kicked_out`. |

Other event types may still be returned by node-address polling. The current node implementation parses all modeled Relay event types that can be scoped to its node address and ignores parsed event types that have no registered handler.

## Event Producers

Relay writes `events` rows from these producer paths:

| Producer path | Event types |
|---------------|-------------|
| Node lifecycle service | `NodeJoin`, `NodeQuit`, `NodeKickedOut`, `NodeSlashed` |
| Task status service | `TaskStarted`, `DownloadModel`, `TaskScoreReady`, `TaskErrorReported`, `TaskValidated`, `TaskEndInvalidated`, `TaskEndGroupRefund`, `TaskEndAborted`, `TaskEndSuccess`, `TaskEndGroupSuccess` |
| Node staking chain processor | `NodeStaking` |
| Delegated staking chain processor | `DelegatorStaking`, `DelegatorUnstaking`, `NodeDelegatorShareChanged`, `DelegatedStakingSlashed` |

The `NodeSlashed` Relay event is emitted when Relay executes the node slash service flow. The `DelegatedStakingSlashed` Relay event is emitted once for each confirmed `DelegatedStaking.DelegatorSlashed` chain event that marks a delegation row `slashed = true`.

## Event Runtime Use

Relay writes all event types listed above, but only a subset has a confirmed runtime consumer.

| Event type | Confirmed runtime use |
|------------|-----------------------|
| `TaskStarted` | Crynux node handles this event and creates a local inference task. |
| `DownloadModel` | Crynux node handles this event and creates a local model download task. |
| `NodeSlashed` | Crynux node handles this event and sets local node state to `slashed`. |
| `NodeKickedOut` | Crynux node handles this event and sets local node state to `kicked_out`. |
| `TaskScoreReady` | Written to `events`; no confirmed runtime handler in Relay, node, Portal, or Relay Wallet. |
| `TaskErrorReported` | Written to `events`; no confirmed runtime handler in Relay, node, Portal, or Relay Wallet. |
| `TaskValidated` | Written to `events`; no confirmed runtime handler in Relay, node, Portal, or Relay Wallet. |
| `TaskEndInvalidated` | Written to `events`; no confirmed runtime handler in Relay, node, Portal, or Relay Wallet. |
| `TaskEndGroupRefund` | Written to `events`; no confirmed runtime handler in Relay, node, Portal, or Relay Wallet. |
| `TaskEndAborted` | Written to `events`; no confirmed runtime handler in Relay, node, Portal, or Relay Wallet. |
| `TaskEndSuccess` | Written to `events`; no confirmed runtime handler in Relay, node, Portal, or Relay Wallet. |
| `TaskEndGroupSuccess` | Written to `events`; no confirmed runtime handler in Relay, node, Portal, or Relay Wallet. |
| `NodeJoin` | Written to `events`; no confirmed runtime handler in Relay, node, Portal, or Relay Wallet. |
| `NodeQuit` | Written to `events`; no confirmed runtime handler in Relay, node, Portal, or Relay Wallet. |
| `NodeStaking` | Written to `events`; no confirmed runtime handler in Relay, node, Portal, or Relay Wallet. |
| `DelegatorStaking` | Written to `events`; no confirmed runtime handler in Relay, node, Portal, or Relay Wallet. |
| `DelegatorUnstaking` | Written to `events`; no confirmed runtime handler in Relay, node, Portal, or Relay Wallet. |
| `NodeDelegatorShareChanged` | Written to `events`; no confirmed runtime handler in Relay, node, Portal, or Relay Wallet. |
| `DelegatedStakingSlashed` | Written to `events`; no confirmed runtime handler in Relay, node, Portal, or Relay Wallet. |

## Current Consumers

The `events` table is read by these Relay APIs:

| API | Use |
|-----|-----|
| `GET /v1/events/current_id` | Returns the latest cursor for a filtered event stream. |
| `GET /v1/events` | Returns event rows after a cursor for API consumers. |

Crynux node consumes both APIs through its Relay client and `EventWatcher`. The node uses only the `node_address` filter for its main event polling loop.

Crynux Portal does not call `/v1/events` or `/v1/events/current_id`. Portal uses dedicated Relay APIs for relay accounts, staking, delegation, network statistics, incentives, vesting, and delegated staking.

Crynux Relay Wallet does not call `/v1/events` or `/v1/events/current_id`. Relay Wallet consumes `/v1/relay_account/event_logs` for account ledger synchronization and `/v1/withdraw/*` APIs for withdrawal processing.

Relay does not read `events` rows in its own business flows. Relay updates task, node, staking, delegation, delegated slash, statistics, and account state directly from the corresponding service, chain processor, and account ledger paths.

Relay tests use `models.Event` to verify blockchain processor event emission and skipped-event behavior. This test usage MUST NOT be treated as a runtime consumer.

The `/v1/stats/node_events` endpoint does not read the `events` table. It builds node selection and release logs from inference tasks and network node data.
