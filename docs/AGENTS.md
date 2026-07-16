## Documentation Index

| Document | Description |
|----------|-------------|
| [architecture.md](./architecture.md) | Single Relay service boundary, configured blockchain networks, node current blockchain network, and terminology rules |
| [task-pricing.md](./task-pricing.md) | Task priority calculation, VRAM weighting, execution-time estimation, calibration, and queue dispatch ordering |
| [task_matching.md](./task_matching.md) | Node scheduling index, batch matching rounds, base-model readiness handling, in-round reservation, and dispatch consistency |
| [node_selection.md](./node_selection.md) | Qualification filters, base-model gate, staking and QoS weight, in-memory locality, and weighted sampling |
| [qos.md](./qos.md) | Long-term performance score (`Q_long`) and short-term reliability factor (`H`) that compose the runtime QoS |
| [task_version.md](./task_version.md) | Version matching rules between task requirements and node capabilities |
| [node_quit_and_unstake.md](./node_quit_and_unstake.md) | Node quit, Relay admin unstake, on-chain recovery unstake, kickout, and slash precedence |
| [task_timeout.md](./task_timeout.md) | Queue timeout, running timeout, abort completion, node health penalty, and timeout race handling |
| [task_validation_and_slashing.md](./task_validation_and_slashing.md) | Validation task lifecycle, result comparison, and slashing conditions |
| [task_tracing.md](./task_tracing.md) | Admin task trace target, lifecycle timestamps, base-ready candidate snapshots, validation, upload, and missing-data reporting |
| [passive_slash_model.md](./passive_slash_model.md) | Passive slash review mode, slash evidence capture, pending slash states, and admin approval flow |
| [relay_event_stream.md](./relay_event_stream.md) | Relay `events` table, v1 event APIs, node watcher polling, and single-shot base-model download events |
| [model_distribution.md](./model_distribution.md) | Demand-driven model distribution controller, persisted download selections, timeout and replacement, and on-disk authority |
| [loaded_models.md](./loaded_models.md) | Successful model execution projection, authoritative node model reports, node counts, and public v2 loaded models API |
| [all_nodes_data.md](./all_nodes_data.md) | Historical joined-node public snapshot, join upsert behavior, quit retention, and SyncNetwork refresh rules |
| [deposit_withdraw_and_risk_control.md](./deposit_withdraw_and_risk_control.md) | Deposit and withdrawal lifecycle across Relay and Wallet, relay account ledger, and risk control checks |
| [deposit-withdraw-only-networks.md](./deposit-withdraw-only-networks.md) | Deposit and withdrawal only network configuration, processing, BenefitAddress |
| [task_fee_charge_and_settlement.md](./task_fee_charge_and_settlement.md) | Task fee charge, refund, settlement split, and rounding rules in relay account ledger |
| [delegated_staking.md](./delegated_staking.md) | Delegation state sync, total-staking selection impact, income split, and delegated staking API surface |
| [stakeable_node_list_filter_and_sorting.md](./stakeable_node_list_filter_and_sorting.md) | Snapshot-backed filtering and sorting for the Portal stakeable node list |
| [delegated_staking_slash.md](./delegated_staking_slash.md) | Batched delegated slash ownership, pagination, audit, and recovery requirements |
| [node-multi-chain-events.md](./node-multi-chain-events.md) | Node join network ownership, multi-chain blockchain processor guards, and join-time staking state rebuild |
| [relay_account_event_cache_flow.md](./relay_account_event_cache_flow.md) | End-to-end relay account flow from event creation to in-memory cache mutation and DB projection for task and withdraw paths |
| [portal_netstats_chart.md](./portal_netstats_chart.md) | Portal netstats chart inventory, data sources, and aggregation logic |
| [emission.md](./emission.md) | Emission week boundaries, task fee allocation, vesting creation and release, and chart aggregation |
| [emission_estimation.md](./emission_estimation.md) | Current-week estimated upcoming emission calculation, snapshot refresh, and API exposure |
| [monitoring.md](./monitoring.md) | Prometheus metric definitions, task delivery and node last-seen tracking, and the AWS AMP/AMG deployment runbook |

## Doc Update Requirements

When updating documentation files:

1. Read the entire document first to understand its structure, sections, and flow
2. Find the most appropriate location to integrate new content based on:
   - Logical relationship with existing sections
   - Document flow and narrative
   - Where readers would naturally expect to find the information
3. Integrate new content naturally into existing sections when possible:
   - Add as a paragraph within a relevant section
   - Extend an existing list or table
   - Add as a subsection under an appropriate parent section
   - Distribute across multiple sections if a feature affects different parts of the document
4. Do NOT simply create a new top-level section and place all new content there
5. Only create a new section if the topic is truly distinct from all existing content

Write documentation as a specification.

Documentation MUST state clear, final decisions and requirements.

Documentation MUST NOT include:
- Recommendations or advice.
- Options or alternatives.
- Speculation or uncertainty.
- Future-facing placeholders.

Documentation MUST use definitive language that can be implemented and tested:
- Requirement keywords: MUST, MUST NOT, SHALL, SHOULD. Use SHOULD only when a requirement level is intended.
- Exact behavior, constraints, and interfaces.

## Architecture Terminology Requirements

Documentation MUST follow [architecture.md](./architecture.md) for system boundary and terminology.

Crynux Relay is one off-chain Relay service that works with multiple configured blockchain networks. Documentation MUST NOT describe this as multiple Relay networks.

The term `network` in blockchain-facing features MUST mean blockchain network unless the document explicitly defines another scope. Documentation MUST use `blockchain network` or `node current blockchain network` when precision is required.

Documentation MUST NOT use `Relay network`, `current Relay network`, `node Relay network`, or `task network` to mean a blockchain network. Tasks are scheduled and dispatched by Relay and are not partitioned by blockchain network.

## Chat Content Isolation

Documentation MUST be generated from task requirements and authoritative project sources only.
User chat instructions about removing content are editing actions, not document content.
The final document MUST NOT restate removal instructions.
If a content type is removed, it must be absent from the final document.

Example chat cycle:
- AI draft includes setup commands.
- User says remove setup commands and keep only flow.
- Wrong final doc line: This document does not include setup commands.
- Right final doc line: Run the flow in order: prepare environment, start services, execute deposit and withdraw, then verify results.
