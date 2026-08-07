import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import { stripDeletedName } from "@/utils/format";

export const formatConnectionName = (connection: Connection, includeDeleted = false): string => {
  const name = includeDeleted ? connection.name : stripDeletedName(connection.name);
  if (name) {
    return name.replace(/->/g, "→");
  }
  return connection.id;
};
