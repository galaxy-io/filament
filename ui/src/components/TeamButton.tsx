import { useCallback, useEffect, useMemo, useState } from "react";

import { styled } from "@linaria/react";
import { PlusIcon, TrashIcon, UsersThreeIcon } from "@phosphor-icons/react";

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

import BaseHeader from "@/layouts/components/BaseHeader";

import { API_URL } from "@/constants";

import { getAccessToken, getProfile } from "@/auth/token";

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

interface Member {
  id: string;
  name: string;
  email: string;
  state: string;
  role: string;
}

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

const initials = (name: string): string =>
  name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase() ?? "")
    .join("");

const MemberAvatar = ({ name }: { name: string }) => (
  <InitialsCircle>
    <Text size={TextSize.CAPTION} variant={TextVariant.SECONDARY}>
      {initials(name)}
    </Text>
  </InitialsCircle>
);

const ROLE_OPTIONS: SelectInputOption[] = [
  { id: "admin", label: "Admin", value: "admin" },
  { id: "creator", label: "Creator", value: "creator" },
  { id: "viewer", label: "Viewer", value: "viewer" },
];

// Invites offer every role and default to creator.
const INVITE_DEFAULT_ROLE =
  ROLE_OPTIONS.find((option) => option.id === "creator") ?? ROLE_OPTIONS[0];

const memberColumns = (
  myID: string | undefined,
  canManage: boolean,
  onRoleChange: (member: Member, role: string) => void,
  onRemove: (member: Member) => void,
): ColumnDef<Member>[] => {
  const columns: ColumnDef<Member>[] = [
    {
      id: "name",
      header: "Name",
      accessorFn: (row) => row.name,
      cellLoading: () => (
        <FlexWrapper gap={12} alignItems={AlignItems.CENTER}>
          <TextShimmer width={120} height={16} />
        </FlexWrapper>
      ),
      cell: ({ row }) => (
        <FlexWrapper gap={12} alignItems={AlignItems.CENTER}>
          <FlexItem grow={0} shrink={0} display="flex">
            <MemberAvatar name={row.original.name} />
          </FlexItem>
          <Text weight={TextWeight.MEDIUM} isEllipsis>
            {row.original.name}
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
      cell: ({ row }) => (
        <FlexWrapper gap={8} alignItems={AlignItems.CENTER}>
          {myID !== undefined && row.original.id === myID && (
            <FlexItem grow={0} shrink={0} display="flex">
              <Beacon variant={BeaconVariant.PRIMARY} isPulse />
            </FlexItem>
          )}
          <FlexWrapper gap={8} alignItems={AlignItems.CENTER}>
            <SelectInput
              options={ROLE_OPTIONS}
              value={ROLE_OPTIONS.find((option) => option.id === row.original.role) ?? null}
              onChange={(option) => onRoleChange(row.original, option.id)}
              width={120}
              dropdownWidth={160}
              isDisabled={!canManage}
            />
            {canManage && (
              <Button
                icon={TrashIcon}
                variant={ButtonVariant.SECONDARY}
                size={ButtonSize.SMALL}
                onClick={() => onRemove(row.original)}
                isDisabled={row.original.id === myID}
              />
            )}
          </FlexWrapper>
        </FlexWrapper>
      ),
    },
  ];
  return columns;
};

type View = "members" | "invite" | "link";

const authHeaders = () => ({
  "Content-Type": "application/json",
  Authorization: `Bearer ${getAccessToken()}`,
});

// TeamButton opens the team dialog: the org's members with editable roles,
// and an invite flow that hands back a shareable link. The link is the
// invite; nothing is emailed. Hidden entirely when auth is disabled.
const TeamButton = () => {
  const [open, setOpen] = useState(false);
  const [view, setView] = useState<View>("members");
  const [members, setMembers] = useState<Member[] | undefined>(undefined);
  const [canManage, setCanManage] = useState(false);
  const [email, setEmail] = useState("");
  const [givenName, setGivenName] = useState("");
  const [familyName, setFamilyName] = useState("");
  const [role, setRole] = useState<SelectInputOption>(INVITE_DEFAULT_ROLE);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | undefined>(undefined);
  const [inviteLink, setInviteLink] = useState<string | undefined>(undefined);
  // Captured on open: the session profile is wired up after first render,
  // so a mount-time snapshot would miss it.
  const [myID, setMyID] = useState<string | undefined>(undefined);

  const loadMembers = useCallback(async () => {
    try {
      const res = await fetch(`${API_URL}/auth/members`, {
        headers: { Authorization: `Bearer ${getAccessToken()}` },
      });
      if (!res.ok) {
        setError(await res.text());
        return;
      }
      const { members, canManage } = (await res.json()) as {
        members: Member[];
        canManage: boolean;
      };
      setMembers(members);
      setCanManage(canManage);
    } catch {
      setError("Could not load members");
    }
  }, []);

  // Both mutations update local state on success instead of refetching:
  // Zitadel's projections are eventually consistent, so an immediate reload
  // can race them and resurrect stale rows.
  const handleRoleChange = useCallback(async (member: Member, newRole: string) => {
    setError(undefined);
    const res = await fetch(`${API_URL}/auth/members/role`, {
      method: "POST",
      headers: authHeaders(),
      body: JSON.stringify({ userId: member.id, role: newRole }),
    }).catch(() => undefined);
    if (!res?.ok) {
      setError(res ? await res.text() : "Could not change role");
      return;
    }
    setMembers((prev) => prev?.map((m) => (m.id === member.id ? { ...m, role: newRole } : m)));
  }, []);

  const handleRemove = useCallback(async (member: Member) => {
    setError(undefined);
    const res = await fetch(`${API_URL}/auth/members/remove`, {
      method: "POST",
      headers: authHeaders(),
      body: JSON.stringify({ userId: member.id }),
    }).catch(() => undefined);
    if (!res?.ok) {
      setError(res ? await res.text() : "Could not remove member");
      return;
    }
    setMembers((prev) => prev?.filter((m) => m.id !== member.id));
  }, []);

  const columns = useMemo(
    () => memberColumns(myID, canManage, handleRoleChange, handleRemove),
    [myID, canManage, handleRoleChange, handleRemove],
  );

  // Current identity first, then everyone else by name.
  const sortedMembers = useMemo(() => {
    if (members === undefined) {
      return undefined;
    }
    return [...members].sort((a, b) => {
      if (a.id === myID) return -1;
      if (b.id === myID) return 1;
      return a.name.localeCompare(b.name);
    });
  }, [members, myID]);

  useEffect(() => {
    if (open && view === "members") {
      void loadMembers();
    }
  }, [open, view, loadMembers]);

  if (!getAccessToken()) {
    return null;
  }

  const resetForm = () => {
    setEmail("");
    setGivenName("");
    setFamilyName("");
    setRole(INVITE_DEFAULT_ROLE);
    setError(undefined);
    setInviteLink(undefined);
  };

  // Members stay cached across opens; reopening shows the last list
  // instantly while a background refresh replaces it.
  const handleClose = () => {
    setOpen(false);
    setView("members");
    resetForm();
    setPending(false);
  };

  const handleSubmit = async () => {
    if (pending) {
      return;
    }
    if (email === "" || givenName === "" || familyName === "") {
      setError("All fields are required");
      return;
    }
    setPending(true);
    setError(undefined);
    try {
      const res = await fetch(`${API_URL}/auth/invite`, {
        method: "POST",
        headers: authHeaders(),
        body: JSON.stringify({ email, givenName, familyName, role: role.value }),
      });
      if (!res.ok) {
        setError(await res.text());
        return;
      }
      const { userId, inviteCode } = (await res.json()) as { userId: string; inviteCode: string };
      const token = btoa(`${userId}:${inviteCode}`).replace(/=+$/, "");
      setInviteLink(`${window.location.origin}/invite/${token}`);
      setView("link");
    } catch {
      setError("Something went wrong, please try again");
    } finally {
      setPending(false);
    }
  };

  return (
    <>
      <Button
        icon={UsersThreeIcon}
        variant={ButtonVariant.SECONDARY}
        size={ButtonSize.SMALL}
        onClick={() => {
          setMyID(getProfile()?.sub);
          setOpen(true);
        }}
        onMouseEnter={() => {
          // Warm the list before the click so the dialog opens populated.
          if (members === undefined) {
            void loadMembers();
          }
        }}
        isIconFilled
      />
      <Modal open={open} onClose={handleClose}>
        <DialogWrapper $wide={view !== "invite"}>
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
                    View and manage your team members
                  </Text>
                  <Button
                    label="Invite team"
                    icon={PlusIcon}
                    variant={ButtonVariant.PRIMARY}
                    onClick={() => {
                      setError(undefined);
                      setView("invite");
                    }}
                  />
                </FlexWrapper>
                <TableWrapper>
                  <InfiniteTable<Member>
                    columns={columns}
                    data={sortedMembers ?? []}
                    getRowId={(row) => row.id}
                    isLoading={members === undefined}
                    loadingRowCount={3}
                    enableSorting
                    noLastRowBorder
                    noLastRowPadding
                  />
                </TableWrapper>
                {error && (
                  <Text size={TextSize.CAPTION} variant={TextVariant.ERROR}>
                    {error}
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
                {error && (
                  <Text size={TextSize.CAPTION} variant={TextVariant.ERROR}>
                    {error}
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
                      isDisabled={pending}
                    />
                    <Button
                      size={ButtonSize.LARGE}
                      label="Create invite"
                      onClick={() => void handleSubmit()}
                      isLoading={pending}
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

export default TeamButton;
