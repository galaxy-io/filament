import { useCallback, useMemo, useState } from "react";

import { styled } from "@linaria/react";
import {
  GearFineIcon,
  PlusIcon,
  SignOutIcon,
  TrashIcon,
  UsersThreeIcon,
} from "@phosphor-icons/react";
import { useAuth } from "react-oidc-context";

import DotGridBackground from "@galaxy-io/dls/backgrounds/DotGridBackground";
import Beacon, { BeaconVariant } from "@galaxy-io/dls/beacons/Beacon";
import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Dropdown, { DropdownPosition } from "@galaxy-io/dls/dropdown/Dropdown";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import CopyInput from "@galaxy-io/dls/inputs/CopyInput";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import SelectInput, { type SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";
import SwitcherInput, { type SwitcherInputItem } from "@galaxy-io/dls/inputs/SwitcherInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Modal from "@galaxy-io/dls/modal/Modal";
import InfiniteTable, { ColumnAlign, type ColumnDef } from "@galaxy-io/dls/table/InfiniteTable";
import Paragraph from "@galaxy-io/dls/text/Paragraph";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";
import { useGalaxyTheme, withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import { GalaxyTheme, type PropsWithTheme } from "@galaxy-io/dls/theme/types";

import type { Member, Role } from "@/gen/auth/v1/members_pb";

import {
  INVITE_DEFAULT_ROLE,
  optionRole,
  ROLE_OPTIONS,
  roleLabel,
  roleOption,
} from "@/components/settings/constants";
import TooltipIconButton from "@/components/TooltipIconButton";

import BaseHeader from "@/layouts/components/BaseHeader";

import {
  useInviteMemberMutation,
  useRemoveMemberMutation,
  useSetMemberRoleMutation,
} from "@/api/mutations/auth";
import { useListMembersQuery } from "@/api/queries/auth";

import { encodeInviteToken } from "@/auth/inviteToken";
import { type AppSession, useAppSession } from "@/auth/session";
import { getErrorMessage } from "@/utils/errors";

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

const SettingsMenuWrapper = styled.div`
  display: flex;
  flex-direction: column;
  width: 100%;
  overflow: hidden;
`;

const SettingsMenuHeader = withTheme(styled.div<PropsWithTheme>`
  width: 100%;
  background-color: ${({ theme }) => theme.color.background.primary};
  border-radius: 5px 5px 0 0;
  overflow: hidden;
`);

const SettingsMenuHeaderContent = styled.div`
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  width: 100%;
  min-width: 0;
  padding: 16px;
`;

const SettingsMenuSection = styled.div`
  display: flex;
  flex-direction: column;
  width: 100%;
  padding: 6px;
`;

const SettingsMenuFooter = styled.div`
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  width: 100%;
  padding: 6px;
`;

const ThemeSwitcherWrapper = styled.div`
  min-width: 0;
  flex-shrink: 0;
`;

const SettingsMenuItem = withTheme(styled.button<PropsWithTheme>`
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  min-height: 36px;
  padding: 6px 10px;
  border: 0;
  border-radius: 5px;
  background-color: transparent;
  text-align: left;
  cursor: pointer;
  transition: background-color 100ms ease;

  &:hover,
  &:focus-visible {
    background-color: ${({ theme }) => theme.color.background.tertiary};
  }

  &:focus-visible {
    outline: 1px solid ${({ theme }) => theme.color.border.secondary};
    outline-offset: 1px;
  }
`);

const SettingsMenuIconWrapper = withTheme(styled.div<PropsWithTheme>`
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 16px;
  height: 16px;
`);

const TableWrapper = withTheme(styled.div<PropsWithTheme>`
  width: 100%;
  border-radius: 5px;
  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
`);

const InitialsCircle = withTheme(styled.div<PropsWithTheme>`
  width: 26px;
  height: 26px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  background-color: ${({ theme }) => theme.color.background.tertiary};
  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 50%;
`);

const LargeInitialsCircle = withTheme(styled.div<PropsWithTheme>`
  width: 36px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  background-color: ${({ theme }) => theme.color.background.secondary};
  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 50%;
`);

const initials = (name: string): string => {
  const value = name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase() ?? "")
    .join("");
  return value || "?";
};

const memberDisplayName = (member: Member): string => member.name || member.email || "Member";

const MemberAvatar = ({ member }: { member: Member }) => (
  <InitialsCircle>
    <Text size={TextSize.CAPTION} variant={TextVariant.SECONDARY}>
      {initials(memberDisplayName(member))}
    </Text>
  </InitialsCircle>
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

type View = "members" | "invite" | "link";

const SETTINGS_DROPDOWN_ID = "settings-dropdown-menu";
const TEAM_SETTINGS_DIALOG_ID = "team-settings-dialog";

interface SettingsDropdownMenuProps {
  name: string;
  email?: string;
  role?: Role;
  onOpenTeamSettings: () => void;
  onLogout: () => void;
  themeItems: SwitcherInputItem[];
  selectedTheme: GalaxyTheme;
}

const SettingsDropdownMenu = ({
  name,
  email,
  role,
  onOpenTeamSettings,
  onLogout,
  themeItems,
  selectedTheme,
}: SettingsDropdownMenuProps) => {
  const { theme } = useGalaxyTheme();
  const displayRole = roleLabel(role);

  return (
    <SettingsMenuWrapper id={SETTINGS_DROPDOWN_ID}>
      <SettingsMenuHeader>
        <DotGridBackground
          dotSize={2}
          dotOpacity={0.25}
          spacing={8}
          backgroundColor={theme.color.background.primary}
          fillContainer={false}
        >
          <SettingsMenuHeaderContent>
            <LargeInitialsCircle>
              <Text size={TextSize.BODY_LG} weight={TextWeight.MEDIUM}>
                {initials(name)}
              </Text>
            </LargeInitialsCircle>
            <FlexWrapper
              direction={FlexDirection.COLUMN}
              alignItems={AlignItems.CENTER}
              gap={8}
              fillWidth
              minWidth={0}
            >
              <FlexWrapper
                direction={FlexDirection.COLUMN}
                alignItems={AlignItems.CENTER}
                gap={2}
                fillWidth
                minWidth={0}
              >
                <Text size={TextSize.BODY_LG} weight={TextWeight.MEDIUM} align="center" isEllipsis>
                  {name}
                </Text>
                {email && (
                  <Text
                    size={TextSize.BODY_SM}
                    variant={TextVariant.SECONDARY}
                    align="center"
                    isEllipsis
                  >
                    {email}
                  </Text>
                )}
              </FlexWrapper>
              {displayRole && (
                <Chip label={displayRole} size={ChipSize.SMALL} variant={ChipVariant.SECONDARY} />
              )}
            </FlexWrapper>
          </SettingsMenuHeaderContent>
        </DotGridBackground>
      </SettingsMenuHeader>
      <HorizontalDivider />
      <SettingsMenuSection>
        <SettingsMenuItem type="button" onClick={onOpenTeamSettings}>
          <SettingsMenuIconWrapper>
            <Icon component={UsersThreeIcon} size={14} variant={IconVariant.TERTIARY} />
          </SettingsMenuIconWrapper>
          <Text weight={TextWeight.MEDIUM}>Manage team</Text>
        </SettingsMenuItem>
      </SettingsMenuSection>
      <HorizontalDivider />
      <SettingsMenuFooter>
        <ThemeSwitcherWrapper>
          <SwitcherInput items={themeItems} selectedId={selectedTheme} size={InputSize.SMALL} />
        </ThemeSwitcherWrapper>
        <Button
          label="Logout"
          icon={SignOutIcon}
          variant={ButtonVariant.TERTIARY}
          size={ButtonSize.SMALL}
          onClick={onLogout}
        />
      </SettingsMenuFooter>
    </SettingsMenuWrapper>
  );
};

const AuthenticatedSettingsButton = ({ session }: { session: AppSession }) => {
  const auth = useAuth();
  const { selectedTheme, setTheme } = useGalaxyTheme();
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [teamOpen, setTeamOpen] = useState(false);
  const [view, setView] = useState<View>("members");
  const [email, setEmail] = useState("");
  const [givenName, setGivenName] = useState("");
  const [familyName, setFamilyName] = useState("");
  const [role, setRole] = useState<SelectInputOption>(INVITE_DEFAULT_ROLE);
  const [error, setError] = useState<string | undefined>(undefined);
  const [inviteLink, setInviteLink] = useState<string | undefined>(undefined);

  const membersQuery = useListMembersQuery({
    options: { enabled: (settingsOpen || teamOpen) && !!session.accessToken },
  });
  const { mutate: inviteMember, isPending: isInviting } = useInviteMemberMutation();
  const { mutate: setMemberRole, isPending: isSettingRole } = useSetMemberRoleMutation();
  const { mutate: removeMember, isPending: isRemovingMember } = useRemoveMemberMutation();

  const members = membersQuery.data?.members ?? [];
  const currentMember = members.find((member) => member.userId === session.userId);
  const canManage = membersQuery.data?.canManage ?? false;
  const isMutatingMembers = isSettingRole || isRemovingMember;
  const profileName =
    (currentMember ? memberDisplayName(currentMember) : session.name || session.email) ?? "Member";
  const profileEmail = currentMember?.email || session.email;
  const displayError =
    error ??
    (membersQuery.error
      ? getErrorMessage(membersQuery.error, "Could not load members")
      : undefined);

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
    setInviteLink(undefined);
  }, []);

  const handleClose = useCallback(() => {
    setTeamOpen(false);
    setView("members");
    resetForm();
  }, [resetForm]);

  const handleSettingsClose = useCallback(() => {
    setSettingsOpen(false);
  }, []);

  const handleOpenTeamSettings = useCallback(() => {
    setSettingsOpen(false);
    setView("members");
    resetForm();
    setTeamOpen(true);
  }, [resetForm]);

  const themeItems = useMemo(
    () => [
      {
        id: GalaxyTheme.LIGHT,
        label: "Light",
        onClick: () => setTheme(GalaxyTheme.LIGHT),
      },
      {
        id: GalaxyTheme.DARK,
        label: "Dark",
        onClick: () => setTheme(GalaxyTheme.DARK),
      },
      {
        id: GalaxyTheme.SYSTEM,
        label: "System",
        onClick: () => setTheme(GalaxyTheme.SYSTEM),
      },
    ],
    [setTheme],
  );

  const handleLogout = useCallback(() => {
    setSettingsOpen(false);
    void auth.signoutRedirect();
  }, [auth]);

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
        canManage,
        isMutatingMembers,
        onRoleChange: handleRoleChange,
        onRemove: handleRemove,
      }),
    [session.userId, canManage, isMutatingMembers, handleRoleChange, handleRemove],
  );

  const handleSubmit = () => {
    if (isInviting) {
      return;
    }
    if (!canManage) {
      setError("Only admins can invite teammates");
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
          setInviteLink(`${window.location.origin}/invite/${token}`);
          setView("link");
        },
        onError: (err) => {
          setError(getErrorMessage(err, "Something went wrong, please try again"));
        },
      },
    );
  };

  return (
    <>
      <Dropdown
        position={DropdownPosition.BOTTOM_END}
        minWidth={260}
        maxWidth={260}
        noPadding
        isOpen={settingsOpen}
        onClose={handleSettingsClose}
        body={
          <SettingsDropdownMenu
            name={profileName}
            email={profileEmail}
            role={currentMember?.role}
            onOpenTeamSettings={handleOpenTeamSettings}
            onLogout={handleLogout}
            themeItems={themeItems}
            selectedTheme={selectedTheme}
          />
        }
      >
        <TooltipIconButton
          label="Settings"
          icon={GearFineIcon}
          onClick={() => {
            setSettingsOpen((isOpen) => !isOpen);
          }}
          isIconFilled
          isTooltipDisabled={settingsOpen}
          ariaExpanded={settingsOpen}
          ariaControls={SETTINGS_DROPDOWN_ID}
          ariaHasPopup
        />
      </Dropdown>
      <Modal open={teamOpen} onClose={handleClose}>
        <DialogWrapper id={TEAM_SETTINGS_DIALOG_ID} $wide={view !== "invite"}>
          <FlexWrapper padding={"16px"}>
            <BaseHeader title="Team" onClose={handleClose} />
          </FlexWrapper>

          <FlexItem grow={0} shrink={0}>
            <HorizontalDivider />
          </FlexItem>

          <BodyWrapper>
            {view === "members" && (
              <FlexWrapper direction={FlexDirection.COLUMN} gap={12} fillWidth>
                <FlexWrapper
                  alignItems={AlignItems.CENTER}
                  justifyContent={JustifyContent.SPACE_BETWEEN}
                  fillWidth
                >
                  <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
                    {canManage ? "View and manage your team members" : "View your team members"}
                  </Text>
                  {canManage && (
                    <Button
                      label="Invite team"
                      icon={PlusIcon}
                      variant={ButtonVariant.PRIMARY}
                      onClick={() => {
                        setError(undefined);
                        setView("invite");
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

            {view === "invite" && (
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
                <TextInput
                  label="Last name"
                  value={familyName}
                  onChange={setFamilyName}
                  fillWidth
                />
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

            {view === "link" && inviteLink && (
              <>
                <FlexWrapper direction={FlexDirection.COLUMN} gap={4} fillWidth>
                  <Text size={TextSize.BODY_LG} weight={TextWeight.MEDIUM}>
                    Data is a team sport
                  </Text>
                  <Paragraph variant={TextVariant.SECONDARY}>
                    Send {givenName} their invite and start shipping pipelines together.
                  </Paragraph>
                </FlexWrapper>
                <CopyInput value={inviteLink} fillWidth isMonospace />
              </>
            )}
          </BodyWrapper>

          {view !== "members" && (
            <>
              <FlexItem grow={0} shrink={0}>
                <HorizontalDivider />
              </FlexItem>

              <FlexWrapper
                justifyContent={view === "link" ? JustifyContent.SPACE_BETWEEN : JustifyContent.END}
                alignItems={AlignItems.CENTER}
                padding={"16px"}
                gap={8}
                fillWidth
              >
                {view === "invite" && (
                  <>
                    <Button
                      size={ButtonSize.LARGE}
                      label="Cancel"
                      variant={ButtonVariant.SECONDARY}
                      onClick={() => {
                        resetForm();
                        setView("members");
                      }}
                      isDisabled={isInviting}
                    />
                    <Button
                      size={ButtonSize.LARGE}
                      label="Create invite"
                      onClick={handleSubmit}
                      isLoading={isInviting}
                    />
                  </>
                )}
                {view === "link" && (
                  <>
                    <Button
                      size={ButtonSize.LARGE}
                      label="Add another teammate"
                      variant={ButtonVariant.SECONDARY}
                      onClick={() => {
                        resetForm();
                        setView("invite");
                      }}
                    />
                    <Button size={ButtonSize.LARGE} label="Done" onClick={handleClose} />
                  </>
                )}
              </FlexWrapper>
            </>
          )}
        </DialogWrapper>
      </Modal>
    </>
  );
};

const SettingsButton = () => {
  const session = useAppSession();

  if (!session.isAuthEnabled || !session.accessToken) {
    return null;
  }

  return <AuthenticatedSettingsButton session={session} />;
};

export default SettingsButton;
