import { EMPTY_VALUE, formatDuration } from "@galaxy-io/dls/utils/format";

export const formatRunDuration = (startedAt: bigint, endedAt: bigint) =>
  startedAt && endedAt ? formatDuration(endedAt - startedAt) : EMPTY_VALUE;
