import { create } from "@bufbuild/protobuf";
import pluralize from "pluralize";

import type { SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";

import {
  type Notifier,
  type NotifierInput,
  NotifierInputSchema,
} from "@/gen/ingestion/v1/notifiers_pb";

import { isNameValid } from "@/pages/connectors/components/form/validation";
import {
  PIPELINE_NOTIFIER_ALL_EVENTS_PINNED_OPTION,
  PIPELINE_NOTIFIER_EVENTS,
} from "@/pages/pipelines/components/notifier/constants";
import type { PipelineNotifierState } from "@/pages/pipelines/components/notifier/types";

export const formatPipelineNotifierEventsSelection = (
  selectedOptions: SelectInputOption[],
  placeholder: string,
): string => {
  if (selectedOptions.length === 0) return placeholder;
  if (selectedOptions.length === 1) return selectedOptions[0].label;
  if (selectedOptions.length === PIPELINE_NOTIFIER_EVENTS.length) {
    return PIPELINE_NOTIFIER_ALL_EVENTS_PINNED_OPTION.label;
  }
  return pluralize("event", selectedOptions.length, true);
};

export const isPipelineNotifierUrlValid = (url: string): boolean => {
  try {
    const { protocol } = new URL(url.trim());
    return protocol === "http:" || protocol === "https:";
  } catch {
    return false;
  }
};

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

export const isPipelineNotifierValid = (state: PipelineNotifierState): boolean =>
  isNameValid(state.name) &&
  state.events.length > 0 &&
  isPipelineNotifierUrlValid(state.url) &&
  parsePipelineNotifierHeaders(state.headers) !== null;

export const mapPipelineNotifierStateToInput = (state: PipelineNotifierState): NotifierInput => {
  const headers = parsePipelineNotifierHeaders(state.headers);
  return create(NotifierInputSchema, {
    name: state.name.trim(),
    notificationType: state.notificationType,
    isEnabled: state.isEnabled,
    events: state.events,
    config: {
      url: state.url.trim(),
      ...(state.headers.trim() !== "" && headers ? { headers } : {}),
    },
    secretRefs: state.secretRefs,
  });
};

export const mapNotifierToPipelineNotifierState = (notifier: Notifier): PipelineNotifierState => ({
  name: notifier.name,
  notificationType: notifier.notificationType,
  isEnabled: notifier.isEnabled,
  events: notifier.events,
  url: typeof notifier.config?.url === "string" ? notifier.config.url : "",
  headers: "",
  secretRefs: notifier.secretRefs,
});
