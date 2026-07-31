export const formatCount = (value: bigint): string => {
  return Number(value).toLocaleString();
};

export const formatTimeAgo = (unixMillis: bigint): string => {
  const elapsedMs = Date.now() - Number(unixMillis);
  const minutes = Math.floor(elapsedMs / 60_000);
  if (minutes < 1) return "just now";
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d ago`;
  return new Date(Number(unixMillis)).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
  });
};

export const formatTimeUntil = (unixMillis: bigint): string => {
  if (!unixMillis) return "—";
  const remainingMs = Number(unixMillis) - Date.now();
  const minutes = Math.floor(remainingMs / 60_000);
  if (minutes < 1) return "in <1m";
  if (minutes < 60) return `in ${minutes}m`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `in ${hours}h`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `in ${days}d`;
  return new Date(Number(unixMillis)).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
  });
};

export const formatTimestamp = (unixMillis: bigint): string => {
  if (!unixMillis) return "—";
  return new Date(Number(unixMillis)).toLocaleString("en-US", {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
    second: "2-digit",
  });
};

export const formatDuration = (startMillis: bigint, endMillis: bigint): string => {
  if (!startMillis || !endMillis) return "—";
  const elapsedMs = Number(endMillis - startMillis);
  if (elapsedMs < 1_000) return `${elapsedMs}ms`;
  const seconds = elapsedMs / 1_000;
  if (seconds < 60) return `${seconds.toFixed(1)}s`;
  const minutes = Math.floor(seconds / 60);
  return `${minutes}m ${Math.round(seconds % 60)}s`;
};

const BYTE_UNITS = ["B", "KB", "MB", "GB", "TB"];

export const formatBytes = (value: bigint): string => {
  let scaled = Number(value);
  let unitIndex = 0;
  while (scaled >= 1024 && unitIndex < BYTE_UNITS.length - 1) {
    scaled /= 1024;
    unitIndex += 1;
  }
  return `${unitIndex === 0 ? scaled : scaled.toFixed(1)} ${BYTE_UNITS[unitIndex]}`;
};
