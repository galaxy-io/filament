import { type PropsWithChildren, useEffect, useState } from "react";

import { BugIcon } from "@phosphor-icons/react";
import { AuthProvider, useAuth } from "react-oidc-context";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";

import ErrorLayout from "@/layouts/ErrorLayout";
import PendingLayout from "@/layouts/PendingLayout";

import { type AuthConfig, fetchAuthConfig } from "@/auth/config";
import InvitePage from "@/auth/InvitePage";
import LoginPage from "@/auth/LoginPage";
import RegisterPage from "@/auth/RegisterPage";
import { setAccessTokenGetter, setProfileGetter } from "@/auth/token";

// Zitadel returns the user's org (the tenant) only when this scope is
// requested; offline_access adds the refresh token silent renew needs.
const SCOPE = "openid profile email offline_access urn:zitadel:iam:user:resourceowner";

// AppAuthProvider fetches /auth/config before first render. Auth disabled
// renders the app untouched; enabled wraps it in the OIDC session gate.
const AppAuthProvider = ({ children }: PropsWithChildren) => {
  const [config, setConfig] = useState<AuthConfig | undefined>(undefined);
  const [error, setError] = useState<Error | undefined>(undefined);

  useEffect(() => {
    fetchAuthConfig().then(setConfig).catch(setError);
  }, []);

  if (error) {
    return (
      <ErrorLayout
        icon={<Icon component={BugIcon} size={24} variant={IconVariant.ERROR} />}
        header="Could not reach the server"
        message="Please try again later"
        error={error}
      />
    );
  }
  if (!config) {
    return <PendingLayout />;
  }
  if (!config.issuer) {
    return children;
  }
  // Zitadel's authorize endpoint redirects here for credentials; the login
  // and signup pages live outside the session gate by definition.
  if (window.location.pathname === "/login") {
    return <LoginPage />;
  }
  if (window.location.pathname === "/register") {
    return <RegisterPage />;
  }
  if (window.location.pathname.startsWith("/invite")) {
    return <InvitePage />;
  }
  return (
    <AuthProvider
      authority={config.issuer}
      client_id={config.clientId}
      redirect_uri={`${window.location.origin}/auth/callback`}
      post_logout_redirect_uri={window.location.origin}
      scope={SCOPE}
      automaticSilentRenew
      onSigninCallback={() => {
        window.history.replaceState({}, "", "/");
      }}
    >
      <SessionGate>{children}</SessionGate>
    </AuthProvider>
  );
};

// SessionGate blocks rendering until a session exists, redirecting to the
// hosted login when there is none, and hands the access token to the RPC
// transport.
const SessionGate = ({ children }: PropsWithChildren) => {
  const auth = useAuth();

  useEffect(() => {
    setAccessTokenGetter(() => auth.user?.access_token);
    setProfileGetter(() => {
      const profile = auth.user?.profile;
      return profile ? { sub: profile.sub, name: profile.name, email: profile.email } : undefined;
    });
    return () => {
      setAccessTokenGetter(null);
      setProfileGetter(null);
    };
  }, [auth.user]);

  useEffect(() => {
    if (!auth.isLoading && !auth.isAuthenticated && !auth.activeNavigator && !auth.error) {
      void auth.signinRedirect();
    }
  }, [auth]);

  if (auth.error) {
    return (
      <ErrorLayout
        icon={<Icon component={BugIcon} size={24} variant={IconVariant.ERROR} />}
        header="Sign-in failed"
        message="Please try again later"
        error={auth.error}
      />
    );
  }
  if (!auth.isAuthenticated) {
    return <PendingLayout />;
  }
  return children;
};

export default AppAuthProvider;
