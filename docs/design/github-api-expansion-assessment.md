# GitHub API expansion assessment

Assessed September 10, 2026, before implementation. This records the three-resource baseline and the expansion proposal. The subsequent implementation is described in the [GitHub connector documentation](../pages/connectors/sources/github.mdx). Evidence for this assessment includes the original manifest and HTTP runtime, GitHub's official documentation and OpenAPI schema, six public API requests, and the existing targeted connector tests.

**Yes: we can replicate substantially more GitHub resources with the existing HTTP manifest engine.** Start with repository metrics, star history, authorized stargazer/watcher listings, and forks. Then add repository-wide comments and releases. Most endpoint descriptions need only YAML, documentation, and tests. Reliable operation at scale also needs a small, concrete improvement to rate-limit handling.

## Baseline connector

The [manifest](../../connectors/http/manifests/github.yaml) has three resources:

| Resource | Endpoint | Current extraction |
|---|---|---|
| `repositories` | `/orgs/{organization}/repos` | Full read; captures repository name |
| `issues` | `/repos/{organization}/{repository}/issues` | `updated_at` → `since`, with a shared watermark and 300-second overlap |
| `pull_requests` | `/repos/{organization}/{repository}/pulls` | Full read, including closed PRs |

The connector requires an organization and bearer token, uses `per_page=100` and `Link: rel="next"` pagination, pins API version `2022-11-28`, and preserves unprojected fields in `raw`. Consequently, repository star/fork counts are already preserved when returned, but are not typed columns. Issues already include PRs; the `pull_request` marker distinguishes them.

Existing engine features cover the expansion:

- Resource-specific headers, including the star timestamp media type.
- Composite primary keys and nested JSON field projection.
- Repository → child → grandchild traversal, with inherited captures.
- Object responses through `response.cardinality: one`, array envelopes, and disabling pagination.
- Selecting child resources without emitting their parents; required parents still get scanned.
- GraphQL POST bodies and cursor pagination, demonstrated by the Linear manifest.

GitHub discovery is currently static: users can select resource types, but the manifest has no repository-level discovery/filter configuration. The host is fixed to `api.github.com`; personal-account repositories and GitHub Enterprise Server are separate scope extensions.

## Stars, watchers, and growth metrics

GitHub's naming is misleading: `watchers_count` and `watchers` are legacy star counts. Actual watchers are subscribers. Since July 2026, GitHub restricts stargazer and subscriber identity listings to repository admins and collaborators. Its changelog says restricted requests can produce empty responses or 403; our unauthenticated requests produced 401. Do not interpret every empty response as an authoritative empty membership set. The endpoint pages still contain generic public-access language that conflicts with their new restriction notices; validate with the actual token and repository role before enabling replacement snapshots. [Access change](https://github.blog/changelog/2026-06-30-upcoming-access-restrictions-to-public-api-endpoints-and-ui-views/), [watching documentation](https://docs.github.com/en/rest/activity/watching).

All paths below begin with `/repos/{owner}/{repo}`. `repository_id` means the immutable parent repository ID, not its name.

| Proposed resource | Path / shape | Recommended key and extraction | Assessment |
|---|---|---|---|
| Typed repository metrics | Existing repository listing | Existing `id`; project `stargazers_count`, `forks_count`, `open_issues_count`, `size` | Easiest change; no additional requests |
| `repository_details` | Base path; object | `id`; full read; no pagination | Obtain `subscribers_count` and richer repository metadata |
| `stargazers` | `/stargazers`; array | `(repository_id, user_id)`; full replacement | Authorized identities plus `starred_at` using the custom media type |
| `watchers` | `/subscribers`; user array | `(repository_id, user_id)`; full replacement | Authorized current subscribers; no documented subscription timestamp in this listing |
| `star_history` | `/stargazers/history`; weekly objects | `(repository_id, week)`; full read | Aggregate weekly total and seven daily counts; override page size to 30 |
| Optional `star_count` | `/stargazers/count`; `{count}` | `repository_id`; singleton | Useful independently, but redundant if repository metrics are already selected |
| `forks` | `/forks`; repository array | `(repository_id, fork_id)`; full replacement | Fork identities, owner, creation date, and repository metadata |

Star identity and watcher lists accept page/per-page pagination, not a time lower bound. Star history permits at most 30 weeks per page and page numbers through 100; retain `week` as epoch seconds and `days` as JSON. Week/day boundaries are not guaranteed UTC. Do not label this series net growth or assume how removed stars affect historical buckets without verification. [Starring API](https://docs.github.com/en/rest/activity/starring), [forks API](https://docs.github.com/en/rest/repos/forks).

The [official OpenAPI schema](https://github.com/github/rest-api-description/blob/main/descriptions/api.github.com/api.github.com.json) confirms two stargazer response shapes. With `Accept: application/vnd.github.star+json`, mappings are `user.id`, `user.login`, `user.node_id`, `user.html_url`, and `starred_at`; ordinary responses put user fields at the root. It also declares `user` nullable. A keyed identity table must explicitly handle an unidentifiable row rather than silently use a null primary key; check real authorized responses before finalizing this behavior.

The organization listing schema allows `subscribers_count` but does not require it. The repository-detail schema requires it. Our public organization-list sample omitted it and the detail sample included it. Merely adding a nullable watcher-count column to `repositories` therefore does not guarantee coverage. A separate detail resource is the simplest reliable route; a detail join inside the listing is unnecessary.

### Replication semantics

Use the repository ID plus user ID for relationship keys. A user can star or watch several repositories, and repository names can change. Add `repository_id: id` to the existing repository `capture` block while preserving `repository: name` for URLs and readable output.

For stars and watchers, use **full read + replace** to maintain current membership. Full read + upsert leaves removed relationships behind. The [existing replication modes](../pages/guides/concepts/replication-modes.mdx) already support replacement; no deletion-diff feature is needed for this use case. Multi-page scans are not atomic snapshots, so concurrent membership changes may require the next scan to converge.

`starred_at` describes creation of an extant relationship; it does not make the endpoint incremental. Do not add a fictitious `since` parameter, or gate star/watcher scans on repository `updated_at` without a documented guarantee that all relationship changes update it.

For historical membership snapshots, retain an observation time through the pipeline/warehouse snapshot process. The source listing itself does not expose unstar or unsubscribe times. If exact future star removals become a requirement, GitHub's `star` webhook has created/deleted actions, but consuming those requires a webhook ingestion path outside this pull manifest. Polling cannot recover arbitrary changes that occur between runs. [Webhook payloads](https://docs.github.com/en/webhooks/webhook-events-and-payloads#star).

## Additional resource coverage

These are feasibility recommendations based on the documented contracts, not live validation of every endpoint. Paths are relative to `/repos/{owner}/{repo}` unless they start with `/orgs`. Most list endpoints use the existing 100-row/Link defaults. Preserve a `raw` remainder on each resource.

| Resource | Endpoint / records | Key | Sync and implementation notes |
|---|---|---|---|
| Issue comments | `/issues/comments`, root array | `id` | Strong incremental candidate: `updated_at` → `since`; includes PR conversation comments; repository fan-out avoids one request per issue. Keep `issue_url` for joining. [Docs](https://docs.github.com/en/rest/issues/comments) |
| PR review comments | `/pulls/comments`, root array | `id` | Strong incremental candidate: `updated_at` → `since`; keep review ID, PR URL, file path, and line context. [Docs](https://docs.github.com/en/rest/pulls/comments) |
| PR reviews | `/pulls/{pull_number}/reviews`, root array | `id` | Full reads; add PR-number capture. Submitted time is not an update cursor. [Docs](https://docs.github.com/en/rest/pulls/reviews) |
| Labels | `/labels`, root array | `id` | Full replacement; separate catalog complements embedded issue labels. [Docs](https://docs.github.com/en/rest/issues/labels) |
| Milestones | `/milestones`, root array | `id` | Full replacement; explicitly `state=all`. [Docs](https://docs.github.com/en/rest/issues/milestones) |
| Issue events | `/issues/events`, root array | `id` | Repository-wide lifecycle events; full scans, no documented `since`. Prefer this before per-issue timelines. [Docs](https://docs.github.com/en/rest/issues/events) |
| Reactions | `/issues/{issue_number}/reactions`, plus separate comment/review-comment endpoints | `id` within each resource | Full replacement; nested fan-out and matching parent captures. [Docs](https://docs.github.com/en/rest/reactions/reactions) |
| Releases | `/releases`, root array | `id` | Full reads; draft visibility depends on access; releases exclude ordinary tags. [Docs](https://docs.github.com/en/rest/releases/releases) |
| Release assets | `/releases/{release_id}/assets`, root array | `id` | Full reads; capture release ID; exposes mutable download counts. Replicate metadata, not binary downloads. [Docs](https://docs.github.com/en/rest/releases/assets) |
| Branches / tags | `/branches`, `/tags`, root arrays | `(repository_id, name)` | Full replacement; branch tips and tags can change/disappear. [Branches](https://docs.github.com/en/rest/branches/branches), [schema](https://github.com/github/rest-api-description/blob/main/descriptions/api.github.com/api.github.com.json) |
| Commits | `/commits`, root array | `(repository_id, sha)` | `since` exists, but defaults to default-branch history. Backdated commits, merges of old history, force pushes, and empty-repository 409s make a timestamp watermark insufficient for a complete Git replica. [Docs](https://docs.github.com/en/rest/commits/commits) |
| Contributors | `/contributors`, root array | `(repository_id, user_id)` for identified users | Cached aggregate, not authoritative author history. Anonymous contributors need a distinct key strategy; empty repositories can return 204. Handle those before shipping. [Docs](https://docs.github.com/en/rest/repos/repos#list-repository-contributors) |
| Languages | `/languages`, object | `repository_id` | Manifest can retain the language→byte-count map as JSON in one row. Converting arbitrary object keys into rows needs downstream transformation. [Schema](https://github.com/github/rest-api-description/blob/main/descriptions/api.github.com/api.github.com.json) |
| Organization members | `/orgs/{org}/members`, root array | `(organization, user_id)` | Full replacement; complete membership requires appropriate organization access. [Docs](https://docs.github.com/en/rest/orgs/members) |
| Teams / team members | `/orgs/{org}/teams`; `/orgs/{org}/teams/{team_slug}/members` | Team `id`; `(team_id, user_id)` | Full replacement; capture slug and immutable team ID. [Teams](https://docs.github.com/en/rest/teams/teams), [members](https://docs.github.com/en/rest/teams/members) |
| Collaborators | `/collaborators`, root array | `(repository_id, user_id)` | Full replacement; permission-scoped relationship data. Preserve permissions/role name. [Schema](https://github.com/github/rest-api-description/blob/main/descriptions/api.github.com/api.github.com.json) |
| Workflows / workflow runs | `/actions/workflows` → `workflows`; `/actions/runs` → `workflow_runs` | `id` | Full reads fit. `created` filtering is not an update cursor; filtered run searches cap at 1,000 results. Do not ship a naive created-time incremental mode. [Runs](https://docs.github.com/en/rest/actions/workflow-runs) |
| Workflow jobs | `/actions/runs/{run_id}/jobs` → `jobs` | `id` | Capture run ID; `filter=all` includes older executions. Full scans initially; completed state can change while jobs run. [Docs](https://docs.github.com/en/rest/actions/workflow-jobs) |
| Deployments / statuses | `/deployments`; `/deployments/{deployment_id}/statuses`, root arrays | `id` per resource | Full reads and nested capture; timestamps do not imply a supported `since` filter. [Deployments](https://docs.github.com/en/rest/deployments/deployments), [statuses](https://docs.github.com/en/rest/deployments/statuses) |
| Projects / fields / items | `/orgs/{org}/projectsV2`, then `/{project_number}/fields` and `/items` | Project `id`; parent-scoped field/item IDs | Current Projects has REST endpoints; GraphQL is not mandatory. Cursor-bearing Link URLs fit the engine. Items default to title only; complete custom-field values require an explicit field-selection design. [Projects](https://docs.github.com/en/rest/projects/projects), [items](https://docs.github.com/en/rest/projects/items) |
| Discussions / comments / replies | `POST /graphql` | GraphQL node ID | Possible using existing body/cursor support, with separate paginated connections and 200-response error detection. More query design and permission testing than REST additions. [Docs](https://docs.github.com/en/graphql/guides/using-the-graphql-api-for-discussions) |
| Security alerts | `/dependabot/alerts`, `/code-scanning/alerts`, `/secret-scanning/alerts` | `(repository_id, number)` per family | Expressible, but feature/access dependent. Separate opt-in scope; retain all relevant states and verify pagination independently. [Code scanning](https://docs.github.com/en/rest/code-scanning/code-scanning), [secret scanning](https://docs.github.com/en/rest/secret-scanning/secret-scanning), [schema](https://github.com/github/rest-api-description/blob/main/descriptions/api.github.com/api.github.com.json) |

For grandchildren, a parent used solely to enumerate children must have a complete walk. In particular, adding comment/reaction children beneath incremental issues can miss activity on parents absent from that issue scan. Repository-wide comments avoid that problem; use a `capture_only` full parent resource where a separate complete enumeration is necessary.

### Traffic and less straightforward APIs

Views and clones are useful growth resources: `/traffic/views` → `views` and `/traffic/clones` → `clones`, keyed by `(repository_id, timestamp)`. Read with `per=day` and no pagination. GitHub exposes only the last 14 days, so recurring full reads with **upsert** preserve older daily observations and refresh recent buckets. Default full replacement would erase accumulated history. Access requires repository write access and the documented Administration-read token permission. Popular paths/referrers expose only the top 10 over 14 days; snapshot them with an observation date if historical comparisons matter. These are aggregates, not visitor identities. [Traffic API](https://docs.github.com/en/rest/metrics/traffic).

Repository statistics are not uniformly ready for YAML-only replication. `/stats/*` can return 202 while computation runs, and some return arrays of numeric tuples. Our engine neither polls 202 to completion nor turns tuple/scalar array elements into records. Some statistics have large-repository limitations; code-frequency statistics reject repositories with 10,000 or more commits. [Statistics API](https://docs.github.com/en/rest/metrics/statistics).

The Events API is a recent activity feed, not a durable change log: timelines are limited to 300 events and 30 days, with delivery latency. It cannot establish complete historical membership or replace the object APIs. [Events API](https://docs.github.com/en/rest/activity/events).

Per-issue timelines contain heterogeneous event shapes, so inspect event-specific identities before choosing a universal `id` key. PR file lists have an API ceiling of 3,000 files; no manifest pagination change can recover records beyond a provider ceiling. These are later additions with explicit completeness limits. [Timeline API](https://docs.github.com/en/rest/issues/timeline), [PR files](https://docs.github.com/en/rest/pulls/pulls#list-pull-requests-files).

## Runtime and operational work

1. **Recognize GitHub throttling correctly.** [doRequest](../../connectors/http/paginate.go) retries 429/5xx but fails all other 4xx. GitHub can signal throttling with 403 or 429. Retry only identified throttling responses; permission-denied 403s must remain errors. Honor `Retry-After` and exhausted-budget `X-RateLimit-Reset`. The current 60-second cap can shorten a server-directed delay, and absent headers produce retries sooner than GitHub's secondary-limit guidance. [Rate-limit guidance](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api).

2. **Use the existing limiter, understanding its implementation.** The GitHub manifest configures none. [DynamicLimiter](../../connectors/http/request/ratelimit.go) can pause after remaining budget reaches zero using Unix reset headers; despite its comments, it does not continuously adapt RPS to remaining budget. Adding that YAML configuration helps, but does not retry the already-failed 403 or fully address secondary limits. A conservative static pace is also available; choose it from expected volume and token budget rather than adding a new configuration surface by default.

3. **Account for token lifetime.** Static bearer auth does not mint or renew GitHub App installation tokens, which expire after one hour. Long scans and scheduled runs need an external token refresh/reconfiguration arrangement or a dedicated App-auth implementation. PATs remain the simpler existing path, subject to their configured expiry and organization access. [Installation tokens](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-an-installation-access-token-for-a-github-app).

4. **Keep permissions attached to selected resources.** Metadata access does not authorize every API family. Comments need Issues/Pull-requests read, code/release resources generally need Contents read, Actions need Actions read, deployments need Deployments read, memberships need Members read, Projects need Projects read, and traffic/security have their own requirements. Endpoint role restrictions apply in addition to token scopes. Today, a failed parent request cancels that resource's sibling fan-out and ultimately fails extraction; there is no YAML policy for skipping unavailable repositories. Avoid globally swallowing 403/404 as empty data.

5. **Handle documented empty responses before adding affected resources.** `doRequest` treats 204 as success, but the response extractor then rejects the empty body as invalid JSON. Contributors are a concrete example. This is separate from unauthorized empty lists and requires status-aware handling, not blanket invalid-JSON suppression. Async 202 is also distinct from empty success.

6. **Keep incremental guarantees modest.** New comment resources can reuse the issue pattern with separate checkpoint keys and overlap. Watermarks are shared across repositories; the existing five-minute overlap reduces exposure to changes during scanning but is not a proof against arbitrarily long scans or late visibility. Child pagination restarts after interruption. Durable per-parent progress or watermark bounds tied to scan start should be considered only if measured scale makes the current approach inadequate.

For cost estimation, a paginated resource requires roughly `sum(max(1, ceil(rows_in_repo / page_size)))` requests, plus parent discovery. One repository with 100,000 stars requires roughly 1,000 identity-list requests per refresh at 100/page. That is about 20% of a typical 5,000-request authenticated hourly budget, before other workloads. Aggregate counts avoid this cost. GitHub App budgets vary; the request budget is shared with other consumers of the credentials. [Limits](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api).

## Recommended implementation sequence

1. Project existing star/fork counts; capture repository IDs. Add `repository_details`, `star_history`, `stargazers`, `watchers`, and `forks`. Validate identity access with the intended token. Use full replacement for current relationship tables. Keep the current API version pin.
2. Address recognized 403 throttling and server-directed delays before broad scans. Add exact header/response/pagination fixtures for the new resources, including multiple repositories sharing the same user, and verify removal through replacement plus empty-snapshot behavior.
3. Add `issue_comments`, `pull_request_review_comments`, `releases`, `release_assets`, labels, and milestones. These provide substantial coverage with familiar manifest patterns; comments are the clearest new incremental resources.
4. Add traffic when growth reporting needs it, with explicit full-upsert history preservation. Add reviews/reactions, organization membership, Actions, Projects, and Discussions according to actual reporting needs. Defer async statistics and complete Git-history replication until their runtime requirements are justified.

The API-version pin does not need to change for the first phase. GitHub lists `2022-11-28` as supported until March 10, 2028; additive endpoints are made available to supported versions. The live aggregate-star calls below worked under that pin. Access restrictions can still change outside the normal version cadence, so the pin does not restore public identity access. [Version policy](https://docs.github.com/en/rest/about-the-rest-api/api-versions).

## Verification performed and remaining uncertainty

Public requests on September 10, 2026 used `X-GitHub-Api-Version: 2022-11-28` and no credentials:

| Request | Result |
|---|---|
| `octocat/Hello-World` stargazers, `per_page=1`, timestamp media type | 401, authentication required |
| Same repository subscribers, `per_page=1` | 401, authentication required |
| Same repository star count | 200, `{count}` |
| Same repository star history, `per_page=1` | 200, array containing `week`, `total`, `days`; Link pagination present |
| `GET /orgs/github/repos?per_page=1` | 200; star/fork counts present, subscriber count absent |
| `GET /repos/octocat/Hello-World` | 200; `subscribers_count` present; `watchers_count` equaled `stargazers_count` |

Downloaded and inspected GitHub's current OpenAPI schema to check response shapes, query parameters, keys, and special status codes. Existing targeted tests passed:

```text
go test ./connectors/http -run 'TestManifests|TestNewGitHubSpecAndEmbeddedManifest' -count=1
ok github.com/galaxy-io/filament/connectors/http
```

The existing GitHub-specific test checks configuration and three-resource discovery; it does not prove authenticated extraction. Authorized stargazer/watcher payloads, null-user behavior, private repositories, mixed permissions, token renewal, rate-limit recovery, and replacement into a real destination remain implementation validation tasks. No shipped manifest or runtime code was changed.
