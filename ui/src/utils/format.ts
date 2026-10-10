import { EMPTY_VALUE, formatDate } from "@galaxy-io/dls/utils/format";

export const formatTimestamp = (unixMillis: bigint) =>
  unixMillis ? formatDate(unixMillis, { style: "dateTime" }) : EMPTY_VALUE;

export const formatVersion = (version: bigint | number | undefined) =>
  version ? `Version ${version}` : EMPTY_VALUE;
