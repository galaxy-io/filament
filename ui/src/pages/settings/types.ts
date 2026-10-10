import type { RotateServiceAccountSecretResponse } from "@/gen/auth/v1/service_accounts_pb";
import type { AcceptInviteRequest } from "@/gen/auth/v1/session_pb";

export enum SettingsTeamView {
  INVITE = "INVITE",
  LINK = "LINK",
}

export interface SettingsInviteToken {
  userId: AcceptInviteRequest["userId"];
  code: AcceptInviteRequest["code"];
}

export type ServiceAccountCredentials = Pick<
  RotateServiceAccountSecretResponse,
  "clientId" | "clientSecret"
>;
