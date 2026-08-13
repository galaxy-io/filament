interface InviteToken {
  userId: string;
  code: string;
}

const toBase64Url = (value: string) =>
  btoa(value).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");

const fromBase64Url = (value: string) => {
  const base64 = value.replace(/-/g, "+").replace(/_/g, "/");
  return atob(base64.padEnd(base64.length + ((4 - (base64.length % 4)) % 4), "="));
};

export const encodeInviteToken = ({ userId, code }: InviteToken): string => {
  return toBase64Url(JSON.stringify({ userId, code }));
};

export const decodeInviteToken = (token: string): InviteToken | undefined => {
  try {
    const parsed = JSON.parse(fromBase64Url(token)) as Partial<InviteToken>;
    if (parsed.userId && parsed.code) {
      return { userId: parsed.userId, code: parsed.code };
    }
  } catch {
    try {
      const [userId = "", code = ""] = fromBase64Url(token).split(":");
      return userId && code ? { userId, code } : undefined;
    } catch {
      return undefined;
    }
  }
  return undefined;
};
