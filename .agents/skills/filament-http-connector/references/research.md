# Research the API contract

Use this reference before deciding scope or writing a new manifest. The goal is
enough evidence to implement the reads correctly, not a long research report.

## Sources and evidence

Start with the company's official API documentation, including any URL supplied
by the user. Read endpoint request and response schemas, not just the product
overview. Use an official OpenAPI specification or documentation index when it
helps enumerate the API. Check versions and relevant deprecation notices.

Official SDK source can clarify serialization or paging. A sanitized live response
can confirm what the service actually returns. Other connectors and community
reports are corroboration, not substitutes for the API contract. A community post
on an official domain is still an observation with limited scope.

Distinguish these kinds of evidence in working notes:

- **Documented:** the endpoint explicitly describes the behavior or schema.
- **Observed:** a response from a known endpoint demonstrates it. Preserve the
  request shape, status, and sanitized response when they affect correctness.
- **Inferred:** behavior is extrapolated from another endpoint or incomplete
  evidence. State the uncertainty and its effect on implementation or live checks.

Never turn a response description such as “No records found” into an exact
error-message literal. Generic examples such as “An error has occurred” do not
prove the body of a specific empty-result response. Resolve conflicts between
prose, schemas, and observed behavior rather than silently choosing one.

If docs require login, look for official public references or specifications.
Report what remains inaccessible. Do not claim endpoint verification from a
search snippet or a similarly named SDK method.

## Coverage matrix

Inventory relevant read endpoints before narrowing the implementation. Record
these details in concise working notes or the existing plan:

| Detail | Questions to resolve |
|---|---|
| Purpose and row | What does one output row represent? Definition, instance, event, aggregate, or snapshot? |
| Request | Exact method, path, required parameters, body encoding, version headers, and field selectors? |
| Response | Array path or singleton object? Are detail fields absent from the listing? |
| Identity | Globally stable ID, parent-scoped ID, documented composite key, or no stable key? |
| Pagination | Cursor path and injection target, termination signal, page size, hard result cap, and ordering? |
| Access | Key type, scopes, roles, plan, workspace visibility, and private-data restrictions? |
| Read mode | Full only, update-based incremental, or append-only events? What changes can be missed? |
| Limits and errors | Rate/size limits, processing delays, empty results, authentication failures, and partial errors? |
| Decision | Included by default, optional, or excluded? Why? Link the endpoint reference. |

Representative coverage includes expected entities and their relationships.
It does not require duplicate basic and detailed listings, every administrative
operation, or every adjacent product API. Definitions differ from completed
records, such as scorecard templates versus submitted scores. Listing webhook
configuration differs from receiving webhook events. Explain those boundaries.

## Connection and access

Verify the base URL, including regional, tenant-specific, or account-provided
hosts. Do not assume a global example host works for every customer. Add host
configuration only when needed. Check version and Accept headers, credential
creation steps, credential lifetime, and scopes.

Keep credentials secret in the config schema. Distinguish personal, workspace,
service, and audit keys when they have different access boundaries. Document
extra permissions or paid features. An empty list may reflect visibility
restrictions, not an empty company account.

The generic connection test sends one request to the first top-level,
non-streaming resource. Choose a suitable endpoint and document its required
access. Static discovery is local and does not prove credentials work. Saving
or validating a config does not perform the explicit live connection test.

## Requests and completeness

Read pagination for each endpoint, even when the product has a general guide.
Check whether the cursor belongs in a query, header, JSON body, or GraphQL
variable. Preserve filters and selectors across pages. Confirm how missing,
null, and empty cursors differ, and whether a short page can still have more data.

Use a documented page size that balances request count and response size. Do
not automatically use the maximum, send an undocumented limit, or add an
arbitrary page cap. A search endpoint's hard result ceiling can make an otherwise
valid paginator incomplete. Prefer an uncapped listing or export endpoint where
it fits the engine. Flag required partitioning as a design gap.

Prefer bulk listings that include the required fields. Add detail reads only
for missing data. If an optional include can make a response too large, look
for a dedicated paginated endpoint. A `413` workaround that drops requested
data is not complete extraction.

Check required date windows and whether omitted bounds mean all history, a
provider default, or an error. Do not invent a start date. Account for content
selectors, retention periods, processing delays, soft deletion, and whether
private or archived records are available.

## Incremental semantics

An upstream lower-bound parameter is necessary but not sufficient. Establish:

- What the cursor measures: creation, event time, or last modification.
- Whether edits, late processing, changing relationships, or newly shared
  records advance that value.
- Whether it is returned on every row and can be projected non-nullably.
- Its units, precision, timezone, comparison rules, and inclusive/exclusive bound.
- Whether the endpoint accepts that lower bound throughout pagination.
- Whether lookback handles a real bounded delay, and what it cannot recover.
- Whether parent listing filters can hide changes to child records.
- Whether deletions are emitted, omitted, or only observable through replacement.

Use full reads when these semantics do not support the promised incremental
behavior. Explain append-only limitations if using an event or creation cursor.
A lookback window does not guarantee capture of arbitrary edits to older records.

## Limits and errors

Confirm the quota owner (key, user, workspace, company) and time window.
Requests-per-second limiting does not enforce a daily allowance shared with
other clients. Use documented headers and existing `Retry-After` handling.
Do not invent headers or add a quota scheduler as part of a manifest.

Inspect empty success responses, malformed JSON, 200-wrapped errors, partial
GraphQL errors, permission failures, throttling, and no-record error statuses.
An empty-result exception needs evidence for that endpoint and a narrow body
match. A `404` can also mean a bad path, missing parent, or invalid workspace.
If the empty case cannot be distinguished reliably, leave the error visible
and describe the limitation. Do not silently skip a core resource.
