import { useState } from "react";

import { styled } from "@linaria/react";
import {
  ArrowsClockwiseIcon,
  DotsThreeVerticalIcon,
  PlusIcon,
  TrashIcon,
} from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Dropdown, { DropdownPosition } from "@galaxy-io/dls/dropdown/Dropdown";
import CopyInput from "@galaxy-io/dls/inputs/CopyInput";
import SelectInput, { type SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import InfiniteTable, { ColumnAlign, type ColumnDef } from "@galaxy-io/dls/table/InfiniteTable";
import Paragraph from "@galaxy-io/dls/text/Paragraph";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import type { ServiceAccount } from "@/gen/auth/v1/service_accounts_pb";

import {
  INVITE_DEFAULT_ROLE,
  optionRole,
  ROLE_OPTIONS,
  roleLabel,
} from "@/components/settings/constants";

import BaseHeader from "@/layouts/components/BaseHeader";

import {
  useCreateServiceAccountMutation,
  useRemoveServiceAccountMutation,
  useRotateServiceAccountSecretMutation,
} from "@/api/mutations/auth";
import { useListServiceAccountsQuery } from "@/api/queries/auth";

import type { AppSession } from "@/auth/session";
import { getErrorMessage } from "@/utils/errors";

const DialogWrapper = withTheme(styled.div<PropsWithTheme>`
  display: flex;
  flex-direction: column;
  width: 720px;
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
  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 5px;
`);

const AccountIdentity = styled.div`
  display: grid;
  grid-template-columns: 8px minmax(0, 1fr);
  align-items: center;
  column-gap: 10px;
  width: 100%;
  min-width: 0;
`;

const StatusDot = withTheme(styled.span<PropsWithTheme>`
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background-color: ${({ theme }) => theme.color.icon.success};
`);

const ActionsButton = withTheme(styled.button<PropsWithTheme>`
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 0;
  border: 0;
  border-radius: 5px;
  color: ${({ theme }) => theme.color.text.secondary};
  background: transparent;
  cursor: pointer;

  &:hover,
  &:focus-visible {
    color: ${({ theme }) => theme.color.text.primary};
    background-color: ${({ theme }) => theme.color.background.tertiary};
  }

  &:focus-visible {
    outline: 1px solid ${({ theme }) => theme.color.border.secondary};
  }

  &:disabled {
    opacity: 0.5;
    cursor: default;
  }
`);

const ActionsMenu = styled.div`
  display: flex;
  flex-direction: column;
  width: 100%;
  padding: 4px;
`;

const ActionsMenuItem = withTheme(styled.button<PropsWithTheme>`
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  min-height: 32px;
  padding: 6px 8px;
  border: 0;
  border-radius: 4px;
  color: ${({ theme }) => theme.color.text.primary};
  background: transparent;
  text-align: left;
  cursor: pointer;

  &:hover,
  &:focus-visible {
    background-color: ${({ theme }) => theme.color.background.tertiary};
  }

  &:focus-visible {
    outline: 1px solid ${({ theme }) => theme.color.border.secondary};
  }
`);

interface Credentials {
  clientId: string;
  clientSecret: string;
}

interface ServiceAccountsModalProps {
  session: AppSession;
  onClose: () => void;
}

const ServiceAccountsModal = ({ session, onClose }: ServiceAccountsModalProps) => {
  const [isCreating, setIsCreating] = useState(false);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [role, setRole] = useState<SelectInputOption>(INVITE_DEFAULT_ROLE);
  const [credentials, setCredentials] = useState<Credentials>();
  const [error, setError] = useState<string>();
  const [openActionsFor, setOpenActionsFor] = useState<string>();

  const accountsQuery = useListServiceAccountsQuery({
    options: { enabled: !!session.accessToken },
  });
  const { mutate: createAccount, isPending: isCreatingAccount } = useCreateServiceAccountMutation();
  const { mutate: rotateSecret, isPending: isRotating } = useRotateServiceAccountSecretMutation();
  const { mutate: removeAccount, isPending: isRemoving } = useRemoveServiceAccountMutation();
  const canManage = accountsQuery.data?.canManage === true;

  const showCredentials = (next: Credentials) => {
    setCredentials(next);
    setIsCreating(false);
    setError(undefined);
  };

  const handleCreate = () => {
    const accountRole = optionRole(role);
    if (name.trim() === "" || accountRole === undefined) {
      setError("Name and role are required");
      return;
    }
    setError(undefined);
    createAccount(
      { name: name.trim(), description: description.trim(), role: accountRole },
      {
        onSuccess: ({ serviceAccount, clientSecret }) => {
          if (serviceAccount) {
            showCredentials({ clientId: serviceAccount.clientId, clientSecret });
          }
        },
        onError: (err) => setError(getErrorMessage(err, "Could not create service account")),
      },
    );
  };

  const handleRotate = (account: ServiceAccount) => {
    if (
      !window.confirm(
        `Rotate the secret for ${account.name}? The current secret will stop working.`,
      )
    ) {
      return;
    }
    setError(undefined);
    rotateSecret(
      { userId: account.userId },
      {
        onSuccess: ({ clientId, clientSecret }) => showCredentials({ clientId, clientSecret }),
        onError: (err) => setError(getErrorMessage(err, "Could not rotate client secret")),
      },
    );
  };

  const handleRemove = (account: ServiceAccount) => {
    if (!window.confirm(`Remove ${account.name}? Its credentials will stop working immediately.`)) {
      return;
    }
    setError(undefined);
    removeAccount(
      { userId: account.userId },
      {
        onError: (err) => setError(getErrorMessage(err, "Could not remove service account")),
      },
    );
  };

  const columns: ColumnDef<ServiceAccount>[] = [
    {
      id: "name",
      header: "Service account",
      accessorFn: (account) => account.name,
      cellLoading: () => <TextShimmer width={120} height={16} />,
      cell: ({ row }) => (
        <AccountIdentity>
          <StatusDot />
          <FlexWrapper direction={FlexDirection.COLUMN} gap={2} minWidth={0}>
            <Text weight={TextWeight.MEDIUM} isEllipsis>
              {row.original.name}
            </Text>
            <Text size={TextSize.CAPTION} variant={TextVariant.SECONDARY} isEllipsis>
              {row.original.clientId}
            </Text>
          </FlexWrapper>
        </AccountIdentity>
      ),
    },
    {
      id: "description",
      header: "Description",
      accessorFn: (account) => account.description,
      size: 180,
      cellLoading: () => <TextShimmer width={100} height={16} />,
      cell: ({ row }) => (
        <Text variant={TextVariant.SECONDARY} isEllipsis>
          {row.original.description || "—"}
        </Text>
      ),
    },
    {
      id: "role",
      header: "Role",
      size: 90,
      cell: ({ row }) => <Text>{roleLabel(row.original.role) ?? "None"}</Text>,
    },
    {
      id: "actions",
      header: "",
      align: ColumnAlign.RIGHT,
      size: 44,
      cell: ({ row }) => (
        <Dropdown
          position={DropdownPosition.BOTTOM_END}
          minWidth={160}
          noPadding
          isOpen={openActionsFor === row.original.userId}
          onClose={() => setOpenActionsFor(undefined)}
          body={
            <ActionsMenu>
              <ActionsMenuItem
                type="button"
                onClick={() => {
                  setOpenActionsFor(undefined);
                  handleRotate(row.original);
                }}
              >
                <ArrowsClockwiseIcon size={16} />
                <Text>Rotate secret</Text>
              </ActionsMenuItem>
              <ActionsMenuItem
                type="button"
                onClick={() => {
                  setOpenActionsFor(undefined);
                  handleRemove(row.original);
                }}
              >
                <TrashIcon size={16} />
                <Text>Delete</Text>
              </ActionsMenuItem>
            </ActionsMenu>
          }
        >
          <ActionsButton
            type="button"
            aria-label={`Actions for ${row.original.name}`}
            onClick={() =>
              setOpenActionsFor((current) =>
                current === row.original.userId ? undefined : row.original.userId,
              )
            }
            disabled={!canManage || isRotating || isRemoving}
          >
            <DotsThreeVerticalIcon size={18} weight="bold" />
          </ActionsButton>
        </Dropdown>
      ),
    },
  ];

  const displayError =
    error ??
    (accountsQuery.error
      ? getErrorMessage(accountsQuery.error, "Could not load service accounts")
      : undefined);

  return (
    <DialogWrapper>
      <FlexWrapper padding="16px">
        <BaseHeader title="Service accounts" onClose={onClose} />
      </FlexWrapper>
      <HorizontalDivider />
      <BodyWrapper>
        {credentials ? (
          <FlexWrapper direction={FlexDirection.COLUMN} gap={12} fillWidth>
            <Text size={TextSize.BODY_LG} weight={TextWeight.MEDIUM}>
              Save these credentials now
            </Text>
            <Paragraph variant={TextVariant.SECONDARY}>
              The client secret is shown once and cannot be retrieved later. Store it in your
              password manager or CI secret store.
            </Paragraph>
            <CopyInput label="Client ID" value={credentials.clientId} fillWidth isMonospace />
            <CopyInput
              label="Client secret"
              value={credentials.clientSecret}
              fillWidth
              isMonospace
            />
            <CopyInput
              label="CLI command"
              value={`filament auth login --server ${window.location.origin} --client-id ${credentials.clientId}`}
              fillWidth
              isMonospace
            />
          </FlexWrapper>
        ) : isCreating ? (
          <FlexWrapper direction={FlexDirection.COLUMN} gap={12} fillWidth>
            <TextInput label="Name" value={name} onChange={setName} fillWidth autoFocus />
            <TextInput
              label="Description"
              value={description}
              onChange={setDescription}
              fillWidth
            />
            <SelectInput
              label="Role"
              options={ROLE_OPTIONS}
              value={role}
              onChange={setRole}
              fillWidth
            />
          </FlexWrapper>
        ) : (
          <FlexWrapper direction={FlexDirection.COLUMN} gap={12} fillWidth>
            <FlexWrapper
              alignItems={AlignItems.CENTER}
              justifyContent={JustifyContent.SPACE_BETWEEN}
              fillWidth
            >
              <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
                Credentials for the CLI, CI, and other automation
              </Text>
              {canManage && (
                <Button
                  label="Create service account"
                  icon={PlusIcon}
                  size={ButtonSize.SMALL}
                  onClick={() => setIsCreating(true)}
                />
              )}
            </FlexWrapper>
            <TableWrapper>
              <InfiniteTable<ServiceAccount>
                columns={columns}
                data={accountsQuery.data?.serviceAccounts ?? []}
                getRowId={(account) => account.userId}
                height={132}
                isLoading={accountsQuery.isLoading}
                loadingRowCount={2}
                contentWhenEmpty={
                  <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
                    No service accounts yet
                  </Text>
                }
                noLastRowBorder
                noLastRowPadding
              />
            </TableWrapper>
          </FlexWrapper>
        )}
        {displayError && (
          <Text size={TextSize.CAPTION} variant={TextVariant.ERROR}>
            {displayError}
          </Text>
        )}
      </BodyWrapper>
      {(credentials || isCreating) && (
        <>
          <FlexItem grow={0} shrink={0}>
            <HorizontalDivider />
          </FlexItem>
          <FlexWrapper justifyContent={JustifyContent.END} padding="16px" gap={8} fillWidth>
            {credentials ? (
              <Button
                size={ButtonSize.LARGE}
                label="Done"
                onClick={() => {
                  setCredentials(undefined);
                  setName("");
                  setDescription("");
                  setRole(INVITE_DEFAULT_ROLE);
                }}
              />
            ) : (
              <>
                <Button
                  size={ButtonSize.LARGE}
                  label="Cancel"
                  variant={ButtonVariant.SECONDARY}
                  onClick={() => setIsCreating(false)}
                  isDisabled={isCreatingAccount}
                />
                <Button
                  size={ButtonSize.LARGE}
                  label="Create"
                  onClick={handleCreate}
                  isLoading={isCreatingAccount}
                  isDisabled={!canManage}
                />
              </>
            )}
          </FlexWrapper>
        </>
      )}
    </DialogWrapper>
  );
};

export default ServiceAccountsModal;
