const DELETED_NAME_SUFFIX = /__deleted__\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$/;

export const stripDeletedName = (name: string) => name.replace(DELETED_NAME_SUFFIX, "");
