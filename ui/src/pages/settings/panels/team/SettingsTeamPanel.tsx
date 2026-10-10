import { type FC, useCallback, useMemo } from "react";

import { PlusIcon, TrashIcon, UserGearIcon } from "@phosphor-icons/react";

import Avatar, { AvatarSize } from "@galaxy-io/dls/avatar/Avatar";
import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import { MenuItem, MenuItemVariant, MenuRadioGroup, MenuSeparator } from "@galaxy-io/dls/menu/Menu";
import ConfirmDialog from "@galaxy-io/dls/modal/ConfirmDialog";
import InfiniteTable from "@galaxy-io/dls/table/InfiniteTable";
import type { TableColumn } from "@galaxy-io/dls/table/types";
import Text, { TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import { type Member, Role } from "@/gen/auth/v1/members_pb";

import SettingsPanelLayout from "@/pages/settings/components/SettingsPanelLayout";
import {
  SETTINGS_ROLE_MENU_OPTIONS,
  SETTINGS_ROLE_TO_CHIP_PROPS_MAP,
  SETTINGS_TEAM_TABLE_COLUMN_MIN_WIDTH_EMAIL,
  SETTINGS_TEAM_TABLE_COLUMN_MIN_WIDTH_NAME,
  SETTINGS_TEAM_TABLE_COLUMN_WIDTH_ROLE,
} from "@/pages/settings/constants";
import { optionIdToRole, roleLabel, roleToOptionId } from "@/pages/settings/utils";

import {
  useListMembersQuery,
  useRemoveMemberMutation,
  useSetMemberRoleMutation,
} from "@/api/queries/auth";

import { useConfirm } from "@/hooks/useConfirm";

import type { AppSession } from "@/auth/types";
import { getErrorMessage } from "@/utils/errors";

const memberDisplayName = (member: Member): string => member.name || member.email || "Member";

const memberColumns = (myID: string | undefined): TableColumn<Member>[] => [
  {
    id: "name",
    header: "Name",
    accessor: (row) => memberDisplayName(row),
    isRowHeader: true,
    canSort: true,
    minWidth: SETTINGS_TEAM_TABLE_COLUMN_MIN_WIDTH_NAME,
    cell: ({ row }) => (
      <Flex gap={8} alignItems={AlignItems.CENTER} minWidth={0}>
        <Avatar size={AvatarSize.SMALL} seed={row.userId} name={memberDisplayName(row)} isSquare />
        <Text lineClamp={1} shouldTooltipOnOverflow>
          {memberDisplayName(row)}
        </Text>
        {myID !== undefined && row.userId === myID && (
          <Chip label="You" size={ChipSize.SMALL} variant={ChipVariant.SECONDARY} />
        )}
      </Flex>
    ),
  },
  {
    id: "email",
    header: "Email",
    accessor: (row) => row.email,
    minWidth: SETTINGS_TEAM_TABLE_COLUMN_MIN_WIDTH_EMAIL,
    canSort: true,
    cell: ({ row }) => (
      <Text
        family={FontFamily.MONO}
        variant={TextVariant.SECONDARY}
        lineClamp={1}
        shouldTooltipOnOverflow
      >
        {row.email}
      </Text>
    ),
  },
  {
    id: "role",
    header: "Role",
    accessor: (row) => roleLabel(row.role),
    width: SETTINGS_TEAM_TABLE_COLUMN_WIDTH_ROLE,
    canSort: true,
    cell: ({ row }) => (
      <Chip
        label={roleLabel(row.role)}
        size={ChipSize.SMALL}
        {...SETTINGS_ROLE_TO_CHIP_PROPS_MAP[row.role]}
      />
    ),
  },
];

interface SettingsTeamPanelProps {
  session: AppSession;
  onInvite: () => void;
}

const SettingsTeamPanel: FC<SettingsTeamPanelProps> = ({ session, onInvite }) => {
  const { toast } = useToast();

  const membersQuery = useListMembersQuery({
    options: { enabled: session.isAuthenticated },
  });
  const { mutate: setMemberRole, isPending: isSettingRole } = useSetMemberRoleMutation();
  const { mutate: removeMember, isPending: isRemovingMember } = useRemoveMemberMutation();

  const canManageTeam = membersQuery.data?.canManage === true;
  const isMutatingMembers = isSettingRole || isRemovingMember;
  const displayError = membersQuery.error
    ? getErrorMessage(membersQuery.error, "Could not load members")
    : undefined;

  const memberConfirm = useConfirm<Member>({
    entityLabel: "Team member",
    entityName: memberDisplayName,
    messages: {
      successHeader: "Team member removed",
      successSubheader: (member) =>
        `${memberDisplayName(member)} no longer has access to this organization.`,
      errorHeader: "Remove failed",
      errorFallback: "Could not remove team member",
    },
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
      if (nextRole === member.role || nextRole === Role.UNSPECIFIED) return;
      setMemberRole(
        { userId: member.userId, role: nextRole },
        {
          onSuccess: () => {
            toast({
              header: "Role updated",
              description: `${memberDisplayName(member)}'s role is now ${roleLabel(nextRole)}.`,
              variant: ToastVariant.SUCCESS,
            });
          },
          onError: (err) => {
            toast({
              header: "Role change failed",
              description: getErrorMessage(err, "Could not change role"),
              variant: ToastVariant.ERROR,
            });
          },
        },
      );
    },
    [setMemberRole, toast],
  );

  const handleRemove = useCallback(
    (member: Member) => {
      memberConfirm.handleOpen(member);
    },
    [memberConfirm],
  );

  const columns = useMemo(() => memberColumns(session.userId), [session.userId]);

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
        <Flex direction={FlexDirection.COLUMN} grow={1} basis={0} minHeight={0} fillWidth>
          <InfiniteTable<Member>
            columns={columns}
            data={sortedMembers}
            getRowId={(row) => row.userId}
            isLoading={membersQuery.isLoading}
            error={displayError}
            rowActions={(row) =>
              canManageTeam && row.userId !== session.userId ? (
                <>
                  <MenuItem label="Change role" icon={UserGearIcon} isDisabled={isMutatingMembers}>
                    <MenuRadioGroup
                      label="Role"
                      options={SETTINGS_ROLE_MENU_OPTIONS}
                      value={roleToOptionId(row.role)}
                      onChange={(id) => handleRoleChange(row, optionIdToRole(id))}
                    />
                  </MenuItem>
                  <MenuSeparator />
                  <MenuItem
                    label="Remove member"
                    icon={TrashIcon}
                    variant={MenuItemVariant.ERROR}
                    onSelect={() => handleRemove(row)}
                    isDisabled={isMutatingMembers}
                  />
                </>
              ) : null
            }
            ariaLabel="Team members"
          />
        </Flex>
      </SettingsPanelLayout>
      <ConfirmDialog
        isOpen={memberConfirm.isOpen}
        onOpenChange={(isOpen) => {
          if (!isOpen) memberConfirm.handleClose();
        }}
        onConfirm={memberConfirm.handleConfirm}
        header="Remove team member?"
        description="This member will immediately lose access to the organization and its resources."
        confirmValue={memberConfirm.target && memberDisplayName(memberConfirm.target)}
        label="Remove member"
        isDestructive
      />
    </>
  );
};

export default SettingsTeamPanel;
