## Documentation Index

| Document | Description |
|----------|-------------|
| [node_selection.md](./node_selection.md) | Hard filters, base weight, model locality boost, and weighted sampling for task-to-node assignment |
| [qos.md](./qos.md) | Long-term performance score (`Q_long`) and short-term reliability factor (`H`) that compose the runtime QoS |
| [task_version.md](./task_version.md) | Version matching rules between task requirements and node capabilities |
| [node_quit_and_unstake.md](./node_quit_and_unstake.md) | Node quit, Relay admin unstake, on-chain recovery unstake, kickout, and slash precedence |
| [task_validation_and_slashing.md](./task_validation_and_slashing.md) | Validation task lifecycle, result comparison, and slashing conditions |
| [model_predownload.md](./model_predownload.md) | Pre-download scheduling, node notification, and model availability tracking |
| [deposit_withdraw_and_risk_control.md](./deposit_withdraw_and_risk_control.md) | Deposit and withdrawal lifecycle across Relay and Wallet, relay account ledger, and risk control checks |
| [deposit-withdraw-only-networks.md](./deposit-withdraw-only-networks.md) | Deposit and withdrawal only network configuration, processing, BenefitAddress |
| [task_fee_charge_and_settlement.md](./task_fee_charge_and_settlement.md) | Task fee charge, refund, settlement split, and rounding rules in relay account ledger |
| [delegated_staking.md](./delegated_staking.md) | Delegation state sync, total-staking selection impact, income split, and delegated staking API surface |
| [delegated_staking_slash.md](./delegated_staking_slash.md) | Batched delegated slash ownership, pagination, audit, and recovery requirements |
| [node-multi-chain-events.md](./node-multi-chain-events.md) | Node join network ownership, multi-chain blockchain processor guards, and join-time staking state rebuild |
| [relay_account_event_cache_flow.md](./relay_account_event_cache_flow.md) | End-to-end relay account flow from event creation to in-memory cache mutation and DB projection for task and withdraw paths |
| [portal_netstats_chart.md](./portal_netstats_chart.md) | Portal netstats chart inventory, data sources, and aggregation logic |
| [emission.md](./emission.md) | Emission week boundaries, task fee allocation, vesting creation and release, and chart aggregation |

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
