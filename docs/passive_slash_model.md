# Passive Slash Mode

This document specifies passive slash mode in Crynux Relay.

## Configuration

Relay MUST read `task.passive_slash_mode` from the runtime configuration file. The key MUST be present. Relay MUST fail startup configuration initialization when the key is missing.

When `task.passive_slash_mode` is `true`, validation invalidation records a pending slash review and MUST NOT execute an automatic on-chain slash. When `task.passive_slash_mode` is `false`, validation invalidation executes the normal automatic slash flow.

The repository configuration templates MUST set `task.passive_slash_mode: true`.

## Runtime Evidence Snapshot

Relay MUST capture an in-memory runtime node snapshot after each task is assigned to a node. The snapshot MUST be keyed by `task_id_commitment`.

The runtime snapshot MUST include node fields that can change after assignment:

- Node address and node current blockchain network.
- GPU name and GPU VRAM.
- Node version.
- Operator stake amount.
- QoS score, health base, and health update time.
- Active model IDs and model in-use state.
- Delegated staking summary fields: delegator count and delegated staking amount.

The runtime snapshot MUST NOT store delegator address lists.

Relay MUST delete each task's runtime snapshot after that task reaches a terminal state. For `TaskEndInvalidated`, Relay MUST build and persist slash evidence before deleting the invalidated task's snapshot.

If the runtime snapshot is missing, Relay MUST still persist evidence with `evidence_complete = false`. The evidence MUST include `incomplete_reason` and MAY include current database task and node context to identify the invalidation.

## Slash Evidence

`SlashEvidence` MUST be serialized as JSON and MUST contain:

- `task_snapshots`: one task snapshot for each task in the validation group. Each snapshot MUST contain task ID commitment, revealed task ID, task arguments, task type, task version, creator, task fee, score, QoS score, model IDs, and task timestamps.
- `node_snapshots`: one assignment-time node snapshot for each task in the validation group. Each snapshot MUST contain node identity, hardware, version, staking, QoS, health, delegated staking summary, and model state.
- `validation_context`: invalidation reason, revealed task ID, group task IDs, and group task ID commitments.
- `input_artifacts`: one optional uploaded input artifact record for each task in the validation group. Normal task input MUST be stored in `task_snapshots[].task_args`; `input_artifacts` is used only when a task type has an uploaded input artifact, such as a fine-tune LoRA checkpoint.
- `result_artifacts`: one result artifact record for each task in the validation group. Result artifacts are the final uploaded task outputs, such as generated images, an LLM response JSON/text file, and a fine-tune LoRA checkpoint.
- `incomplete_reason` when evidence is incomplete.

Relay MUST copy uploaded input artifacts from `data/inference_tasks/<task_id_commitment>/input` to `data/slashed_tasks/<task_id_commitment>/input` for each validation group task when the source path exists. If a task's source input artifact path is absent, that task's `input_artifacts[].status` MUST be `missing`.

Relay MUST store each validation group task's result artifacts under `data/slashed_tasks/<task_id_commitment>/results` when any task in the group is invalidated. Before each task's result upload, that task's `result_artifacts[].status` MUST be `pending_upload`. After any validation group task uploads its result, Relay MUST update matching pending slash evidence with all three group result artifact records. Uploaded task records MUST use `result_artifacts[].status = uploaded`; group task records without uploaded files MUST remain `pending_upload`.

The slash evidence copy under `data/slashed_tasks` MUST be the canonical result artifact location for all three validation group tasks when the group contains an invalidated task. Normal successful result storage under `data/inference_tasks/<task_id_commitment>/results` MAY also exist for successful group tasks, but slash review MUST read the evidence paths from `result_artifacts`.

## Passive Flow

When validation sets a task to `TaskEndInvalidated` and `task.passive_slash_mode` is `true`, Relay MUST:

1. Update the task status to `TaskEndInvalidated`.
2. Emit `TaskEndInvalidated`.
3. Create a `pending_slashes` row with `status = pending`, node address, node current blockchain network, task ID commitment, evidence JSON, and evidence completeness.
4. Finish the node's task through the normal post-task node handling path.

Passive mode MUST NOT emit `NodeSlashed`, queue a slash transaction, slash vesting records, or create delegated slash jobs during validation.

Passive mode MUST allow the node to continue through normal post-task state transitions. A busy node MUST become available, a pending-pause node MUST pause, and a pending-quit node MUST quit. Pending slash records do not guarantee future slash execution.

## Active Flow

When validation sets a task to `TaskEndInvalidated` and `task.passive_slash_mode` is `false`, Relay MUST:

1. Update the task status to `TaskEndInvalidated`.
2. Emit `TaskEndInvalidated`.
3. Execute `SlashNode` with the task ID commitment and slash evidence.
4. Emit `NodeQuit` and `NodeSlashed` through the normal slash flow.

Active mode MUST NOT create a `pending_slashes` row.

## Pending Slash States

`pending_slashes.status` MUST use the following values:

- `pending`: the invalidation has been recorded for admin review and no approval slash has been executed through the pending record.
- `slashed`: an admin approval slash has been submitted through the pending record.

`pending_slashes` MUST NOT duplicate blockchain transaction IDs, slash event IDs, delegated slash jobs, or delegated slash audit rows. The existing slash execution records remain the source of truth for executed slashes.

## Admin Review API

Relay MUST expose authenticated admin endpoints:

- `GET /v2/admin/pending_slashes`: lists pending slash review records with evidence.
- `GET /v2/admin/pending_slashes/:pending_slash_id`: returns one pending slash review record with evidence.
- `GET /v2/admin/pending_slashes/:pending_slash_id/artifacts/:artifact_type/:task_id_commitment/:file_name`: downloads one pending slash evidence artifact. `artifact_type` MUST be `input` or `result`. Relay MUST serve only files listed in the pending slash evidence for the requested task ID commitment.

Relay MUST allow `POST /v2/admin/nodes/slash` to accept `pending_slash_id`. When `pending_slash_id` is present, Relay MUST load the pending row, require `status = pending`, parse its evidence, use its node address and task ID commitment for `SlashNode`, and mark the pending row `slashed` after slash execution.

Admin-triggered slash without `pending_slash_id` MUST keep using the existing node-address flow. When no task ID commitment is available, `NodeSlashed` MUST use `0x`.

## Final Slash Evidence

`NodeSlashedEvent` MUST include an optional `evidence` field in the serialized `events.args` JSON. Historical `NodeSlashed` rows without evidence MUST remain readable.

In active mode, Relay MUST embed the evidence captured during validation directly in the `NodeSlashed` event. In passive mode, Relay MUST embed the pending slash evidence in the `NodeSlashed` event when an admin approves the pending slash.

Slash reports MUST unmarshal `NodeSlashedEvent` from `events.args` and prefer embedded evidence for card name, task context, and node snapshot fields. Slash reports MUST use the current `nodes` table only as a fallback for historical `NodeSlashed` events without evidence.
