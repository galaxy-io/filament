import { useCallback, useMemo } from "react";

import { PlusIcon, TrashIcon } from "@phosphor-icons/react";

import Avatar from "@galaxy-io/dls/avatar/Avatar";
import Beacon, { BeaconVariant } from "@galaxy-io/dls/beacons/Beacon";
import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Skeleton from "@galaxy-io/dls/feedback/Skeleton";
import SelectInput from "@galaxy-io/dls/inputs/SelectInput";
import Box, { BoxVariant } from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import InfiniteTable from "@galaxy-io/dls/table/InfiniteTable";
import type { TableColumn } from "@galaxy-io/dls/table/types";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

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
import { optionRole, roleLabel, roleOption } from "@/pages/settings/utils";

import {
  useListMembersQuery,
  useRemoveMemberMutation,
  useSetMemberRoleMutation,
} from "@/api/queries/auth";

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
}): TableColumn<Member>[] => {
  return [
    {
      id: "name",
      header: "Name",
      accessorFn: (row) => memberDisplayName(row),
      enableSorting: true,
      sortDescFirst: false,
      cellLoading: () => (
        <Flex gap={12} alignItems={AlignItems.CENTER}>
          <Box width={120}>
            <Skeleton />
          </Box>
        </Flex>
      ),
      cell: ({ row }) => (
        <Flex gap={12} alignItems={AlignItems.CENTER}>
          <FlexItem
            grow={0}
            shrink={
              0
            } /* @dls-migrate flexitem.display: A FlexItem is not a flex container: use `<Flex grow={1}>` or nest a `Flex`. */
            display="flex"
          >
            <Avatar size={26} seed={row.original.userId} />
          </FlexItem>
          <Text weight={TextWeight.MEDIUM} lineClamp={1}>
            {memberDisplayName(row.original)}
          </Text>
        </Flex>
      ),
    },
    {
      id: "email",
      header: "Email",
      accessorFn: (row) => row.email,
      size: SETTINGS_TEAM_TABLE_COLUMN_WIDTH_EMAIL,
      enableSorting: true,
      sortDescFirst: false,
      cellLoading: () => (
        <Box width="80%">
          <Skeleton />
        </Box>
      ),
      cell: ({ row }) => (
        <Text variant={TextVariant.SECONDARY} lineClamp={1}>
          {row.original.email}
        </Text>
      ),
    },
    {
      id: "role",
      header: "",
      align: "right",
      size: SETTINGS_TEAM_TABLE_COLUMN_WIDTH_ROLE,
      enableSorting: false,
      cellLoading: () => (
        <Box width="80%">
          <Skeleton />
        </Box>
      ),
      cell: ({ row }) => {
        const isMe = myID !== undefined && row.original.userId === myID;
        return (
          <Flex gap={8} alignItems={AlignItems.CENTER}>
            {isMe && (
              <FlexItem
                grow={0}
                shrink={
                  0
                } /* @dls-migrate flexitem.display: A FlexItem is not a flex container: use `<Flex grow={1}>` or nest a `Flex`. */
                display="flex"
              >
                <Beacon variant={BeaconVariant.PRIMARY} isPulse />
              </FlexItem>
            )}
            <Flex gap={8} alignItems={AlignItems.CENTER}>
              <Box width={SETTINGS_TEAM_TABLE_ROLE_SELECT_WIDTH}>
                <SelectInput
                  fillWidth
                  options={ROLE_OPTIONS}
                  /* @dls-migrate selectinput.value: `value` and `onChange` now carry option ids, not option objects. */ value={roleOption(
                    row.original.role,
                  )}
                  onChange={(option) => {
                    const role = optionRole(option);
                    if (role !== undefined) {
                      onRoleChange(row.original, role);
                    }
                  }}
                  isDisabled={!canManage || isMe || isMutatingMembers}
                />
              </Box>
              {canManage && (
                <Button
                  icon={TrashIcon}
                  variant={ButtonVariant.SECONDARY}
                  size={ButtonSize.SMALL}
                  onClick={() => onRemove(row.original)}
                  isDisabled={isMe || isMutatingMembers}
                />
              )}
            </Flex>
          </Flex>
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
      if (nextRole === member.role) return;
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
        <Flex alignItems={AlignItems.START} grow={1} basis={0} minHeight={0} fillWidth>
          <Box variant={BoxVariant.BASE} height="100%">
            <InfiniteTable<Member>
              columns={columns}
              data={sortedMembers}
              getRowId={(row) => row.userId}
              isLoading={membersQuery.isLoading}
              /* @dls-migrate infinitetable.enableSorting: Sorting is per column (`canSort`) with a `TableSort` value. */ enableSorting
            />
          </Box>
        </Flex>
        {displayError && (
          <Flex alignItems={AlignItems.START} padding={16} fillWidth>
            <Text size={TextSize.CAPTION} variant={TextVariant.ERROR}>
              {displayError}
            </Text>
          </Flex>
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
