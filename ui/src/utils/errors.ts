import { Code, ConnectError } from "@connectrpc/connect";

export const getErrorMessage = (error: unknown, fallback: string) => {
  if (error instanceof ConnectError) return error.rawMessage;
  return error instanceof Error ? error.message : fallback;
};

export const isNotFoundError = (error: unknown) => ConnectError.from(error).code === Code.NotFound;
