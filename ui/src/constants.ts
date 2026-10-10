import { RunStatus } from "@/gen/ingestion/v1/runs_pb";

export const IS_DEBUG = process.env.NODE_ENV !== "production";

export const NOOP = () => {};

export const GITHUB_REPO_URL: string = "https://github.com/galaxy-io/filament";
export const DOCUMENTATION_URL: string = "https://filament.getgalaxy.io";
export const SLACK_COMMUNITY_URL: string =
  "https://join.slack.com/t/galaxy-filament/shared_invite/zt-45qln0zg5-9CTvKVXugvh3sEhpD6KzJQ";

export const LIST_SEARCH_DEBOUNCE_MS = 300;
export const VALIDATION_DEBOUNCE_MS = 250;

export const ACTIVE_RUN_STATUSES = new Set<RunStatus>([
  RunStatus.REQUESTED,
  RunStatus.RUNNING,
  RunStatus.PAUSED,
]);
