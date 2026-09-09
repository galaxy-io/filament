# Pipeline Notifiers: Implementation PR Stack

Status: Proposed implementation sequence

Date: 2026-09-09

Specification: [Pipeline Notifiers Technical PRD](notifier-prd.md)

## 1. Stack overview

Implement the backend in **eight incremental PRs**, beginning with database migrations, domain types, and interfaces. Every PR must compile, include the tests for its own behavior, and be reviewable against its immediate parent. External notification delivery starts only after the final wiring PR is deployed and consumption is enabled.

This plan organizes the existing PRD; it does not introduce additional notification types or delivery guarantees.

| PR | Suggested title | Result after merge |
| --- | --- | --- |
| 1 | Add notifier schema, enum, and persistence interface | Both databases can represent notifier configuration; domain and protobuf types are defined. |
| 2 | Implement tenant-scoped notifier persistence | Both datastores support versioned CRUD and pipeline deletion behavior. |
| 3 | Define notification contracts, validation, and attempt events | Shared sender contracts and validation exist; consumers understand `notifier.attempted`. |
| 4 | Add pipeline notifier APIs and secret management | Callers can configure multiple notifiers per pipeline securely. |
| 5 | Implement the webhook notification sender | A tested sender can deliver the specified webhook payload when invoked directly. |
| 6 | Make in-process event publication cancellation-aware | Publishing into a full local subscriber buffer can be interrupted safely. |
| 7 | Implement the generic notifier module | The module matches events, dispatches senders, and reports attempts in tests. It is not mounted yet. |
| 8 | Wire notifier delivery into application runtimes | The feature works end to end, with deployment controls, integration coverage, and operator documentation. |

Use this review order even where code dependencies are looser. PR 5 primarily needs PR 3; PR 6 is an independent transport fix. Keeping a linear stack makes the overall feature easier to follow without asking reviewers to resolve a branching dependency graph.

### Decisions carried through every PR

- The configuration table is singular **`notifier`** and the module is generic **`notifier`**.
- A pipeline supports multiple independent rules, up to the PRD's eight-row limit. Disabled, non-deleted rows count toward the limit.
- `NotificationType` is an enum: protobuf/Go have `UNSPECIFIED` and `WEBHOOK`; Postgres/SQLite persist only the `webhook` label.
- There is no `EVENT` notification type or placeholder sender.
- `notifier.attempted` remains a shared tracking event. It is not a notification channel.
- Graph versions, checkpoint identities, and pipeline execution status are unaffected by notification configuration or failures.
- NATS provides retained source-message redelivery. Attempt history is best-effort; duplicates and missing reports remain possible.
- No frontend screens, delivery table, outbox, retry scheduler, or additional channel implementations are part of the stack.

## 2. PR 1 — Database migration, enums, domain types, and interfaces

**Suggested branch:** `mitch/notifier-01`

**Base:** Repository default branch

**PRD coverage:** Persistence model, enum contract, and optional datastore capability

### Changes

1. Add a new Postgres migration defining `notification_type AS ENUM ('webhook')` and the `notifier` table from the PRD.
2. Add the equivalent SQLite migration using a text column with a closed-set `CHECK` constraint. Follow existing JSON and timestamp conventions.
3. Include ownership, filters, config/secret references, version, soft-delete, and audit columns; add the composite pipeline/tenant foreign key and active-list index.
4. Add root domain types in `notifier.go`: integer `NotificationType`, `NotificationUnspecified`, `NotificationWebhook`, `Notifier`, and the minimal query/mutation parameter types needed by the persistence interface.
5. Define a separate `NotifierStore` capability with create, scoped load/list, versioned update, and versioned soft-delete operations. Include tenant and pipeline identity in every operation. Keep `DataStore` unchanged.
6. Define the protobuf enum in a new `notifiers.proto` and generate its Go and TypeScript definitions. Do not add service RPCs yet.
7. Add explicit domain/database label conversion and strict JSON conversion for the domain enum. Protobuf-to-domain conversion is added with the API in PR 4.
8. Regenerate sqlc models affected by the new schema. Additive generated model changes belong with the migration that caused them.

Finalize method signatures here, including how returned records expose old references for later cleanup and how soft-deleted metadata can be inspected internally. Avoid exposing raw SQL types through the interface. The interface must permit PR 2 to perform atomic version checks and PR 4 to clean up only owned secrets after committed mutations.

### Validation

- Apply migrations to empty and previously migrated Postgres/SQLite databases.
- Verify the table, index, composite ownership constraint, defaults, and enum/check constraint directly through the databases.
- Test domain enum conversion: valid round trips, rejected unknown values, and rejection of persisted `UNSPECIFIED`.
- Check protobuf lint and deterministic code generation.
- Confirm existing datastore implementations and custom adapters still compile without implementing `NotifierStore`.

### Review boundary and exit criteria

This PR defines storage and contracts. It adds no CRUD query implementation, RPC, sender, consumer, or automatic secret writes. Existing code can run against the migrated schema unchanged. Do not change already-applied migrations or assign fixed future migration filenames before checking the current sequence at implementation time.

## 3. PR 2 — Datastore implementations and pipeline lifecycle

**Suggested branch:** `mitch/notifier-02`

**Base:** PR 1

**PRD coverage:** Versioned persistence, ownership, collection limit, and parent deletion

### Changes

1. Add notifier queries, generated sqlc code, and datastore adapters for Postgres and SQLite.
2. Implement `NotifierStore` in both providers and add compile-time interface assertions.
3. Scope all reads/writes by tenant and pipeline; keep list ordering deterministic.
4. Recheck parent pipeline liveness inside create/update/delete transactions.
5. Enforce the eight non-deleted row limit under a parent lock or equivalent SQLite write transaction. An API-only count check is insufficient.
6. Implement compare-and-swap updates/deletes using expected version. Increment versions and stamp audit metadata according to repository conventions.
7. Extend pipeline deletion to soft-delete its notifier rows within the existing transaction that handles schedules and pending runs.
8. Preserve enough scoped metadata to support managed-secret cleanup in PR 4. Datastore code does not call the secrets provider.

### Validation

- Exercise equivalent create/load/list/update/delete behavior in both databases.
- Verify cross-tenant and cross-pipeline IDs are inaccessible.
- Race two updates at the same version; exactly one succeeds.
- Race creates at the collection limit; the database cannot end up with nine non-deleted rows.
- Verify disabled rows count toward the limit and deleted rows do not.
- Race notifier creation with pipeline deletion; no newly live notifier survives a deleted parent.
- Verify pipeline deletion keeps executed run history and soft-deletes its notification rules.
- Verify notifier edits never create graph versions or alter checkpoint and replication identities.

### Review boundary and exit criteria

Both stores fully implement the optional capability and preserve transactional invariants. Configuration is accessible through internal Go APIs only. No network or secret-provider side effects occur.

## 4. PR 3 — Shared notification contracts, validation, and event schema

**Suggested branch:** `mitch/notifier-03`

**Base:** PR 2

**PRD coverage:** Generic sender contract, rule validation, destination configuration, and attempt-event contract

### Changes

1. Add `internal/notification` with `Notification`, `DeliveryResult`, outcome/error classifications, and the `Sender` interface.
2. Keep persistence-only types and `NotificationType` in the root package. The shared contract package may import `events`; the root package must not import it.
3. Add stable delivery-ID construction and attempt-ID generation, following the PRD's notifier/subject/broker-sequence identity rules.
4. Add shared rule validation: catalog-backed exact event names, `*`, resource lists, bounds, and exclusion of `notifier.*` lifecycle events.
5. Add webhook destination types and validation under `internal/notification/webhook`: URL/header shape, protected headers, payload bounds, HTTP opt-in, and shared destination-address policy helpers. These are reused by the API and sender.
6. Define `NotifierAttemptedEvent` and register `notifier.attempted` in the existing catalog. Include the full correlation, outcome, `request_attempted`, and sanitized error-code fields from the PRD.
7. Verify existing event consumers can decode the new registered fact without changing pipeline state. Adjust any consumer that incorrectly assumes a closed set of actionable event types.

### Validation

- Round-trip the attempt event through the existing codec, including domain enum wire labels and decimal-string broker sequence.
- Verify delivery IDs are stable on source redelivery and differ for different notifiers or source messages; verify attempt IDs are distinct.
- Test rule validation against every current eligible catalog event, wildcard rules, invalid names, lifecycle exclusions, and resource filtering semantics.
- Test destination validation and address classification without making external requests; use injected resolver/address fixtures.
- Test invalid/secret-bearing inputs produce sanitized errors.
- Verify tracker and run-tail handling preserve existing run lifecycle/progress behavior when they encounter an attempt event.

### Review boundary and exit criteria

This PR defines contracts and permits consumers to understand the new event. It sends no webhook, publishes no attempt fact in production, and mounts no module. Keep validation in this PR so the API in PR 4 does not need to duplicate logic that the sender later replaces.

Deploying this PR's catalog support before PR 8 enables publication avoids older codecs terminating the new event as unknown.

## 5. PR 4 — Pipeline notifier APIs and secrets

**Suggested branch:** `mitch/notifier-04`

**Base:** PR 3

**PRD coverage:** Four pipeline-scoped RPCs and the full credential lifecycle

### Changes

1. Extend `notifiers.proto` with separate write-only input and safe response messages; add create/update/list/delete request/response messages.
2. Add `CreatePipelineNotifier`, `UpdatePipelineNotifier`, `ListPipelineNotifiers`, and `DeletePipelineNotifier` to `IngestionService`.
3. Implement handlers, explicit protobuf/domain enum conversion, error mapping, and `NotifierStore` capability detection. Unsupported custom stores return `Unimplemented`.
4. Reuse existing authentication, tenant context, and mutation logging conventions. Do not add unauthenticated routes.
5. Support inline destination JSON or an existing destination secret reference. Omitted update credentials preserve the current reference; supplied credentials replace the whole destination.
6. Add notifier-owned, tenant-scoped reference generation with random revision IDs and exact ownership checks.
7. Implement write-before-persist, cleanup of a newly written secret on failed persistence, and best-effort cleanup of replaced/deleted managed references only after database success.
8. Add managed notifier-secret cleanup to the pipeline deletion API path. Its datastore mutation already became transactional in PR 2.
9. Support environment/read-only providers through supplied JSON references; return a sanitized error for inline input when writing is unavailable.
10. Add API-side destination-policy options using PR 3's policy types. Leave executable environment configuration to PR 8.
11. Generate Go/Connect and TypeScript definitions and add API reference documentation.

### Validation

- Exercise all four RPCs through the Connect handler with tenant context and both persisted providers where relevant.
- Verify multiple independent notifiers can be created, updated, disabled, and deleted on one pipeline.
- Cover stale versions, unknown enum values, deleted parents, mismatched sender input, and cross-tenant references.
- Verify credentials are absent from table columns, API responses, error messages, logs, run requests, and graph versions.
- Verify racing updates use distinct secret references; losers clean up only their own writes.
- Verify replacing/removing a notifier never deletes external references or another notifier's managed secrets.
- Test read-only, missing, malformed, and failing secret providers, including cleanup failures after metadata commits.
- Verify request/response/codegen compatibility and the optional-store `Unimplemented` path.

### Review boundary and exit criteria

Callers can securely configure notifier rules, but no production delivery module exists yet. The API may accept `enabled=true`; it does not promise delivery until consumption is deployed. State this staging behavior in the PR description and deployment notes.

No existing pipeline create/update message is replaced, and no bulk notifier replacement or frontend controls are added.

## 6. PR 5 — Webhook sender

**Suggested branch:** `mitch/notifier-05`

**Base:** PR 4; code depends primarily on PR 3

**PRD coverage:** HTTP delivery, stable payload, transport policy, and outcome classification

### Changes

1. Implement the webhook `Sender` using an injected reusable HTTP client.
2. Build the fixed versioned notification envelope and embed the trigger using `events.Marshal`. Keep whole run requests and resolved configuration out of the payload.
3. Add Filament delivery/attempt/event headers and merge allowed destination headers.
4. Enforce the five-second request timeout, disabled redirects, TLS verification, bounded response reading, and response-body closure.
5. Implement the dedicated transport using PR 3's address policy: resolve and validate at dial time, dial an approved address, preserve TLS server name, and bypass ambient proxy environment settings.
6. Classify accepted, failed, and unknown outcomes, retryability, request-attempt status, HTTP status, and sanitized error codes.

### Validation

- Use controlled HTTP servers for the full response-classification matrix, including 2xx, permanent rejection, retryable responses, timeouts, and uncertain transport failures.
- Verify request payload and headers, protected-header handling, credential forwarding only to the configured destination, and response closure.
- Cover redirect refusal, DNS rebinding, private/loopback/link-local IPv4/IPv6 cases, approved internal destinations, and HTTP opt-in with injected transport fixtures.
- Verify cancellation and request timeouts terminate sender work.
- Verify no raw response body, credential, or secret-bearing URL escapes into returned diagnostics.

### Review boundary and exit criteria

The sender works through direct invocation and controlled tests. It owns no NATS subscription, database lookup, secret-provider lookup, retry loop, or attempt-event publication. Those remain module responsibilities.

## 7. PR 6 — In-process event bus cancellation

**Suggested branch:** `mitch/notifier-06`

**Base:** PR 5 for review order; no notifier code dependency

**PRD coverage:** Bounded publication and shutdown on the embedded transport

### Changes

1. Make blocking fanout during `inproc.Publish` observe the publish context.
2. Return a context error when publication cannot finish before cancellation, even if some subscribers already received the message.
3. Preserve successful publication behavior, subscription closure, and existing negative-ack redelivery semantics.
4. Document partial fanout on cancellation in the relevant provider contract/comments.

This is a small transport correction motivated by a concrete module behavior: publishing an attempt event while the module's own subscription buffer is full must not hang past the handler deadline.

### Validation

- Fill a subscriber buffer, publish, cancel, and assert prompt return without needing to close the entire bus.
- Exercise cancellation between two subscriber deliveries and document/verify permitted partial delivery without assuming map iteration order.
- Race close, publish, and cancellation; check for panics, leaked goroutines, and stuck wait groups.
- Run existing in-process delivery/replay/redelivery tests and the relevant race tests.

### Review boundary and exit criteria

No changes to `eventbus.Bus` signatures, host acknowledgement policy, NATS behavior, buffer sizes, or notification configuration. This PR can be landed independently earlier if useful; it must land before embedded notifier wiring.

## 8. PR 7 — Generic notifier module

**Suggested branch:** `mitch/notifier-07`

**Base:** PR 6

**PRD coverage:** Matching, fanout, secrets at delivery, attempt reporting, and source acknowledgement

### Changes

1. Add `internal/modules/notifier` implementing the existing module contract.
2. Inject dependencies through `module.Deps` and a constructor-supplied sender map keyed by the domain enum.
3. Declare the subscription exactly as specified: `ingestion.v1.>`, durable `notifier`, `Replay=false`, `MaxInFlight=1`.
4. Ignore `notifier.*` before datastore access. Resolve pipeline ownership through the tenant-scoped saved run request.
5. Load current notifier rows and filter enabled rules by event/resource. Do not cache rules across messages or snapshot them onto runs.
6. Resolve destination secrets just before delivery. Reload once on a missing managed reference if configuration changed; skip newly deleted/disabled rows.
7. Perform bounded fanout to all matching rules, with the PRD's eight-operation concurrency cap, 18-second processing phase, and 25-second whole-handler budget.
8. Publish correlated `notifier.attempted` facts through shared `events.Emit`, including resolution failures with `request_attempted=false`.
9. Aggregate delivery retryability into the host's nil/error acknowledgement contract. Reporting failure alone must not turn an accepted remote request into a retryable delivery.
10. Add structured diagnostics and bounded-label metrics from the PRD; make lifecycle and cancellation behavior explicit.

### Validation

- Use a fake sender and controlled bus/store/secret providers to isolate matching and acknowledgement behavior.
- Exercise all eligible catalog events, multiple notifiers, exact resource filters, disabled/deleted rows, missing parents/runs, and runs without pipelines.
- Verify injected malformed lifecycle rules cannot create notification loops.
- Confirm one failed sender does not prevent other matching senders from being attempted.
- Cover missing/rotated credentials and settings changes between processing passes.
- Verify stable delivery IDs and distinct attempt IDs on redelivery; a successful destination can repeat when another requires retry.
- Simulate report-publication failure after HTTP acceptance and verify it is observable without independently causing source retry.
- Verify deadlines, bounded fanout, cancellation, shutdown reporting, and no detached delivery goroutines.
- Cover typed sender selection and fail closed on corrupt/unknown persisted notification types.

### Review boundary and exit criteria

The module is tested and ready to mount. It is not yet added to `app.compose` or the distributed control plane. No production process starts external delivery merely by merging this PR.

## 9. PR 8 — Runtime wiring, end-to-end verification, and rollout

**Suggested branch:** `mitch/notifier-08`

**Base:** PR 7

**PRD coverage:** Production/embedded composition, deployment controls, integration acceptance, and operator documentation

### Changes

1. Register the webhook sender and mount the notifier module in `cmd/control-plane/main.go` and `app/app.go`.
2. Keep delivery consumption out of the API-only server and worker binary. Ensure replicas share the stable durable name.
3. Add the PRD's programmatic options and executable settings: `NOTIFIER_ENABLED`, `NOTIFIER_ALLOW_HTTP`, and `NOTIFIER_ALLOWED_CIDRS`. Validate settings at startup and propagate consistent destination policy to API and delivery processes.
4. Preserve custom datastore compatibility: omit consumption with a diagnostic when `NotifierStore` is absent, and retain `Unimplemented` notifier RPCs.
5. Keep configured secret providers unchanged, including standalone's read-only environment provider.
6. Add real JetStream and end-to-end coverage across API configuration, stored rules/secrets, pipeline events, HTTP requests, and attempt facts.
7. Document configuration, secret-reference examples, current per-route completion semantics, duplicate handling, counting attempts, retention, local backpressure, and the staged rollout procedure.

### Validation

- Start an existing pipeline, create multiple enabled notifiers via RPC, and verify matching receivers and correlated attempt facts.
- Restart the notifier process and verify the existing durable resumes pending work.
- Create a fresh durable against retained historical messages and confirm no initial backlog replay.
- Bind two module hosts to the same durable; verify shared consumption while retaining the documented duplicate possibility after failures.
- Use one accepting and one transiently failing receiver to prove whole-message retry semantics and stable delivery identity.
- Exercise disable/delete before redelivery, pipeline deletion, and missing/rotated secrets.
- Verify catalog-aware tracker/tail consumers tolerate notification events and pipeline results remain independent of sender failures.
- Exercise embedded NATS and the in-process application composition, including a full-buffer cancellation regression and bounded shutdown.
- Verify `NOTIFIER_ENABLED=false` starts no notification consumer and does not delete the durable or stored rules.
- Verify empty notifier collections and datastores without the optional capability preserve existing application behavior.
- Run affected root, `cmd`, and integration-module builds/tests using the repository's Go module conventions, and finish the full PRD acceptance checklist.

### Review boundary and exit criteria

This is the first PR that enables runtime consumption. Its integration tests and operator instructions ship together with that activation; no essential reliability test is deferred to a later cleanup PR.

The stack is complete when configuration, webhook delivery, attempt reporting, and both shipped datastore paths satisfy the PRD. Frontend implementation remains a separate effort.

## 10. Review and merge mechanics

Suggested stack bases are PR 1 against the repository's actual default branch and each subsequent PR against its predecessor. Branch names above are suggestions, not branches created by this document.

- Keep migrations and affected sqlc models together; keep protobuf changes and generated clients together.
- Add service RPC declarations and their working handlers in PR 4 so intermediate commits do not depend on unfinished server implementations.
- Keep shared validation in PR 3, then reuse it in PRs 4 and 5. Avoid a temporary second validator or a knowingly permissive API.
- Put ownership/version/count invariants in datastore transactions in PR 2, with API validation as an additional layer.
- Keep secret side effects out of datastore code and HTTP mechanics out of the module.
- Introduce metrics and logs with the module behavior they describe; PR 8 verifies wiring rather than inventing a second observability implementation.
- Each PR description should state its resulting behavior, validation performed, whether it changes production behavior, and the parent PR dependency.
- After a predecessor merges, retarget/rebase successors as needed so their diffs remain incremental. Re-run checks affected by conflict resolution.

Every PR must pass relevant generation checks and focused tests. Broaden testing for new failure evidence or final composition; do not defer a PR's essential tests to PR 8. Documentation-only changes do not need application tests.

## 11. Deployment milestones and rollback

Merge order and production rollout order are related but distinct. A stacked PR being merged does not prove all running processes contain its catalog changes.

| Milestone | What is safe to deploy | Operational behavior |
| --- | --- | --- |
| After PR 1 | Additive schema | Existing binaries continue working; no notifications are configured through APIs. |
| After PR 3 | Catalog-aware consumers | All consumers can decode `notifier.attempted`; nothing publishes it yet. |
| After PR 4 | Configuration API | Rules and encrypted destinations can be created; consumption is not mounted. |
| After PR 7 | Module implementation | Code exists but remains inactive in shipped compositions. |
| PR 8 staged | Runtime wiring with `NOTIFIER_ENABLED=false` | Final configuration can be verified without starting delivery. |
| PR 8 enabled | Consumption after catalog rollout is verified | Matching events begin to generate HTTP requests and attempt facts. |

Before activation, verify migrations are applied, every relevant consumer has the new event catalog, and API/control-plane destination policy agrees. Start with a controlled pipeline and receiver. `NOTIFIER_ENABLED` defaults to true in the PRD, so set it explicitly to false for a staged deployment; do not assume it is opt-in.

Rules configured before the durable consumer is first created do not cause historical replay. Enabling an already-existing durable can process pending events using current notifier settings. Disabled/deleted rules are skipped on that new processing pass.

To stop delivery, disable notifier consumption or disable the affected rules. A request already in flight may finish. Preserve durable state, configuration, and secrets; do not delete a consumer or drop the schema as a routine rollback. Prefer rolling back to catalog-aware binaries so retained attempt facts remain decodable. Do not run a down migration over populated notifier data to undo application activation.

## 12. PRD acceptance ownership

| Requirement | Primary owner | Final verification |
| --- | --- | --- |
| Enum and schema parity | PR 1 | PR 2 and PR 8 |
| Multiple rules, tenant isolation, version/count races | PR 2 | PR 4 and PR 8 |
| Pipeline deletion and secret cleanup | PR 2 for rows; PR 4 for secrets | PR 8 |
| Shared contract and catalog-wide validation | PR 3 | PR 7 |
| Credentials, safe RPCs, generated clients | PR 4 | PR 8 |
| HTTP payload, destination policy, classification | PR 5 | PR 8 |
| Local transport cancellation | PR 6 | PR 7 and PR 8 |
| Matching, retry decisions, loop prevention, tracking | PR 7 | PR 8 |
| Durable restart, replica behavior, deployment controls | PR 8 | PR 8 |

The next implementation step is **PR 1: notifier migrations, enum, domain types, and the optional persistence interface**. Creating this plan does not create branches, open pull requests, apply migrations, or start implementation.
