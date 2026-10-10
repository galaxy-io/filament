import { type FC, useState } from "react";

import { create } from "@bufbuild/protobuf";
import { match } from "ts-pattern";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Alert, { AlertVariant } from "@galaxy-io/dls/feedback/Alert";
import CopyInput from "@galaxy-io/dls/inputs/CopyInput";
import SelectInput from "@galaxy-io/dls/inputs/SelectInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Box from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import Modal from "@galaxy-io/dls/modal/Modal";
import Text, { TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";

import {
  type InviteMemberRequest,
  InviteMemberRequestSchema,
  Role,
} from "@/gen/auth/v1/members_pb";

import { SETTINGS_INVITE_DEFAULT_ROLE, SETTINGS_ROLE_OPTIONS } from "@/pages/settings/constants";
import { SettingsTeamView } from "@/pages/settings/types";
import { createInviteUrl, encodeInviteToken } from "@/pages/settings/utils";

import { useInviteMemberMutation } from "@/api/queries/auth";

import { getErrorMessage } from "@/utils/errors";
import { mapOptionIdToEnum } from "@/utils/select";

interface SettingsTeamInviteModalState {
  email: InviteMemberRequest["email"];
  givenName: InviteMemberRequest["givenName"];
  familyName: InviteMemberRequest["familyName"];
  role: InviteMemberRequest["role"];
  error: string | undefined;
}

const DEFAULT_STATE: SettingsTeamInviteModalState = {
  email: "",
  givenName: "",
  familyName: "",
  role: SETTINGS_INVITE_DEFAULT_ROLE,
  error: undefined,
};

interface SettingsTeamInviteModalProps {
  isOpen: boolean;
  view: SettingsTeamView | undefined;
  inviteToken?: string;
  onViewChange: (view: SettingsTeamView, inviteToken?: string) => void;
  onClose: () => void;
}

const SettingsTeamInviteModal: FC<SettingsTeamInviteModalProps> = ({
  isOpen,
  view,
  inviteToken,
  onViewChange,
  onClose,
}) => {
  const [state, setState] = useState<SettingsTeamInviteModalState>(DEFAULT_STATE);

  const { mutate: inviteMember, isPending: isInviting } = useInviteMemberMutation();

  const inviteLink = inviteToken ? createInviteUrl(inviteToken) : undefined;

  const handleSubmit = () => {
    if (state.email === "" || state.givenName === "" || state.familyName === "") {
      setState((prev) => ({ ...prev, error: "All fields are required" }));
      return;
    }
    setState((prev) => ({ ...prev, error: undefined }));
    inviteMember(
      create(InviteMemberRequestSchema, {
        email: state.email,
        givenName: state.givenName,
        familyName: state.familyName,
        role: state.role,
      }),
      {
        onSuccess: ({ userId, code }) => {
          onViewChange(SettingsTeamView.LINK, encodeInviteToken({ userId, code }));
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
    onViewChange(SettingsTeamView.INVITE);
  };

  const handleOpenChange = (nextIsOpen: boolean) => {
    if (!nextIsOpen && !isInviting) onClose();
  };

  const content = match(view)
    .with(SettingsTeamView.LINK, () => ({
      header: "Invite created",
      footer: (
        <>
          <Button
            label="Add another teammate"
            variant={ButtonVariant.SECONDARY}
            onClick={handleAddAnother}
          />
          <Button label="Done" onClick={onClose} />
        </>
      ),
      body: (
        <Flex alignItems={AlignItems.STRETCH} direction={FlexDirection.COLUMN} gap={16}>
          <Text isProse variant={TextVariant.SECONDARY}>
            Share this link with your new teammate
          </Text>
          {inviteLink && <CopyInput value={inviteLink} fillWidth family={FontFamily.MONO} />}
        </Flex>
      ),
    }))
    .otherwise(() => ({
      header: "Invite team",
      footer: (
        <>
          <Button
            label="Cancel"
            variant={ButtonVariant.SECONDARY}
            onClick={onClose}
            isDisabled={isInviting}
          />
          <Button label="Create invite" onClick={handleSubmit} isLoading={isInviting} />
        </>
      ),
      body: (
        <Flex alignItems={AlignItems.STRETCH} direction={FlexDirection.COLUMN} gap={16}>
          <Text isProse variant={TextVariant.SECONDARY}>
            Invite a new member to your organization
          </Text>
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
            options={SETTINGS_ROLE_OPTIONS}
            value={String(state.role)}
            onChange={(id) =>
              setState((prev) => ({ ...prev, role: mapOptionIdToEnum(Role, id ?? "") }))
            }
            fillWidth
            isRequired
          />
          {state.error && (
            <Box fillWidth>
              <Alert variant={AlertVariant.ERROR}>{state.error}</Alert>
            </Box>
          )}
        </Flex>
      ),
    }));

  return (
    <Modal
      header={content.header}
      isOpen={isOpen}
      isDismissable={!isInviting}
      onOpenChange={handleOpenChange}
      footer={content.footer}
    >
      {content.body}
    </Modal>
  );
};

export default SettingsTeamInviteModal;
