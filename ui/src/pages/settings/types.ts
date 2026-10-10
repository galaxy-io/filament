import type { RotateServiceAccountSecretResponse } from "@/gen/auth/v1/service_accounts_pb";

export enum SettingsPanel {
  TEAM = "TEAM",
  SERVICE_ACCOUNTS = "SERVICE_ACCOUNTS",
  PREFERENCES = "PREFERENCES",
}

export enum TeamSettingsView {
  MEMBERS = "MEMBERS",
  INVITE = "INVITE",
  LINK = "LINK",
}

export enum SettingsTeamView {
  INVITE = "INVITE",
  LINK = "LINK",
}

export type ServiceAccountCredentials = Pick<
  RotateServiceAccountSecretResponse,
  "clientId" | "clientSecret"
>;
