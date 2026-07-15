import { PipelineGroup, PipelineHealth, PipelineListItem } from "@/pages/pipelines/types";

// Layout constants
export const PIPELINE_CARD_HEIGHT = 48;
export const PIPELINE_GROUP_BAND_HEIGHT = 40;
export const PIPELINE_INDICATOR_WIDTH = 16;
export const PIPELINE_SEARCH_WIDTH = 280;
export const PIPELINE_MAX_VISIBLE_SINKS = 2;

// Maps
export const PIPELINE_GROUP_TO_LABEL_MAP: Record<PipelineGroup, string> = {
  [PipelineGroup.ACTIVE]: "Active",
  [PipelineGroup.NEEDS_ATTENTION]: "Needs attention",
  [PipelineGroup.PAUSED]: "Paused",
};

export const PIPELINE_METRIC_COLUMN_WIDTH_MAP = {
  connectors: 160,
  lastRun: 120,
  volume: 120,
  schedule: 120,
} as const;

/**
 * Placeholder data shown until the backend exposes run metadata for the list
 * view. Mirrors the v1 design mocks.
 */
export const MOCK_PIPELINE_ITEMS: PipelineListItem[] = [
  {
    id: "pl_notion_pages",
    name: "Notion Pages Sync",
    health: PipelineHealth.HEALTHY,
    source: "notion",
    sinks: ["bigquery", "redshift"],
    lastRunLabel: "14:29:44",
    volumeLabel: "1.2M rows",
    scheduleLabel: "0 * * * *",
    isEnabled: true,
  },
  {
    id: "pl_slack_users",
    name: "Slack Users Sync",
    health: PipelineHealth.HEALTHY,
    source: "slack",
    sinks: ["snowflake", "s3", "redshift"],
    lastRunLabel: "14:32:07",
    volumeLabel: "1.2M rows",
    scheduleLabel: "*/15 * * * *",
    isEnabled: true,
  },
  {
    id: "pl_github_repos",
    name: "GitHub Repos Sync",
    health: PipelineHealth.HEALTHY,
    source: "github",
    sinks: ["s3", "iceberg", "bigquery", "snowflake"],
    lastRunLabel: "14:08:19",
    volumeLabel: "402k rows",
    scheduleLabel: "0 * * * *",
    isEnabled: true,
  },
  {
    id: "pl_postgres_cdc",
    name: "Postgres CDC Stream",
    health: PipelineHealth.HEALTHY,
    source: "postgres",
    sinks: ["snowflake", "iceberg", "s3"],
    lastRunLabel: "now",
    volumeLabel: "512k rows",
    scheduleLabel: "*/5 * * * *",
    isEnabled: true,
  },
  {
    id: "pl_hubspot_contacts",
    name: "HubSpot Contacts Sync",
    health: PipelineHealth.HEALTHY,
    source: "hubspot",
    sinks: ["bigquery", "s3"],
    lastRunLabel: "13:58:11",
    volumeLabel: "220k rows",
    scheduleLabel: "*/30 * * * *",
    isEnabled: true,
  },
  {
    id: "pl_googleads_spend",
    name: "GoogleAds Spend Sync",
    health: PipelineHealth.HEALTHY,
    source: "googleads",
    sinks: ["bigquery"],
    lastRunLabel: "06:00:02",
    volumeLabel: "8.4k rows",
    scheduleLabel: "0 6 * * *",
    isEnabled: true,
  },
  {
    id: "pl_mysql_orders",
    name: "MySQL Orders Sync",
    health: PipelineHealth.HEALTHY,
    source: "mysql",
    sinks: ["redshift", "s3", "snowflake"],
    lastRunLabel: "14:30:12",
    volumeLabel: "2.4M rows",
    scheduleLabel: "*/10 * * * *",
    isEnabled: true,
  },
  {
    id: "pl_jira_projects",
    name: "Jira Projects Sync",
    health: PipelineHealth.DEGRADED,
    source: "jira",
    sinks: ["redshift", "s3"],
    lastRunLabel: "14:21:58",
    volumeLabel: "89k rows",
    scheduleLabel: "*/15 * * * *",
    isEnabled: true,
  },
  {
    id: "pl_workday_people",
    name: "Workday People Sync",
    health: PipelineHealth.FAILING,
    source: "workday",
    sinks: ["snowflake"],
    lastRunLabel: "14:15:02",
    volumeLabel: "—",
    scheduleLabel: "0 6 * * *",
    isEnabled: true,
  },
  {
    id: "pl_salesforce_accounts",
    name: "Salesforce Accounts Sync",
    health: PipelineHealth.PAUSED,
    source: "salesforce",
    sinks: ["snowflake"],
    lastRunLabel: "—",
    volumeLabel: "—",
    scheduleLabel: "0 6 * * *",
    isEnabled: false,
  },
  {
    id: "pl_linear_issues",
    name: "Linear Issues Sync",
    health: PipelineHealth.PAUSED,
    source: "linear",
    sinks: ["bigquery"],
    lastRunLabel: "—",
    volumeLabel: "—",
    scheduleLabel: "0 * * * *",
    isEnabled: false,
  },
];
