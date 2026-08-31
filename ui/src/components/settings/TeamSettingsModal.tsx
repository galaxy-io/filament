import { useCallback, useEffect, useMemo, useState } from "react";

import { styled } from "@linaria/react";
import { PlusIcon, TrashIcon } from "@phosphor-icons/react";

import Avatar from "@galaxy-io/dls/avatar/Avatar";
import Beacon, { BeaconVariant } from "@galaxy-io/dls/beacons/Beacon";
import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import CopyInput from "@galaxy-io/dls/inputs/CopyInput";
import SelectInput, { type SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Modal from "@galaxy-io/dls/modal/Modal";
import InfiniteTable, { ColumnAlign, type ColumnDef } from "@galaxy-io/dls/table/InfiniteTable";
import Paragraph from "@galaxy-io/dls/text/Paragraph";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import type { Member, Role } from "@/gen/auth/v1/members_pb";

import DeleteConfirmDialog from "@/components/DeleteConfirmDialog";
import {
  INVITE_DEFAULT_ROLE,
  optionRole,
  ROLE_OPTIONS,
  roleOption,
} from "@/components/settings/constants";
import { TeamSettingsView } from "@/components/settings/types";

import BaseHeader, { BaseHeaderSize } from "@/layouts/components/BaseHeader";

import {
  useInviteMemberMutation,
  useRemoveMemberMutation,
  useSetMemberRoleMutation,
} from "@/api/mutations/auth";
import { useListMembersQuery } from "@/api/queries/auth";

import { useDeleteConfirm } from "@/hooks/useDeleteConfirm";

import { encodeInviteToken } from "@/auth/inviteToken";
import type { AppSession } from "@/auth/session";
import { getErrorMessage } from "@/utils/errors";

const TEAM_SETTINGS_DIALOG_ID = "team-settings-dialog";

const DialogWrapper = withTheme(
  styled.div<PropsWithTheme<{ $wide?: boolean; $panel?: boolean }>>`
  display: flex;
  flex-direction: column;
  width: ${({ $wide, $panel }) => ($panel ? "100%" : `${$wide ? 680 : 640}px`)};
  max-width: ${({ $panel }) => ($panel ? "none" : "calc(100vw - 32px)")};
  height: auto;
  background-color: ${({ theme, $panel }) =>
    $panel ? "transparent" : theme.color.background.primary};
  border: ${({ theme, $panel }) => ($panel ? "none" : `0.5px solid ${theme.color.border.primary}`)};
  border-radius: ${({ $panel }) => ($panel ? 0 : 8)}px;
  overflow: hidden;
`,
);

const BodyWrapper = withTheme(styled.div<PropsWithTheme<{ $panel?: boolean }>>`
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: ${({ $panel }) => ($panel ? "20px 0 0" : "16px")};
  background-color: ${({ theme, $panel }) =>
    $panel ? "transparent" : theme.color.background.base};
`);

const TableWrapper = withTheme(styled.div<PropsWithTheme<{ $fill?: boolean }>>`
  width: 100%;
  flex: ${({ $fill }) => ($fill ? 1 : "initial")};
  min-height: 0;
  border-radius: 5px;
  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
`);

const FormContent = styled.div`
  display: flex;
  flex-direction: column;
  gap: 12px;
  width: 100%;
`;

const memberDisplayName = (member: Member): string => member.name || member.email || "Member";

const MemberAvatar = ({ member }: { member: Member }) => (
  <Avatar size={26} seed={member.email || member.name || member.userId} />
);

const memberColumns = ({
  myID,
  canManage,
  isMutatingMembers,
  onRoleChange,
  onRemove,
}: {
  myID: string | undefined;
  canManage: boolean;
  isMutatingMembers: boolean;
  onRoleChange: (member: Member, role: Role) => void;
  onRemove: (member: Member) => void;
}): ColumnDef<Member>[] => {
  return [
    {
      id: "name",
      header: "Name",
      accessorFn: (row) => memberDisplayName(row),
      cellLoading: () => (
        <FlexWrapper gap={12} alignItems={AlignItems.CENTER}>
          <TextShimmer width={120} height={16} />
        </FlexWrapper>
      ),
      cell: ({ row }) => (
        <FlexWrapper gap={12} alignItems={AlignItems.CENTER}>
          <FlexItem grow={0} shrink={0} display="flex">
            <MemberAvatar member={row.original} />
          </FlexItem>
          <Text weight={TextWeight.MEDIUM} isEllipsis>
            {memberDisplayName(row.original)}
          </Text>
        </FlexWrapper>
      ),
    },
    {
      id: "email",
      header: "Email",
      accessorFn: (row) => row.email,
      size: 200,
      enableSorting: true,
      cellLoading: () => <TextShimmer height={16} width="80%" />,
      cell: ({ row }) => (
        <Text variant={TextVariant.SECONDARY} isEllipsis>
          {row.original.email}
        </Text>
      ),
    },
    {
      id: "role",
      header: "",
      align: ColumnAlign.RIGHT,
      size: 200,
      cellLoading: () => <TextShimmer height={16} width="80%" />,
      cell: ({ row }) => {
        const isMe = myID !== undefined && row.original.userId === myID;
        return (
          <FlexWrapper gap={8} alignItems={AlignItems.CENTER}>
            {isMe && (
              <FlexItem grow={0} shrink={0} display="flex">
                <Beacon variant={BeaconVariant.PRIMARY} isPulse />
              </FlexItem>
            )}
            <FlexWrapper gap={8} alignItems={AlignItems.CENTER}>
              <SelectInput
                options={ROLE_OPTIONS}
                value={roleOption(row.original.role)}
                onChange={(option) => {
                  const role = optionRole(option);
                  if (role !== undefined) {
                    onRoleChange(row.original, role);
                  }
                }}
                width={120}
                dropdownWidth={160}
                isDisabled={!canManage || isMe || isMutatingMembers}
              />
              {canManage && (
                <Button
                  icon={TrashIcon}
                  variant={ButtonVariant.SECONDARY}
                  size={ButtonSize.SMALL}
                  onClick={() => onRemove(row.original)}
                  isDisabled={isMe || isMutatingMembers}
                />
              )}
            </FlexWrapper>
          </FlexWrapper>
        );
      },
    },
  ];
};

interface TeamSettingsModalProps {
  session: AppSession;
  view: TeamSettingsView;
  inviteToken?: string;
  onViewChange: (view: TeamSettingsView, options?: { replace?: boolean }) => void;
  onInviteCreated: (token: string) => void;
  onClose?: () => void;
  isPanel?: boolean;
}

const TeamSettingsModal = ({
  session,
  view,
  inviteToken,
  onViewChange,
  onInviteCreated,
  onClose,
  isPanel = false,
}: TeamSettingsModalProps) => {
  const [email, setEmail] = useState("");
  const [givenName, setGivenName] = useState("");
  const [familyName, setFamilyName] = useState("");
  const [role, setRole] = useState<SelectInputOption>(INVITE_DEFAULT_ROLE);
  const [error, setError] = useState<string | undefined>(undefined);
  const [memberToRemove, setMemberToRemove] = useState<Member>();

  const membersQuery = useListMembersQuery({
    options: { enabled: !!session.accessToken },
  });
  const { mutate: inviteMember, isPending: isInviting } = useInviteMemberMutation();
  const { mutate: setMemberRole, isPending: isSettingRole } = useSetMemberRoleMutation();
  const { mutate: removeMember, isPending: isRemovingMember } = useRemoveMemberMutation();

  const memberDeleteConfirm = useDeleteConfirm({
    entityLabel: "Team member",
    entityName: memberToRemove ? memberDisplayName(memberToRemove) : "",
    onDelete: ({ onSuccess, onError }) => {
      if (!memberToRemove) return;
      removeMember({ userId: memberToRemove.userId }, { onSuccess, onError });
    },
    onDeleted: () => setMemberToRemove(undefined),
  });

  const members = membersQuery.data?.members ?? [];
  const canManage = membersQuery.data?.canManage;
  const canManageTeam = canManage === true;
  const isMutatingMembers = isSettingRole || isRemovingMember;
  const inviteLink = inviteToken ? `${window.location.origin}/invite/${inviteToken}` : undefined;
  const effectiveView =
    (view === TeamSettingsView.LINK && !inviteToken) ||
    (canManage === false && view !== TeamSettingsView.MEMBERS)
      ? TeamSettingsView.MEMBERS
      : view;
  const displayError =
    error ??
    (membersQuery.error
      ? getErrorMessage(membersQuery.error, "Could not load members")
      : undefined);

  useEffect(() => {
    if (effectiveView !== view) {
      onViewChange(effectiveView, { replace: true });
    }
  }, [effectiveView, onViewChange, view]);

  const sortedMembers = useMemo(() => {
    return [...members].sort((a, b) => {
      if (a.userId === session.userId) return -1;
      if (b.userId === session.userId) return 1;
      return memberDisplayName(a).localeCompare(memberDisplayName(b));
    });
  }, [members, session.userId]);

  const resetForm = useCallback(() => {
    setEmail("");
    setGivenName("");
    setFamilyName("");
    setRole(INVITE_DEFAULT_ROLE);
    setError(undefined);
  }, []);

  const handleClose = useCallback(() => {
    resetForm();
    onClose?.();
  }, [onClose, resetForm]);

  const handleDone = useCallback(() => {
    resetForm();
    if (isPanel) {
      onViewChange(TeamSettingsView.MEMBERS);
      return;
    }
    onClose?.();
  }, [isPanel, onClose, onViewChange, resetForm]);

  const handleRoleChange = useCallback(
    (member: Member, nextRole: Role) => {
      setError(undefined);
      setMemberRole(
        { userId: member.userId, role: nextRole },
        {
          onError: (err) => {
            setError(getErrorMessage(err, "Could not change role"));
          },
        },
      );
    },
    [setMemberRole],
  );

  const handleRemove = useCallback(
    (member: Member) => {
      setError(undefined);
      setMemberToRemove(member);
      memberDeleteConfirm.handleOpen();
    },
    [memberDeleteConfirm],
  );

  const columns = useMemo(
    () =>
      memberColumns({
        myID: session.userId,
        canManage: canManageTeam,
        isMutatingMembers,
        onRoleChange: handleRoleChange,
        onRemove: handleRemove,
      }),
    [session.userId, canManageTeam, isMutatingMembers, handleRoleChange, handleRemove],
  );

  const handleSubmit = () => {
    if (isInviting) {
      return;
    }
    if (canManage !== true) {
      setError(
        canManage === false ? "Only admins can invite teammates" : "Team permissions are loading",
      );
      return;
    }
    const inviteRole = optionRole(role);
    if (email === "" || givenName === "" || familyName === "" || inviteRole === undefined) {
      setError("All fields are required");
      return;
    }
    setError(undefined);
    inviteMember(
      { email, givenName, familyName, role: inviteRole },
      {
        onSuccess: ({ userId, code }) => {
          const token = encodeInviteToken({ userId, code });
          onInviteCreated(token);
        },
        onError: (err) => {
          setError(getErrorMessage(err, "Something went wrong, please try again"));
        },
      },
    );
  };

  return (
    <>
      <DialogWrapper
        id={TEAM_SETTINGS_DIALOG_ID}
        $wide={effectiveView !== TeamSettingsView.INVITE}
        $panel={isPanel}
      >
        <FlexWrapper padding={isPanel ? "0" : "16px"}>
          <BaseHeader
            title={
              effectiveView === TeamSettingsView.INVITE
                ? "Invite team"
                : effectiveView === TeamSettingsView.LINK
                  ? "Invite created"
                  : "Team"
            }
            description={
              effectiveView === TeamSettingsView.INVITE
                ? "Invite a new member to your organization"
                : effectiveView === TeamSettingsView.LINK
                  ? "Share this link with your new teammate"
                  : "Manage the people who can access this organization"
            }
            size={isPanel ? BaseHeaderSize.LARGE : undefined}
            actions={
              isPanel && effectiveView === TeamSettingsView.MEMBERS && canManageTeam
                ? [
                    <Button
                      key="invite-team"
                      label="Invite team"
                      icon={PlusIcon}
                      variant={ButtonVariant.PRIMARY}
                      onClick={() => {
                        setError(undefined);
                        onViewChange(TeamSettingsView.INVITE);
                      }}
                    />,
                  ]
                : undefined
            }
            onClose={isPanel ? undefined : handleClose}
          />
        </FlexWrapper>

        {!isPanel && (
          <FlexItem grow={0} shrink={0}>
            <HorizontalDivider />
          </FlexItem>
        )}

        <BodyWrapper $panel={isPanel}>
          {effectiveView === TeamSettingsView.MEMBERS && (
            <FlexWrapper direction={FlexDirection.COLUMN} gap={12} fillWidth>
              {!isPanel && (
                <FlexWrapper
                  alignItems={AlignItems.CENTER}
                  justifyContent={JustifyContent.SPACE_BETWEEN}
                  fillWidth
                >
                  <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
                    {canManageTeam ? "View and manage your team members" : "View your team members"}
                  </Text>
                  {canManageTeam && (
                    <Button
                      label="Invite team"
                      icon={PlusIcon}
                      variant={ButtonVariant.PRIMARY}
                      onClick={() => {
                        setError(undefined);
                        onViewChange(TeamSettingsView.INVITE);
                      }}
                    />
                  )}
                </FlexWrapper>
              )}
              <TableWrapper>
                <InfiniteTable<Member>
                  columns={columns}
                  data={sortedMembers}
                  getRowId={(row) => row.userId}
                  isLoading={membersQuery.isLoading}
                  loadingRowCount={3}
                  enableSorting
                  noLastRowBorder
                  noLastRowPadding
                />
              </TableWrapper>
              {displayError && (
                <Text size={TextSize.CAPTION} variant={TextVariant.ERROR}>
                  {displayError}
                </Text>
              )}
            </FlexWrapper>
          )}

          {effectiveView === TeamSettingsView.INVITE && (
            <FormContent>
              <TextInput
                label="Email"
                value={email}
                onChange={setEmail}
                placeholder="name@example.com"
                fillWidth
                isRequired
                autoFocus
              />
              <TextInput
                label="First name"
                value={givenName}
                onChange={setGivenName}
                fillWidth
                isRequired
              />
              <TextInput
                label="Last name"
                value={familyName}
                onChange={setFamilyName}
                fillWidth
                isRequired
              />
              <SelectInput
                label="Role"
                options={ROLE_OPTIONS}
                value={role}
                onChange={setRole}
                fillWidth
                isRequired
              />
              {displayError && (
                <Text size={TextSize.CAPTION} variant={TextVariant.ERROR}>
                  {displayError}
                </Text>
              )}
            </FormContent>
          )}

          {effectiveView === TeamSettingsView.LINK && inviteLink && (
            <FormContent>
              <FlexWrapper direction={FlexDirection.COLUMN} gap={4} fillWidth>
                <Text size={TextSize.BODY_LG} weight={TextWeight.MEDIUM}>
                  Data is a team sport
                </Text>
                <Paragraph variant={TextVariant.SECONDARY}>
                  Send {givenName || "your teammate"} their invite and start shipping pipelines
                  together.
                </Paragraph>
              </FlexWrapper>
              <CopyInput value={inviteLink} fillWidth isMonospace />
            </FormContent>
          )}
        </BodyWrapper>

        {effectiveView !== TeamSettingsView.MEMBERS && (
          <>
            {!isPanel && (
              <FlexItem grow={0} shrink={0}>
                <HorizontalDivider />
              </FlexItem>
            )}

            <FlexWrapper
              justifyContent={
                effectiveView === TeamSettingsView.LINK
                  ? JustifyContent.SPACE_BETWEEN
                  : JustifyContent.END
              }
              alignItems={AlignItems.CENTER}
              padding={isPanel ? "16px 0 0" : "16px"}
              gap={8}
              fillWidth
            >
              {effectiveView === TeamSettingsView.INVITE && (
                <>
                  <Button
                    size={ButtonSize.LARGE}
                    label="Cancel"
                    variant={ButtonVariant.SECONDARY}
                    onClick={() => {
                      resetForm();
                      onViewChange(TeamSettingsView.MEMBERS);
                    }}
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
              )}
              {effectiveView === TeamSettingsView.LINK && (
                <>
                  <Button
                    size={ButtonSize.LARGE}
                    label="Add another teammate"
                    variant={ButtonVariant.SECONDARY}
                    onClick={() => {
                      resetForm();
                      onViewChange(TeamSettingsView.INVITE);
                    }}
                  />
                  <Button size={ButtonSize.LARGE} label="Done" onClick={handleDone} />
                </>
              )}
            </FlexWrapper>
          </>
        )}
      </DialogWrapper>
      <Modal
        open={memberDeleteConfirm.isOpen}
        onClose={() => {
          memberDeleteConfirm.handleClose();
          setMemberToRemove(undefined);
        }}
      >
        <DeleteConfirmDialog
          open={memberDeleteConfirm.isOpen}
          onClose={() => {
            memberDeleteConfirm.handleClose();
            setMemberToRemove(undefined);
          }}
          onConfirm={memberDeleteConfirm.handleConfirm}
          title="Remove team member"
          body="This member will immediately lose access to the organization and its resources."
          confirmationPhrase={memberToRemove ? memberDisplayName(memberToRemove) : ""}
          confirmLabel="Remove member"
          isPending={isRemovingMember}
        />
      </Modal>
    </>
  );
};

export default TeamSettingsModal;
