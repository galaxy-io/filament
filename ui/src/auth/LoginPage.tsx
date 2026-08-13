import { useState } from "react";

import { Code, ConnectError } from "@connectrpc/connect";

import PasswordInput from "@galaxy-io/dls/inputs/PasswordInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";

import { useLoginMutation } from "@/api/mutations/auth";

import AuthCard, { AuthCardFooter } from "@/auth/AuthCard";

// LoginPage is filament's own credential screen. The provider's authorize
// endpoint redirects here with ?authRequest=<id>; submitting exchanges the
// credentials for the callback that completes the login.
const LoginPage = () => {
  const authRequestId = new URLSearchParams(window.location.search).get("authRequest") ?? "";
  const [loginName, setLoginName] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | undefined>(undefined);
  const { mutate: login, isPending } = useLoginMutation();

  const handleSubmit = () => {
    if (isPending) {
      return;
    }
    if (loginName === "" || password === "") {
      setError("Enter your email and password");
      return;
    }
    setError(undefined);
    login(
      { authRequestId, loginName, password },
      {
        onSuccess: ({ callbackUrl }) => {
          window.location.href = callbackUrl;
        },
        onError: (err) => {
          setError(
            ConnectError.from(err).code === Code.Unauthenticated
              ? "Incorrect email or password"
              : "Sign-in failed, please try again",
          );
        },
      },
    );
  };

  if (!authRequestId) {
    // Reached directly instead of via the authorize redirect; restart the
    // flow from the app so the provider issues a fresh auth request.
    window.location.replace("/");
    return null;
  }

  return (
    <AuthCard
      subtitle="Sign in with your organization account"
      error={error}
      submitLabel="Sign in"
      pendingLabel="Signing in..."
      isPending={isPending}
      onSubmit={handleSubmit}
      footer={
        <AuthCardFooter prompt="New to filament?" href="/register" label="Create an organization" />
      }
    >
      <TextInput
        label="Email"
        value={loginName}
        onChange={setLoginName}
        placeholder="you@company.com"
        fillWidth
        autoFocus
      />
      <PasswordInput label="Password" value={password} onChange={setPassword} fillWidth />
    </AuthCard>
  );
};

export default LoginPage;
