export const mapRecordValues = <K extends PropertyKey, V, T>(
  record: Record<K, V>,
  map: (value: V) => T,
): Record<K, T> =>
  Object.fromEntries(
    (Object.entries(record) as [string, V][]).map(([key, value]) => [key, map(value)]),
  ) as Record<K, T>;
