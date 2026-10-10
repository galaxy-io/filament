export interface AppSession {
  isAuthenticated: boolean;
  userId?: string;
  name?: string;
  email?: string;
  avatarUrl?: string;
}
