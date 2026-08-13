import { API_URL } from "@/constants";

// AuthConfig mirrors the server's GET /auth/config. An empty issuer means
// auth is disabled and the app renders without a session.
export interface AuthConfig {
  issuer: string;
  clientId: string;
}

export const fetchAuthConfig = async (): Promise<AuthConfig> => {
  const res = await fetch(`${API_URL}/auth/config`);
  if (!res.ok) {
    throw new Error(`auth config: ${res.status}`);
  }
  return (await res.json()) as AuthConfig;
};
