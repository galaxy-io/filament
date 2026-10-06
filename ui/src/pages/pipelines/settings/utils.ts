import cronstrue from "cronstrue";

import { ConnectorKind, ReplicationMode } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import type {
  Pipeline,
  PipelineSchedule,
  PipelineScheduleConfig,
} from "@/gen/ingestion/v1/pipelines_pb";

import {
  PIPELINE_SCHEDULE_CRON_FIELD_BOUNDS,
  PIPELINE_SCHEDULE_CRON_PART_PATTERN,
  PIPELINE_SCHEDULE_DEFAULT_TIMEZONE,
} from "@/pages/pipelines/settings/constants";
import type { PipelineSettingsPageScheduleState } from "@/pages/pipelines/settings/types";

export const isPipelineScheduleCronValid = (cron: PipelineScheduleConfig["cron"]): boolean => {
  const fields = cron.trim().split(/\s+/);
  if (fields.length !== PIPELINE_SCHEDULE_CRON_FIELD_BOUNDS.length) return false;
  return fields.every((field, index) => {
    const [min, max] = PIPELINE_SCHEDULE_CRON_FIELD_BOUNDS[index];
    return field.split(",").every((part) => {
      const match = PIPELINE_SCHEDULE_CRON_PART_PATTERN.exec(part);
      if (!match) return false;
      const [, range, step] = match;
      if (step !== undefined && Number(step) < 1) return false;
      if (range === "*") return true;
      const [lo, hi = lo] = range.split("-").map(Number);
      return lo >= min && hi <= max && lo <= hi;
    });
  });
};

export const formatPipelineScheduleSummary = (
  state: PipelineSettingsPageScheduleState,
): string | null => {
  if (!isPipelineScheduleCronValid(state.cron)) return null;
  try {
    const description = cronstrue.toString(state.cron.trim(), { use24HourTimeFormat: true });
    return `Runs ${description.charAt(0).toLowerCase()}${description.slice(1)} ${state.timezone}`;
  } catch {
    return null;
  }
};

export const hasPipelineScheduleChanges = (
  state: PipelineSettingsPageScheduleState,
  schedule?: PipelineSchedule,
): boolean => {
  if (!schedule?.config) return true;
  return (
    state.isEnabled !== schedule.config.isEnabled ||
    state.cron.trim() !== schedule.config.cron ||
    state.timezone !== (schedule.config.timezone || PIPELINE_SCHEDULE_DEFAULT_TIMEZONE)
  );
};

export const getPipelineCdcSourceConnections = (
  pipeline: Pipeline,
  connections: Connection[],
): Connection[] => {
  const connectionsById = new Map(connections.map((connection) => [connection.id, connection]));
  const cdcConnectionsById = new Map<Connection["id"], Connection>();
  for (const node of pipeline.currentVersion?.graph?.nodes ?? []) {
    if (node.kind !== ConnectorKind.SOURCE) continue;
    const connection = connectionsById.get(node.connectionId);
    if (connection?.replication !== ReplicationMode.CDC) continue;
    cdcConnectionsById.set(connection.id, connection);
  }
  return [...cdcConnectionsById.values()];
};
