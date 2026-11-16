import type { ConfigField } from "@/gen/ingestion/v1/common_pb";
import type { ConnectorSpec, ValidationError } from "@/gen/ingestion/v1/connectors_pb";

import { formatConnectorName } from "@/components/connections/utils";
import { getFieldDefaults } from "@/components/fields/utils";
import { WebhookIngestion } from "@/components/webhooks/types";
import { isWebhookProviderConnector } from "@/components/webhooks/utils";

import {
  ConnectionFormPhase,
  type ConnectionFormState,
} from "@/pages/connections/components/form/types";

export const createInitialConnectionFormState = (
  initialState: Pick<ConnectionFormState, "name" | "config">,
): ConnectionFormState => ({
  name: initialState.name,
  config: initialState.config,
  phase: ConnectionFormPhase.IDLE,
  validationErrors: [],
  shouldShowErrors: false,
});

export const getUnplacedValidationErrors = (
  errors: ValidationError[],
  fields: ConfigField[],
): ValidationError[] => {
  const fieldNames = new Set(fields.map((field) => field.name));
  return errors.filter((error) => !fieldNames.has(error.field.split(".")[0]));
};

export const getConnectionFormConfig = (
  fields: ConfigField[],
  config: ConnectionFormState["config"],
): ConnectionFormState["config"] => {
  const defaults = getFieldDefaults(fields, config);
  return {
    ...config,
    ...Object.fromEntries(
      Object.entries(defaults).map(([name, value]) => [name, config[name] ?? value]),
    ),
  };
};

export const getWebhookConnectionFormConfig = (
  config: ConnectionFormState["config"],
  ingestionField: string | undefined,
): ConnectionFormState["config"] =>
  ingestionField ? { ...config, [ingestionField]: WebhookIngestion.WEBHOOKS } : config;

export const formatConnectionFormConnectorName = (connector: ConnectorSpec, isWebhook: boolean) =>
  isWebhook && isWebhookProviderConnector(connector)
    ? `${formatConnectorName(connector)} webhook`
    : formatConnectorName(connector);
