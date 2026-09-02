import { useState } from "react";

import { useNavigate } from "@tanstack/react-router";

import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import PasswordInput from "@galaxy-io/dls/inputs/PasswordInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";

import type { RegisterRequest } from "@/gen/auth/v1/session_pb";

import AuthForm, { AuthFormFooter } from "@/layouts/auth/AuthForm";

import { useRegisterMutation } from "@/api/mutations/auth";

import { getErrorMessage } from "@/utils/errors";

interface RegisterPageState {
  orgName: RegisterRequest["orgName"];
  givenName: RegisterRequest["givenName"];
  familyName: RegisterRequest["familyName"];
  email: RegisterRequest["email"];
  password: RegisterRequest["password"];
  error: string | undefined;
}

const DEFAULT_STATE: RegisterPageState = {
  orgName: "",
  givenName: "",
  familyName: "",
  email: "",
  password: "",
  error: undefined,
};

const RegisterPage = () => {
  const navigate = useNavigate();
  const [state, setState] = useState<RegisterPageState>(DEFAULT_STATE);
  const { mutate: register, isPending } = useRegisterMutation();

  const handleSubmit = () => {
    if (
      state.orgName === "" ||
      state.givenName === "" ||
      state.familyName === "" ||
      state.email === "" ||
      state.password === ""
    ) {
      setState((prev) => ({ ...prev, error: "All fields are required" }));
      return;
    }
    setState((prev) => ({ ...prev, error: undefined }));
    register(
      {
        orgName: state.orgName,
        givenName: state.givenName,
        familyName: state.familyName,
        email: state.email,
        password: state.password,
      },
      {
        onSuccess: () => {
          void navigate({ to: "/", replace: true });
        },
        onError: (err) => {
          setState((prev) => ({
            ...prev,
            error: getErrorMessage(err, "Sign-up failed, please try again"),
          }));
        },
      },
    );
  };

  return (
    <AuthForm
      title="Create your organization"
      subtitle="Set up your organization and get started."
      error={state.error}
      submitLabel="Create organization"
      isPending={isPending}
      onSubmit={handleSubmit}
      footer={<AuthFormFooter prompt="Already have an account?" to="/" label="Sign in" />}
    >
      <TextInput
        label="Organization"
        value={state.orgName}
        onChange={(orgName) => setState((prev) => ({ ...prev, orgName }))}
        placeholder="Intergalactic Data Labs"
        size={InputSize.LARGE}
        fillWidth
        autoFocus
      />
      <FlexWrapper gap={FlexGap.MEDIUM} fillWidth>
        <FlexItem grow={1} basis={0} minWidth={0}>
          <TextInput
            label="First name"
            value={state.givenName}
            onChange={(givenName) => setState((prev) => ({ ...prev, givenName }))}
            size={InputSize.LARGE}
            fillWidth
          />
        </FlexItem>
        <FlexItem grow={1} basis={0} minWidth={0}>
          <TextInput
            label="Last name"
            value={state.familyName}
            onChange={(familyName) => setState((prev) => ({ ...prev, familyName }))}
            size={InputSize.LARGE}
            fillWidth
          />
        </FlexItem>
      </FlexWrapper>
      <TextInput
        label="Email"
        value={state.email}
        onChange={(email) => setState((prev) => ({ ...prev, email }))}
        placeholder="hello@getgalaxy.io"
        size={InputSize.LARGE}
        fillWidth
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

export default RegisterPage;
