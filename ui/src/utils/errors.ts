import { ConnectError } from "@connectrpc/connect";

export const getErrorMessage = (error: unknown, fallback: string) => {
  if (error instanceof ConnectError) return error.rawMessage;
  return error instanceof Error ? error.message : fallback;
};
