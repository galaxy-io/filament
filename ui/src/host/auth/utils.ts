import type { GetSessionResponse } from "@/gen/auth/v1/session_pb";

import type { AppSession } from "@/host/auth/types";

export const resolveReturnTo = (returnTo: string | undefined): string =>
  returnTo?.startsWith("/") && !returnTo.startsWith("//") ? returnTo : "/";

export const sessionFromResponse = ({
  userId,
  name,
  email,
  avatarUrl,
}: GetSessionResponse): AppSession => ({ isAuthenticated: true, userId, name, email, avatarUrl });
