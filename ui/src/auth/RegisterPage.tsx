import { useState } from "react";

import Button from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import GalaxyLogomark from "@galaxy-io/dls/icons/GalaxyLogomark";
import PasswordInput from "@galaxy-io/dls/inputs/PasswordInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import FilamentWordmark from "@/components/FilamentWordmark";

import { API_URL } from "@/constants";

import { Card, LinkText, Page } from "@/auth/LoginPage";

// RegisterPage is filament's self-service signup: one organization plus its
// first admin user. The org becomes the tenant. On success the user goes
// through the normal sign-in flow.
const RegisterPage = () => {
  const [orgName, setOrgName] = useState("");
  const [givenName, setGivenName] = useState("");
  const [familyName, setFamilyName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | undefined>(undefined);

  const handleSubmit = async () => {
    if (pending) {
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
    setPending(true);
    setError(undefined);
    try {
      const res = await fetch(`${API_URL}/auth/register`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ orgName, givenName, familyName, email, password }),
      });
      if (!res.ok) {
        setError(await res.text());
        return;
      }
      // Registration done; run the normal sign-in flow from the top.
      window.location.replace("/");
    } catch {
      setError("Sign-up failed, please try again");
    } finally {
      setPending(false);
    }
  };

  return (
    <Page>
      <Card
        onKeyDown={(e: React.KeyboardEvent) => {
          if (e.key === "Enter") {
            void handleSubmit();
          }
        }}
      >
        <FlexWrapper
          direction={FlexDirection.COLUMN}
          alignItems={AlignItems.CENTER}
          gap={FlexGap.MEDIUM}
        >
          <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.MEDIUM}>
            <GalaxyLogomark height={14} />
            <FilamentWordmark height={22} />
          </FlexWrapper>
          <Text size={TextSize.BODY_MD} variant={TextVariant.SECONDARY}>
            Create your organization to get started
          </Text>
        </FlexWrapper>
        <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.MEDIUM} fillWidth>
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
          {error && (
            <Text size={TextSize.CAPTION} variant={TextVariant.ERROR}>
              {error}
            </Text>
          )}
          <Button
            label={pending ? "Creating..." : "Create organization"}
            onClick={() => void handleSubmit()}
            isDisabled={pending}
            fillWidth
          />
        </FlexWrapper>
        <FlexWrapper
          alignItems={AlignItems.CENTER}
          justifyContent={JustifyContent.CENTER}
          gap={FlexGap.SMALL}
          fillWidth
        >
          <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
            Already have an account?
          </Text>
          <LinkText
            onClick={() => {
              window.location.href = "/";
            }}
          >
            <Text size={TextSize.BODY_SM} variant={TextVariant.BLUE}>
              Sign in
            </Text>
          </LinkText>
        </FlexWrapper>
      </Card>
    </Page>
  );
};

export default RegisterPage;
