import { TextVariant } from "@galaxy-io/dls/text/Text";

import type { RunEvent } from "@/gen/ingestion/v1/runs_pb";

export const formatRunEventTime = (event: RunEvent): string => {
  const date = new Date(Number(event.createdAt));
  return date.toLocaleTimeString("en-US", { hour12: false });
};

export const formatRunEventDetail = (event: RunEvent): string => {
  const parts: string[] = [];

  if (event.resource) {
    parts.push(event.resource);
  }

  const fields = event.fields;
  if (fields) {
    if (fields.records > 0n) {
      parts.push(`records=${fields.records}`);
    }
    if (fields.bytes > 0n) {
      parts.push(`bytes=${fields.bytes}`);
    }
    if (fields.uri) {
      parts.push(fields.uri);
    }
    if (fields.error) {
      parts.push(fields.error);
    }
  }

  return parts.join(" ");
};

export const getRunEventTextVariant = (event: RunEvent): TextVariant => {
  if (event.eventType.endsWith(".failed") || event.fields?.error) {
    return TextVariant.ERROR;
  }
  if (event.eventType === "run.completed") {
    return TextVariant.SUCCESS;
  }
  if (event.isReplay) {
    return TextVariant.TERTIARY;
  }
  return TextVariant.SECONDARY;
};
