# Task Error Diagnostic Reporting

This document specifies Relay storage, authorization, and administrative access for Node task execution diagnostics.

## Protocol Isolation

Task error diagnostics MUST remain separate from the Task protocol. Accepting, rejecting, or storing a diagnostic report MUST NOT change Task status, QoS, settlement, validation, refund, invalidation, or slashing behavior.

The existing protocol error report MUST remain limited to `TaskInvalid`. All other unfinished Tasks MUST retain the existing Relay timeout behavior.

Relay MUST accept a diagnostic report after its Task reaches any terminal status.

## Node Report API

Nodes MUST submit diagnostic reports to:

`POST /v2/tasks/:task_id_commitment/node_error`

The signed request data MUST contain:

- `node_address`;
- `task_id_commitment`;
- `task_args`;
- `error_type`;
- `message`;
- `stack_trace`.

The request envelope MUST also contain `captured_at`, `timestamp`, and `signature`. `captured_at` MUST be an `int64` Node capture time and MUST be stored with the report, but MUST NOT be included in the signed data. `task_args` MUST contain the complete Task arguments used by the Node. `stack_trace` MUST contain the complete traceback when one exists, or the Node-generated no-traceback explanation.

Relay MUST canonicalize only the six signed fields as JSON using lexicographically sorted object keys and compact separators. Relay MUST append the base-10 `timestamp` without a separator and MUST verify the secp256k1 signature against the Keccak-256 hash of those bytes. The timestamp MUST be within 60 seconds of Relay time.

Relay MUST reject the request unless all of the following conditions hold:

- the signature is valid;
- the path `task_id_commitment` equals the signed body `task_id_commitment`;
- the recovered signer address equals `node_address`;
- the Task selected by `task_id_commitment` exists;
- the recovered signer address equals the Task's selected Node address.

Address comparisons MUST use Ethereum address identity. A valid report MUST be stored with the checksummed Node address.

Relay MUST NOT require a specific Task status and MUST NOT invoke a Task state transition service while processing this API.

## Storage and Idempotency

Relay MUST store reports in `node_task_errors`. Each record MUST contain:

- Relay record ID, creation time, and update time;
- Node address;
- Task ID Commitment;
- complete Task arguments;
- error type;
- diagnostic message;
- complete stack trace or no-traceback explanation;
- Node capture time.

The pair `(node_address, task_id_commitment)` MUST be unique. A retry for an existing pair MUST return success without changing the existing record. This constraint limits one Node execution attempt for one Task ID Commitment to one stored report.

The table MUST provide exact-query indexes for Node address and Task ID Commitment and an index supporting `created_at DESC, id DESC` traversal.

## Admin Query API

Administrators MUST query reports through:

`GET /v2/admin/node_task_errors`

The endpoint MUST use the existing Admin authentication middleware. It MUST accept:

- optional exact `node_address`;
- optional exact `task_id_commitment`;
- `page`;
- `page_size`.

The filters MAY be used independently, together, or omitted. Relay MUST use equality predicates and MUST NOT use pattern matching. Results MUST be ordered by `created_at DESC, id DESC`.

`page` MUST default to `1`. `page_size` MUST default to `30` and MUST be limited to `100`. The response data MUST contain `total`, `page`, `page_size`, and `items`. Every item MUST include the complete Task arguments and stack trace content.
