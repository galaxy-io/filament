import type { AcceptInviteRequest } from "@/gen/auth/v1/session_pb";

export interface InviteToken {
  userId: AcceptInviteRequest["userId"];
  code: AcceptInviteRequest["code"];
}

export interface AppSession {
  isAuthenticated: boolean;
  userId?: string;
  name?: string;
  email?: string;
  avatarUrl?: string;
}
