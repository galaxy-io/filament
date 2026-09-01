import type { Interceptor, Transport } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";

import { API_URL, IS_DEBUG, IS_PRODUCTION } from "@/constants";

import { getAccessToken } from "@/auth/oidc";

export function createAuthInterceptor(): Interceptor {
  return (next) => (req) => {
    const token = getAccessToken();
    if (token) {
      req.header.set("Authorization", `Bearer ${token}`);
    }
    return next(req);
  };
}

export function createLoggingInterceptor(): Interceptor {
  return (next) => async (req) => {
    if (IS_DEBUG) {
      console.debug("[Connect-RPC] Request:", {
        service: req.service.typeName,
        method: req.method.name,
        url: req.url,
      });
    }

    try {
      const res = await next(req);

      if (IS_DEBUG) {
        console.debug("[Connect-RPC] Response:", {
          service: req.service.typeName,
          method: req.method.name,
          status: "success",
        });
      }

      return res;
    } catch (error) {
      if (IS_DEBUG) {
        console.debug("[Connect-RPC] Error:", {
          service: req.service.typeName,
          method: req.method.name,
          error,
        });
      }
      throw error;
    }
  };
}

export const createTransport = ({
  baseUrl = API_URL,
  interceptors = [createAuthInterceptor(), createLoggingInterceptor()],
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
