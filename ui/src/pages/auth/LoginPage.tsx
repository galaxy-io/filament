import { useState } from "react";

import { Code, ConnectError } from "@connectrpc/connect";
import { useNavigate, useSearch } from "@tanstack/react-router";

import { InputSize } from "@galaxy-io/dls/inputs/Input";
import PasswordInput from "@galaxy-io/dls/inputs/PasswordInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";

import type { LoginRequest } from "@/gen/auth/v1/session_pb";

import AuthForm, { AuthFormFooter } from "@/layouts/auth/AuthForm";

import { useLoginMutation } from "@/api/mutations/auth";

import { resolveReturnTo } from "@/auth/utils";
import { getErrorMessage } from "@/utils/errors";

interface LoginPageState {
  loginName: LoginRequest["loginName"];
  password: LoginRequest["password"];
  error: string | undefined;
}

const DEFAULT_STATE: LoginPageState = {
  loginName: "",
  password: "",
  error: undefined,
};

const LoginPage = () => {
  const { returnTo } = useSearch({ from: "/login" });
  const navigate = useNavigate();
  const [state, setState] = useState<LoginPageState>(DEFAULT_STATE);
  const { mutate: login, isPending } = useLoginMutation();

  const handleSubmit = () => {
    if (state.loginName === "" || state.password === "") {
      setState((prev) => ({ ...prev, error: "Enter your email and password" }));
      return;
    }
    setState((prev) => ({ ...prev, error: undefined }));
    login(
      { loginName: state.loginName, password: state.password },
      {
        onSuccess: () => {
          void navigate({ href: resolveReturnTo(returnTo), replace: true });
        },
        onError: (err) => {
          setState((prev) => ({
            ...prev,
            error:
              ConnectError.from(err).code === Code.Unauthenticated
                ? "Incorrect email or password"
                : getErrorMessage(err, "Sign-in failed, please try again"),
          }));
        },
      },
    );
  };

  return (
    <AuthForm
      title="Sign in"
      subtitle="Continue with your organization account."
      error={state.error}
      submitLabel="Sign in"
      isPending={isPending}
      onSubmit={handleSubmit}
      footer={
        <AuthFormFooter prompt="New to Filament?" to="/register" label="Create an organization" />
      }
    >
      <TextInput
        label="Email"
        value={state.loginName}
        onChange={(loginName) => setState((prev) => ({ ...prev, loginName }))}
        placeholder="hello@getgalaxy.io"
        size={InputSize.LARGE}
        fillWidth
        autoFocus
      />
      <PasswordInput
        label="Password"
        value={state.password}
        onChange={(password) => setState((prev) => ({ ...prev, password }))}
        size={InputSize.LARGE}
        fillWidth
      />
    </AuthForm>
  );
};

export default LoginPage;
