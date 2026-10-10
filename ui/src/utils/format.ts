import { EMPTY_VALUE, formatDate } from "@galaxy-io/dls/utils/format";

export const formatTimestamp = (unixMillis: bigint) =>
  unixMillis ? formatDate(unixMillis, { style: "dateTime" }) : EMPTY_VALUE;
