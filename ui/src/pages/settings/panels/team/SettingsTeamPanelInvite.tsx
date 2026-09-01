import { useEffect, useState } from "react";

import { match } from "ts-pattern";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import CopyInput from "@galaxy-io/dls/inputs/CopyInput";
import SelectInput from "@galaxy-io/dls/inputs/SelectInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import type { InviteMemberRequest } from "@/gen/auth/v1/members_pb";

import Dialog from "@/components/Dialog";

import { INVITE_DEFAULT_ROLE, ROLE_OPTIONS } from "@/pages/settings/constants";
import { TeamSettingsView } from "@/pages/settings/types";
import { optionRole, roleOption } from "@/pages/settings/utils";

import { useInviteMemberMutation } from "@/api/mutations/auth";
import { useListMembersQuery } from "@/api/queries/auth";

import type { AppSession } from "@/auth/types";
import { buildInviteUrl, encodeInviteToken } from "@/auth/utils";
import { getErrorMessage } from "@/utils/errors";

interface SettingsTeamPanelInviteState {
  email: InviteMemberRequest["email"];
  givenName: InviteMemberRequest["givenName"];
  familyName: InviteMemberRequest["familyName"];
  role: InviteMemberRequest["role"];
  error: string | undefined;
}

const DEFAULT_STATE: SettingsTeamPanelInviteState = {
  email: "",
  givenName: "",
  familyName: "",
  role: INVITE_DEFAULT_ROLE,
  error: undefined,
};

interface SettingsTeamPanelInviteProps {
  open: boolean;
  session: AppSession;
  view: TeamSettingsView;
  inviteToken?: string;
  onViewChange: (view: TeamSettingsView, options?: { replace?: boolean }) => void;
  onInviteCreated: (token: string) => void;
  onClose: () => void;
}

const SettingsTeamPanelInvite = ({
  open,
  session,
  view,
  inviteToken,
  onViewChange,
  onInviteCreated,
  onClose,
}: SettingsTeamPanelInviteProps) => {
  const [state, setState] = useState<SettingsTeamPanelInviteState>(DEFAULT_STATE);

  const membersQuery = useListMembersQuery({
    options: { enabled: session.isAuthenticated },
  });
  const { mutate: inviteMember, isPending: isInviting } = useInviteMemberMutation();

  const canManage = membersQuery.data?.canManage;
  const inviteLink = inviteToken ? buildInviteUrl(inviteToken) : undefined;

  useEffect(() => {
    if ((view === TeamSettingsView.LINK && !inviteToken) || canManage === false) {
      onViewChange(TeamSettingsView.MEMBERS, { replace: true });
    }
  }, [canManage, inviteToken, onViewChange, view]);

  const handleSubmit = () => {
    if (canManage !== true) {
      setState((prev) => ({
        ...prev,
        error:
          canManage === false ? "Only admins can invite teammates" : "Team permissions are loading",
      }));
      return;
    }
    if (state.email === "" || state.givenName === "" || state.familyName === "") {
      setState((prev) => ({ ...prev, error: "All fields are required" }));
      return;
    }
    setState((prev) => ({ ...prev, error: undefined }));
    inviteMember(
      {
        email: state.email,
        givenName: state.givenName,
        familyName: state.familyName,
        role: state.role,
      },
      {
        onSuccess: ({ userId, code }) => {
          onInviteCreated(encodeInviteToken({ userId, code }));
        },
        onError: (err) => {
          setState((prev) => ({
            ...prev,
            error: getErrorMessage(err, "Something went wrong, please try again"),
          }));
        },
      },
    );
  };

  const handleAddAnother = () => {
    setState(DEFAULT_STATE);
    onViewChange(TeamSettingsView.INVITE);
  };

  return match(view)
    .with(TeamSettingsView.LINK, () => (
      <Dialog
        open={open}
        title="Invite created"
        onClose={onClose}
        footer={
          <>
            <Button
              size={ButtonSize.LARGE}
              label="Add another teammate"
              variant={ButtonVariant.SECONDARY}
              onClick={handleAddAnother}
            />
            <Button size={ButtonSize.LARGE} label="Done" onClick={onClose} />
          </>
        }
      >
        <FlexWrapper direction={FlexDirection.COLUMN} gap={4} fillWidth>
          <Text variant={TextVariant.SECONDARY}>Share this link with your new teammate</Text>
        </FlexWrapper>
        {inviteLink && <CopyInput value={inviteLink} fillWidth isMonospace />}
      </Dialog>
    ))
    .otherwise(() => (
      <Dialog
        open={open}
        title="Invite team"
        description="Invite a new member to your organization"
        onClose={onClose}
        footer={
          <>
            <Button
              size={ButtonSize.LARGE}
              label="Cancel"
              variant={ButtonVariant.SECONDARY}
              onClick={onClose}
              isDisabled={isInviting}
            />
            <Button
              size={ButtonSize.LARGE}
              label="Create invite"
              onClick={handleSubmit}
              isLoading={isInviting}
              isDisabled={canManage !== true}
            />
          </>
        }
      >
        <TextInput
          label="Email"
          value={state.email}
          onChange={(email) => setState((prev) => ({ ...prev, email }))}
          placeholder="name@example.com"
          fillWidth
          isRequired
          autoFocus
        />
        <TextInput
          label="First name"
          value={state.givenName}
          onChange={(givenName) => setState((prev) => ({ ...prev, givenName }))}
          fillWidth
          isRequired
        />
        <TextInput
          label="Last name"
          value={state.familyName}
          onChange={(familyName) => setState((prev) => ({ ...prev, familyName }))}
          fillWidth
          isRequired
        />
        <SelectInput
          label="Role"
          options={ROLE_OPTIONS}
          value={roleOption(state.role)}
          onChange={(option) => setState((prev) => ({ ...prev, role: optionRole(option) }))}
          fillWidth
          isRequired
        />
        {state.error && (
          <Text size={TextSize.CAPTION} variant={TextVariant.ERROR}>
            {state.error}
          </Text>
        )}
      </Dialog>
    ));
};

export default SettingsTeamPanelInvite;
