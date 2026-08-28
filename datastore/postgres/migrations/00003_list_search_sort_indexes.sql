-- +goose Up
-- B-tree indexes cover the common list orders. GIN and PostgreSQL's built-in
-- full-text search provide word-oriented search without any extension.
CREATE INDEX connections_list_name_idx
  ON connections (tenant_id, kind, lower(name), id)
  WHERE NOT is_deleted;

CREATE INDEX connections_search_idx
  ON connections USING gin (
    to_tsvector('simple', coalesce(name, '') || ' ' || coalesce(connector, ''))
  );

CREATE INDEX pipelines_list_name_idx
  ON pipelines (tenant_id, lower(name), id)
  WHERE NOT is_deleted;

CREATE INDEX pipelines_search_idx
  ON pipelines USING gin (
    to_tsvector('simple', coalesce(name, '') || ' ' || coalesce(description, ''))
  );

CREATE INDEX pipeline_versions_created_idx
  ON pipeline_versions (pipeline_id, created_at, id);

CREATE INDEX runs_tenant_started_idx
  ON runs (tenant_id, started_at DESC NULLS FIRST, id DESC);

CREATE INDEX runs_pipeline_started_idx
  ON runs (pipeline_id, started_at DESC NULLS FIRST, id DESC);

-- +goose Down
DROP INDEX runs_pipeline_started_idx;
DROP INDEX runs_tenant_started_idx;
DROP INDEX pipeline_versions_created_idx;
DROP INDEX pipelines_search_idx;
DROP INDEX pipelines_list_name_idx;
DROP INDEX connections_search_idx;
DROP INDEX connections_list_name_idx;
