import { type FC, useCallback, useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { PlusIcon, TrashIcon, UserGearIcon } from "@phosphor-icons/react";

import Avatar, { AvatarSize } from "@galaxy-io/dls/avatar/Avatar";
import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import PageLayout from "@galaxy-io/dls/layout/PageLayout";
import { MenuItem, MenuItemVariant, MenuRadioGroup, MenuSeparator } from "@galaxy-io/dls/menu/Menu";
import ConfirmDialog from "@galaxy-io/dls/modal/ConfirmDialog";
import InfiniteTable from "@galaxy-io/dls/table/InfiniteTable";
import type { TableColumn } from "@galaxy-io/dls/table/types";
import Text, { TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import {
  type Member,
  RemoveMemberRequestSchema,
  Role,
  SetMemberRoleRequestSchema,
} from "@/gen/auth/v1/members_pb";
import type { GetSessionResponse } from "@/gen/auth/v1/session_pb";

import SettingsTeamInviteModal from "@/pages/settings/components/team/SettingsTeamInviteModal";
import {
  SETTINGS_ROLE_OPTIONS,
  SETTINGS_ROLE_TO_CHIP_PROPS_MAP,
  SETTINGS_ROLE_TO_LABEL_MAP,
  SETTINGS_TEAM_TABLE_COLUMN_MIN_WIDTH_EMAIL,
  SETTINGS_TEAM_TABLE_COLUMN_MIN_WIDTH_NAME,
  SETTINGS_TEAM_TABLE_COLUMN_WIDTH_ROLE,
} from "@/pages/settings/constants";
import { SettingsTeamView } from "@/pages/settings/types";
import { formatMemberName } from "@/pages/settings/utils";

import { useFilamentSearchUpdate, useTeamSearch } from "@/module/hooks";
import type { TeamSearch } from "@/module/schemas";

import {
  useGetSessionQuery,
  useRemoveMemberMutation,
  useSetMemberRoleMutation,
  useSuspenseListMembersQuery,
} from "@/api/queries/auth";

import { useConfirm } from "@/hooks/useConfirm";
import { useOverlaySession } from "@/hooks/useOverlaySession";

import { getErrorMessage } from "@/utils/errors";
import { mapOptionIdToEnum } from "@/utils/select";

const createSettingsTeamColumns = (
  userId: GetSessionResponse["userId"] | undefined,
): TableColumn<Member>[] => [
  {
    id: "name",
    header: "Name",
    accessor: (row) => formatMemberName(row),
    isRowHeader: true,
    canSort: true,
    minWidth: SETTINGS_TEAM_TABLE_COLUMN_MIN_WIDTH_NAME,
    cell: ({ row }) => (
      <Flex gap={8} alignItems={AlignItems.CENTER} minWidth={0}>
        <Avatar size={AvatarSize.SMALL} seed={row.userId} name={formatMemberName(row)} isSquare />
        <Text lineClamp={1} shouldTooltipOnOverflow>
          {formatMemberName(row)}
        </Text>
        {userId !== undefined && row.userId === userId && (
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
    accessor: (row) => SETTINGS_ROLE_TO_LABEL_MAP[row.role],
    width: SETTINGS_TEAM_TABLE_COLUMN_WIDTH_ROLE,
    canSort: true,
    cell: ({ row }) => (
      <Chip
        label={SETTINGS_ROLE_TO_LABEL_MAP[row.role]}
        size={ChipSize.SMALL}
        {...SETTINGS_ROLE_TO_CHIP_PROPS_MAP[row.role]}
      />
    ),
  },
];

const TeamPage: FC = () => {
  const { toast } = useToast();
  const updateSearch = useFilamentSearchUpdate<TeamSearch>();
  const { view, inviteToken } = useTeamSearch();

  const { data: session } = useGetSessionQuery();
  const { data: membersData } = useSuspenseListMembersQuery();
  const canManageTeam = membersData.canManage;
  const { mutate: setMemberRole, isPending: isSettingRole } = useSetMemberRoleMutation();
  const { mutate: removeMember, isPending: isRemovingMember } = useRemoveMemberMutation();

  const isMutatingMembers = isSettingRole || isRemovingMember;
  const memberConfirm = useConfirm<Member>({
    entityLabel: "Team member",
    entityName: formatMemberName,
    messages: {
      successHeader: "Team member removed",
      successSubheader: (member) =>
        `${formatMemberName(member)} no longer has access to this organization.`,
      errorHeader: "Remove failed",
      errorFallback: "Could not remove team member",
    },
    onConfirm: (member, { onSuccess, onError }) =>
      removeMember(create(RemoveMemberRequestSchema, { userId: member.userId }), {
        onSuccess,
        onError,
      }),
  });

  const isInviteOpen =
    canManageTeam &&
    (view === SettingsTeamView.INVITE || (view === SettingsTeamView.LINK && !!inviteToken));

  const inviteSession = useOverlaySession(isInviteOpen);

  const handleViewChange = (nextView: SettingsTeamView | undefined, token?: string) => {
    void updateSearch((prev) => ({ ...prev, view: nextView, inviteToken: token }));
  };

  const sortedMembers = useMemo(() => {
    const members = membersData.members;
    return [...members].sort((a, b) => {
      if (a.userId === session?.userId) return -1;
      if (b.userId === session?.userId) return 1;
      return formatMemberName(a).localeCompare(formatMemberName(b));
    });
  }, [membersData.members, session?.userId]);

  const handleRoleChange = useCallback(
    (member: Member, nextRole: Role) => {
      if (nextRole === member.role || nextRole === Role.UNSPECIFIED) return;
      setMemberRole(create(SetMemberRoleRequestSchema, { userId: member.userId, role: nextRole }), {
        onSuccess: () => {
          toast({
            header: "Role updated",
            description: `${formatMemberName(member)}'s role is now ${SETTINGS_ROLE_TO_LABEL_MAP[nextRole]}.`,
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
      });
    },
    [setMemberRole, toast],
  );

  const columns = useMemo(() => createSettingsTeamColumns(session?.userId), [session?.userId]);

  return (
    <>
      <PageLayout
        header="Team"
        actions={
          canManageTeam && (
            <Button
              label="Invite member"
              icon={PlusIcon}
              variant={ButtonVariant.PRIMARY}
              onClick={() => handleViewChange(SettingsTeamView.INVITE)}
            />
          )
        }
      >
        <Flex direction={FlexDirection.COLUMN} grow={1} basis={0} minHeight={0} fillWidth>
          <InfiniteTable<Member>
            columns={columns}
            data={sortedMembers}
            getRowId={(row) => row.userId}
            rowActions={(row) =>
              canManageTeam && row.userId !== session?.userId ? (
                <>
                  <MenuItem label="Change role" icon={UserGearIcon} isDisabled={isMutatingMembers}>
                    <MenuRadioGroup
                      label="Role"
                      options={SETTINGS_ROLE_OPTIONS}
                      value={String(row.role)}
                      onChange={(id) => handleRoleChange(row, mapOptionIdToEnum(Role, id))}
                    />
                  </MenuItem>
                  <MenuSeparator />
                  <MenuItem
                    label="Remove member"
                    icon={TrashIcon}
                    variant={MenuItemVariant.ERROR}
                    onSelect={() => memberConfirm.handleOpen(row)}
                    isDisabled={isMutatingMembers}
                  />
                </>
              ) : null
            }
            ariaLabel="Team members"
          />
        </Flex>
      </PageLayout>
      <ConfirmDialog
        isOpen={memberConfirm.isOpen}
        onOpenChange={(isOpen) => {
          if (!isOpen) memberConfirm.handleClose();
        }}
        onConfirm={memberConfirm.handleConfirm}
        header="Remove team member?"
        description="This member will immediately lose access to the organization and its resources."
        confirmValue={memberConfirm.target && formatMemberName(memberConfirm.target)}
        label="Remove member"
        isDestructive
      />
      <SettingsTeamInviteModal
        key={inviteSession}
        isOpen={isInviteOpen}
        view={view}
        inviteToken={inviteToken}
        onViewChange={handleViewChange}
        onClose={() => handleViewChange(undefined)}
      />
    </>
  );
};

export default TeamPage;
