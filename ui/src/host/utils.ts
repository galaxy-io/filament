import { HOST_RUNTIME_CONFIG_ELEMENT_ID } from "@/host/constants";
import { type HostRuntimeConfig, hostRuntimeConfigSchema } from "@/host/schemas";

export const getHostRuntimeConfig = (): HostRuntimeConfig => {
  try {
    return hostRuntimeConfigSchema.parse(
      JSON.parse(document.getElementById(HOST_RUNTIME_CONFIG_ELEMENT_ID)?.textContent ?? "{}"),
    );
  } catch {
    return {};
  }
};
