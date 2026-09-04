# Task Version Matching

This document explains how the relay uses task version to select compatible nodes and how runner version is handled.

For the full node selection pipeline that applies these version filters, see [node_selection.md](./node_selection.md).

At a high level, task version defines the minimum compatible runtime for execution. The relay validates version formats at task creation and node join or update, then selects nodes using a strict compatibility gate where `major` must match exactly and node `minor.patch` must be greater than or equal to task `minor.patch`. The same compatibility check is performed again right before moving a task to `Started` to prevent race-condition mismatches. The `node version` stored by relay SHALL be the runner version reported by the node. Runner version is also tracked separately for worker count APIs and does not affect task dispatch through those APIs.

## Scope

- Task version matching is implemented for node selection and task start validation.
- The `node version` recorded by relay SHALL represent the node runner version.
- Runner version is tracked as runtime count data and is not used in task dispatch through worker count APIs.

## Version Format

Both task version and node version use semantic version style with three numeric parts.

- Task version format: `major.minor.patch`
- Node version format: `major.minor.patch`

In relay, the `version` field sent by node join, joined-node capability synchronization, and node version update APIs SHALL be interpreted as the current runner version of that node. It MUST NOT be interpreted as the version of the node manager, web UI, packaging layer, or any other component.

Validation entry points:

- Task creation validates `task_version` in `api/v1/inference_tasks/create_task.go`
- Node join validates `version` in `api/v2/nodes/join.go`
- Joined-node capability synchronization validates `version` in `api/v2/nodes/capabilities.go`
- Node version update validates `version` in `api/v1/nodes/version.go`

## Data Model

- Task version is stored as string in `models.InferenceTask.TaskVersion`
- Node version is stored as numeric fields in `models.Node`
  - `MajorVersion`
  - `MinorVersion`
  - `PatchVersion`

The node-side source of these relay fields SHALL be the runner version reported by the node process at join time and at every later version change.

The task version parser is `InferenceTask.VersionNumbers` in `models/inference_task.go`.

Relay persistence requirements:

- `models.Node.MajorVersion`, `models.Node.MinorVersion`, and `models.Node.PatchVersion` SHALL store the runner version currently reported by the node
- Relay MUST use these fields as the only node-side version input for task compatibility matching
- Relay MUST NOT infer task compatibility from worker count records or any separate node application version

## Node Compatibility Rule

The compatibility rule used by relay is:

- Node major version must equal task major version
- Node minor and patch must be greater than or equal to task minor and patch

Equivalent comparison:

- `node.major == task.major`
- `node.minor > task.minor`
- or `node.minor == task.minor` and `node.patch >= task.patch`

This means matching is strict on major version and forward compatible on minor and patch.

Because relay stores runner version in `models.Node`, this rule is a compatibility rule between task version and runner version.

## Where Matching Happens

### Step 1: Candidate Filtering During Node Selection

When dispatcher selects a node for a queued task, the filtering happens in:

- `service/select_nodes.go`
  - `filterNodesByGPU`
  - `filterNodesByVram`

Database filter conditions include:

- `status = available`
- hardware constraints
- `major_version = task.major`
- `minor_version > task.minor OR minor_version = task.minor AND patch_version >= task.patch`

Selection call chain:

- `service/start_task.go` -> `TaskDispatcher.Dispatch`
- `service/select_nodes.go` -> `selectNodeForInferenceTask`
- The node version used here SHALL be the stored runner version

### Step 2: Safety Recheck Before Task Starts

Before final state change to `TaskStarted`, relay checks version again in:

- `service/task_status.go`
  - `isNodeVersionValidForTask`
  - used by `SetTaskStatusStarted`

If version is not compatible, task start fails with:

- `node version is not compatible with task`

This prevents race conditions where node version may have changed after initial selection.

## Runner Version Reporting and Update Propagation

In this codebase, the same runner version value reaches relay through two different reporting paths.

- The node version path updates `models.Node` and is used for task dispatch
- The worker count path updates `models.WorkerCount` and is used only for per-version worker count APIs

Reporting requirements:

- On local worker connection, node SHALL receive the runner version from the worker connection handshake
- When node joins relay, the `version` field in the join request SHALL be that runner version
- When a node process starts while its address remains joined, node SHALL report that runner version through joined-node capability synchronization
- When the local runner version changes later, node SHALL call the relay node version update API with the new runner version
- Relay SHALL persist that new runner version into `models.Node`
- The worker process SHALL also report the same runner version directly to relay worker count APIs
- Relay SHALL persist that direct worker report into `models.WorkerCount.WorkerVersion`

### Source Of WorkerVersion

`WorkerVersion` in relay SHALL come from the runner process itself. It is not created by relay, and it is not copied from node join requests.

The reporting chain SHALL be:

- The runner process determines its own current runner version
- The runner process reports that version directly to relay worker count APIs
- On startup, the runner process SHALL call `POST /v1/worker/{version}`
- On shutdown, the runner process SHALL call `DELETE /v1/worker/{version}`

Therefore:

- `models.Node` version comes from node join and node version update APIs
- `models.WorkerCount.WorkerVersion` comes from the worker process direct calls to relay worker APIs
- These two records can carry the same runner version string, but they are written through different API paths and stored for different purposes

Worker count records continue to exist for separate counting APIs.

- `api/v1/worker/worker.go` stores counts by exact `WorkerVersion` string
- `models/worker.go` persists `WorkerVersion` and `Count`

Current behavior:

- Worker version count data is only used for join, quit, and count query APIs
- No code path uses worker count records to filter nodes
- No code path uses task version to pick a worker count record
- No task dispatch logic depends on worker count records

The compatibility version used by relay task dispatch SHALL come from `models.Node`, and `models.Node` SHALL reflect the latest runner version reported by the node.

### Runner Auto Update

Runner auto update is performed on the node side by the runner process wrapper.

Behavior requirements:

- The runner process wrapper SHALL read the current runner version from the local runner package
- It SHALL poll the configured patch source once every 60 seconds
- It SHALL read `patches.txt` and collect only versions that are newer than the current local runner version
- It SHALL only apply patches whose `major` version matches the current local runner `major`
- After applying one or more patches, it SHALL restart the local runner process
- After the restarted runner reconnects, node SHALL observe the new runner version and SHALL report that new version to relay

This mechanism allows relay node selection to follow runner upgrades without requiring a separate relay-side migration step. Once the updated runner reconnects and the node reports the new version, all later task compatibility checks SHALL use the new runner version.

## Practical Outcome

Given a task version `A.B.C`:

- Relay will only consider nodes whose reported runner major version is `A`
- Among those nodes, relay accepts runner versions `A.B.C` and newer within major `A`
- Relay rejects any node whose reported runner major version is not equal to `A`
- Worker count records have no effect on this decision

## Related Source Files

- `models/inference_task.go`
- `models/node.go`
- `service/select_nodes.go`
- `service/start_task.go`
- `service/task_status.go`
- `api/v1/inference_tasks/create_task.go`
- `api/v2/nodes/join.go`
- `api/v2/nodes/capabilities.go`
- `api/v1/nodes/version.go`
- `api/v1/worker/worker.go`
- `models/worker.go`
