import type { Interceptor, Transport } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";

import { IS_DEBUG } from "@/constants";

const API_URL: string = import.meta.env.VITE_API_URL ?? "";
const IS_PRODUCTION = import.meta.env.PROD;

const createLoggingInterceptor = (): Interceptor => {
  return (next) => async (req) => {
    console.debug("[Connect-RPC] Request:", {
      service: req.service.typeName,
      method: req.method.name,
      url: req.url,
    });

    try {
      const res = await next(req);
      console.debug("[Connect-RPC] Response:", {
        service: req.service.typeName,
        method: req.method.name,
        status: "success",
      });
      return res;
    } catch (error) {
      console.debug("[Connect-RPC] Error:", {
        service: req.service.typeName,
        method: req.method.name,
        error,
      });
      throw error;
    }
  };
};

const DEFAULT_INTERCEPTORS = IS_DEBUG ? [createLoggingInterceptor()] : [];

export const createTransport = ({
  baseUrl = API_URL,
  interceptors = DEFAULT_INTERCEPTORS,
  useBinaryFormat = IS_PRODUCTION,
}: {
  baseUrl?: string;
  interceptors?: Interceptor[];
  useBinaryFormat?: boolean;
} = {}): Transport => {
  return createConnectTransport({
    baseUrl,
    useBinaryFormat,
    interceptors,
  });
};

export const transport = createTransport();
