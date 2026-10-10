import { create } from "@bufbuild/protobuf";
import pluralize from "pluralize";

import type { SelectOption } from "@galaxy-io/dls/inputs/SelectInput";

import {
  NotificationType,
  type Notifier,
  type NotifierInput,
  NotifierInputSchema,
} from "@/gen/ingestion/v1/notifiers_pb";

import {
  PIPELINE_NOTIFIER_ALL_EVENTS_LABEL,
  PIPELINE_NOTIFIER_EVENTS,
  PIPELINE_NOTIFIER_SLACK_URL_PREFIXES,
  PIPELINE_NOTIFIER_URL_SECRET_REF_KEY,
} from "@/pages/pipelines/components/notifier/constants";
import type { PipelineNotifierState } from "@/pages/pipelines/components/notifier/types";

import { isNameValid } from "@/utils/validation";

export const formatPipelineNotifierEventsSelection = (options: SelectOption[]): string => {
  if (options.length === 1) return options[0].label;
  if (options.length === PIPELINE_NOTIFIER_EVENTS.length) return PIPELINE_NOTIFIER_ALL_EVENTS_LABEL;
  return pluralize("event", options.length, true);
};

export const isPipelineNotifierUrlValid = (url: string): boolean => {
  try {
    const { protocol } = new URL(url.trim());
    return protocol === "http:" || protocol === "https:";
  } catch {
    return false;
  }
};

export const isPipelineNotifierSlackUrlValid = (url: string): boolean => {
  const trimmed = url.trim();
  return PIPELINE_NOTIFIER_SLACK_URL_PREFIXES.some(
    (prefix) => trimmed.startsWith(prefix) && trimmed.length > prefix.length,
  );
};

export const hasPipelineNotifierStoredUrl = (state: PipelineNotifierState): boolean =>
  PIPELINE_NOTIFIER_URL_SECRET_REF_KEY in state.secretRefs;

export const parsePipelineNotifierHeaders = (text: string): Record<string, string> | null => {
  if (text.trim() === "") return {};
  try {
    const parsed: unknown = JSON.parse(text);
    if (parsed === null || typeof parsed !== "object" || Array.isArray(parsed)) return null;
    const headers = parsed as Record<string, unknown>;
    return Object.values(headers).every((value) => typeof value === "string")
      ? (headers as Record<string, string>)
      : null;
  } catch {
    return null;
  }
};

const isPipelineNotifierSlackValid = (state: PipelineNotifierState): boolean =>
  state.url.trim() === ""
    ? hasPipelineNotifierStoredUrl(state)
    : isPipelineNotifierSlackUrlValid(state.url);

const isPipelineNotifierWebhookValid = (state: PipelineNotifierState): boolean =>
  isPipelineNotifierUrlValid(state.url) && parsePipelineNotifierHeaders(state.headers) !== null;

export const isPipelineNotifierValid = (state: PipelineNotifierState): boolean =>
  isNameValid(state.name) &&
  state.events.length > 0 &&
  (state.notificationType === NotificationType.SLACK
    ? isPipelineNotifierSlackValid(state)
    : isPipelineNotifierWebhookValid(state));

const mapPipelineNotifierStateToConfig = (
  state: PipelineNotifierState,
): NotifierInput["config"] => {
  const url = state.url.trim();
  if (state.notificationType === NotificationType.SLACK) return url === "" ? {} : { url };

  const headers = parsePipelineNotifierHeaders(state.headers);
  return {
    url,
    ...(state.headers.trim() !== "" && headers ? { headers } : {}),
  };
};

export const mapPipelineNotifierStateToInput = (state: PipelineNotifierState): NotifierInput =>
  create(NotifierInputSchema, {
    name: state.name.trim(),
    notificationType: state.notificationType,
    isEnabled: state.isEnabled,
    events: state.events,
    config: mapPipelineNotifierStateToConfig(state),
    secretRefs: state.secretRefs,
  });

export const mapNotifierToPipelineNotifierState = (notifier: Notifier): PipelineNotifierState => ({
  name: notifier.name,
  notificationType: notifier.notificationType,
  isEnabled: notifier.isEnabled,
  events: notifier.events,
  url: typeof notifier.config?.url === "string" ? notifier.config.url : "",
  headers: "",
  secretRefs: notifier.secretRefs,
});
