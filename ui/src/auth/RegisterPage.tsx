import { useState } from "react";

import PasswordInput from "@galaxy-io/dls/inputs/PasswordInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";

import { useRegisterMutation } from "@/api/mutations/auth";

import AuthCard, { AuthCardFooter } from "@/auth/AuthCard";
import { getErrorMessage } from "@/utils/errors";

// RegisterPage is filament's self-service signup: one organization plus its
// first admin user. The org becomes the tenant. On success the user goes
// through the normal sign-in flow.
const RegisterPage = () => {
  const [orgName, setOrgName] = useState("");
  const [givenName, setGivenName] = useState("");
  const [familyName, setFamilyName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | undefined>(undefined);
  const { mutate: register, isPending } = useRegisterMutation();

  const handleSubmit = () => {
    if (isPending) {
      return;
    }
    if (
      orgName === "" ||
      givenName === "" ||
      familyName === "" ||
      email === "" ||
      password === ""
    ) {
      setError("All fields are required");
      return;
    }
    setError(undefined);
    register(
      { orgName, givenName, familyName, email, password },
      {
        onSuccess: () => {
          window.location.replace("/");
        },
        onError: (err) => {
          setError(getErrorMessage(err, "Sign-up failed, please try again"));
        },
      },
    );
  };

  return (
    <AuthCard
      subtitle="Create your organization to get started"
      error={error}
      submitLabel="Create organization"
      pendingLabel="Creating..."
      isPending={isPending}
      onSubmit={handleSubmit}
      footer={<AuthCardFooter prompt="Already have an account?" href="/" label="Sign in" />}
    >
      <TextInput
        label="Organization"
        value={orgName}
        onChange={setOrgName}
        placeholder="Acme Inc."
        fillWidth
        autoFocus
      />
      <TextInput label="First name" value={givenName} onChange={setGivenName} fillWidth />
      <TextInput label="Last name" value={familyName} onChange={setFamilyName} fillWidth />
      <TextInput
        label="Email"
        value={email}
        onChange={setEmail}
        placeholder="you@company.com"
        fillWidth
      />
      <PasswordInput label="Password" value={password} onChange={setPassword} fillWidth />
    </AuthCard>
  );
};

export default RegisterPage;
