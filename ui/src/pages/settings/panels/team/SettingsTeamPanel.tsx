import { useCallback, useMemo, useState } from "react";

import { PlusIcon, TrashIcon } from "@phosphor-icons/react";

import Avatar from "@galaxy-io/dls/avatar/Avatar";
import Beacon, { BeaconVariant } from "@galaxy-io/dls/beacons/Beacon";
import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { AlignItems } from "@galaxy-io/dls/containers/FlexWrapper";
import SelectInput from "@galaxy-io/dls/inputs/SelectInput";
import InfiniteTable, {
  ColumnAlign,
  type ColumnDef,
  TableVariant,
} from "@galaxy-io/dls/table/InfiniteTable";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";

import type { Member, Role } from "@/gen/auth/v1/members_pb";

import Dialog from "@/components/Dialog";

import SettingsPanelLayout from "@/pages/settings/components/SettingsPanelLayout";
import {
  ROLE_OPTIONS,
  SETTINGS_TEAM_TABLE_COLUMN_WIDTH_EMAIL,
  SETTINGS_TEAM_TABLE_COLUMN_WIDTH_ROLE,
  SETTINGS_TEAM_TABLE_LOADING_ROW_COUNT,
  SETTINGS_TEAM_TABLE_ROLE_SELECT_DROPDOWN_WIDTH,
  SETTINGS_TEAM_TABLE_ROLE_SELECT_WIDTH,
} from "@/pages/settings/constants";
import { optionRole, roleOption } from "@/pages/settings/utils";

import { useRemoveMemberMutation, useSetMemberRoleMutation } from "@/api/mutations/auth";
import { useListMembersQuery } from "@/api/queries/auth";

import { useConfirm } from "@/hooks/useConfirm";

import type { AppSession } from "@/auth/types";
import { getErrorMessage } from "@/utils/errors";

const memberDisplayName = (member: Member): string => member.name || member.email || "Member";

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
      enableSorting: true,
      sortDescFirst: false,
      cellLoading: () => (
        <FlexWrapper gap={12} alignItems={AlignItems.CENTER}>
          <TextShimmer width={120} height={16} />
        </FlexWrapper>
      ),
      cell: ({ row }) => (
        <FlexWrapper gap={12} alignItems={AlignItems.CENTER}>
          <FlexItem grow={0} shrink={0} display="flex">
            <Avatar size={26} seed={row.original.userId} />
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
      size: SETTINGS_TEAM_TABLE_COLUMN_WIDTH_EMAIL,
      enableSorting: true,
      sortDescFirst: false,
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
      size: SETTINGS_TEAM_TABLE_COLUMN_WIDTH_ROLE,
      enableSorting: false,
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
                width={SETTINGS_TEAM_TABLE_ROLE_SELECT_WIDTH}
                dropdownWidth={SETTINGS_TEAM_TABLE_ROLE_SELECT_DROPDOWN_WIDTH}
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

interface SettingsTeamPanelProps {
  session: AppSession;
  onInvite: () => void;
}

const SettingsTeamPanel = ({ session, onInvite }: SettingsTeamPanelProps) => {
  const [error, setError] = useState<string>();

  const membersQuery = useListMembersQuery({
    options: { enabled: session.isAuthenticated },
  });
  const { mutate: setMemberRole, isPending: isSettingRole } = useSetMemberRoleMutation();
  const { mutate: removeMember, isPending: isRemovingMember } = useRemoveMemberMutation();

  const canManageTeam = membersQuery.data?.canManage === true;
  const isMutatingMembers = isSettingRole || isRemovingMember;
  const displayError =
    error ??
    (membersQuery.error
      ? getErrorMessage(membersQuery.error, "Could not load members")
      : undefined);

  const memberConfirm = useConfirm<Member>({
    entityLabel: "Team member",
    entityName: memberDisplayName,
    onConfirm: (member, { onSuccess, onError }) =>
      removeMember({ userId: member.userId }, { onSuccess, onError }),
  });

  const sortedMembers = useMemo(() => {
    const members = membersQuery.data?.members ?? [];
    return [...members].sort((a, b) => {
      if (a.userId === session.userId) return -1;
      if (b.userId === session.userId) return 1;
      return memberDisplayName(a).localeCompare(memberDisplayName(b));
    });
  }, [membersQuery.data?.members, session.userId]);

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
      memberConfirm.handleOpen(member);
    },
    [memberConfirm],
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

  return (
    <>
      <SettingsPanelLayout
        title="Team"
        actions={
          canManageTeam
            ? [
                <Button
                  key="invite-team"
                  label="Invite team"
                  icon={PlusIcon}
                  variant={ButtonVariant.PRIMARY}
                  onClick={onInvite}
                />,
              ]
            : undefined
        }
      >
        <FlexWrapper grow={1} basis={0} minHeight={0} fillWidth>
          <InfiniteTable<Member>
            columns={columns}
            data={sortedMembers}
            getRowId={(row) => row.userId}
            isLoading={membersQuery.isLoading}
            loadingRowCount={SETTINGS_TEAM_TABLE_LOADING_ROW_COUNT}
            enableSorting
            variant={TableVariant.BASE}
            fillWidth
            fillHeight
          />
        </FlexWrapper>
        {displayError && (
          <FlexWrapper padding="16px" fillWidth>
            <Text size={TextSize.CAPTION} variant={TextVariant.ERROR}>
              {displayError}
            </Text>
          </FlexWrapper>
        )}
      </SettingsPanelLayout>
      <Dialog
        open={memberConfirm.isOpen}
        onClose={memberConfirm.handleClose}
        onConfirm={memberConfirm.handleConfirm}
        title="Remove team member"
        body="This member will immediately lose access to the organization and its resources."
        confirmationPhrase={memberConfirm.target && memberDisplayName(memberConfirm.target)}
        confirmLabel="Remove member"
        confirmVariant={ButtonVariant.ERROR}
        isPending={isRemovingMember}
      />
    </>
  );
};

export default SettingsTeamPanel;
