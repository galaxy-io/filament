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
import InfiniteTable, { ColumnAlign, type ColumnDef } from "@galaxy-io/dls/table/InfiniteTable";
import Paragraph from "@galaxy-io/dls/text/Paragraph";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import type { Member, Role } from "@/gen/auth/v1/members_pb";

import {
  INVITE_DEFAULT_ROLE,
  optionRole,
  ROLE_OPTIONS,
  roleOption,
} from "@/components/settings/constants";
import { TeamSettingsView } from "@/components/settings/types";

import BaseHeader from "@/layouts/components/BaseHeader";

import {
  useInviteMemberMutation,
  useRemoveMemberMutation,
  useSetMemberRoleMutation,
} from "@/api/mutations/auth";
import { useListMembersQuery } from "@/api/queries/auth";

import { encodeInviteToken } from "@/auth/inviteToken";
import type { AppSession } from "@/auth/session";
import { getErrorMessage } from "@/utils/errors";

const TEAM_SETTINGS_DIALOG_ID = "team-settings-dialog";

const DialogWrapper = withTheme(styled.div<PropsWithTheme<{ $wide?: boolean }>>`
  display: flex;
  flex-direction: column;
  width: ${({ $wide }) => ($wide ? 680 : 520)}px;
  background-color: ${({ theme }) => theme.color.background.primary};
  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 8px;
`);

const BodyWrapper = styled.div`
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 16px;
`;

const TableWrapper = withTheme(styled.div<PropsWithTheme>`
  width: 100%;
  border-radius: 5px;
  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
`);

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
  onClose: () => void;
}

const TeamSettingsModal = ({
  session,
  view,
  inviteToken,
  onViewChange,
  onInviteCreated,
  onClose,
}: TeamSettingsModalProps) => {
  const [email, setEmail] = useState("");
  const [givenName, setGivenName] = useState("");
  const [familyName, setFamilyName] = useState("");
  const [role, setRole] = useState<SelectInputOption>(INVITE_DEFAULT_ROLE);
  const [error, setError] = useState<string | undefined>(undefined);

  const membersQuery = useListMembersQuery({
    options: { enabled: !!session.accessToken },
  });
  const { mutate: inviteMember, isPending: isInviting } = useInviteMemberMutation();
  const { mutate: setMemberRole, isPending: isSettingRole } = useSetMemberRoleMutation();
  const { mutate: removeMember, isPending: isRemovingMember } = useRemoveMemberMutation();

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
    onClose();
  }, [onClose, resetForm]);

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
      removeMember(
        { userId: member.userId },
        {
          onError: (err) => {
            setError(getErrorMessage(err, "Could not remove member"));
          },
        },
      );
    },
    [removeMember],
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
    <DialogWrapper id={TEAM_SETTINGS_DIALOG_ID} $wide={effectiveView !== TeamSettingsView.INVITE}>
      <FlexWrapper padding={"16px"}>
        <BaseHeader title="Team" onClose={handleClose} />
      </FlexWrapper>

      <FlexItem grow={0} shrink={0}>
        <HorizontalDivider />
      </FlexItem>

      <BodyWrapper>
        {effectiveView === TeamSettingsView.MEMBERS && (
          <FlexWrapper direction={FlexDirection.COLUMN} gap={12} fillWidth>
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
          <FlexWrapper direction={FlexDirection.COLUMN} gap={12} fillWidth>
            <TextInput
              label="Email"
              value={email}
              onChange={setEmail}
              placeholder="name@example.com"
              fillWidth
              autoFocus
            />
            <TextInput label="First name" value={givenName} onChange={setGivenName} fillWidth />
            <TextInput label="Last name" value={familyName} onChange={setFamilyName} fillWidth />
            <SelectInput
              label="Role"
              options={ROLE_OPTIONS}
              value={role}
              onChange={setRole}
              fillWidth
            />
            {displayError && (
              <Text size={TextSize.CAPTION} variant={TextVariant.ERROR}>
                {displayError}
              </Text>
            )}
          </FlexWrapper>
        )}

        {effectiveView === TeamSettingsView.LINK && inviteLink && (
          <>
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
          </>
        )}
      </BodyWrapper>

      {effectiveView !== TeamSettingsView.MEMBERS && (
        <>
          <FlexItem grow={0} shrink={0}>
            <HorizontalDivider />
          </FlexItem>

          <FlexWrapper
            justifyContent={
              effectiveView === TeamSettingsView.LINK
                ? JustifyContent.SPACE_BETWEEN
                : JustifyContent.END
            }
            alignItems={AlignItems.CENTER}
            padding={"16px"}
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
                <Button size={ButtonSize.LARGE} label="Done" onClick={handleClose} />
              </>
            )}
          </FlexWrapper>
        </>
      )}
    </DialogWrapper>
  );
};

export default TeamSettingsModal;
