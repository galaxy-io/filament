import { useState } from "react";

import { styled } from "@linaria/react";
import {
  ArrowsClockwiseIcon,
  CheckIcon,
  CopyIcon,
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
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import SelectInput, { type SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Modal from "@galaxy-io/dls/modal/Modal";
import InfiniteTable, { ColumnAlign, type ColumnDef } from "@galaxy-io/dls/table/InfiniteTable";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import type { ServiceAccount } from "@/gen/auth/v1/service_accounts_pb";

import ConfirmDialog from "@/components/ConfirmDialog";
import DeleteConfirmDialog from "@/components/DeleteConfirmDialog";
import {
  optionRole,
  SERVICE_ACCOUNT_DEFAULT_ROLE,
  SERVICE_ACCOUNT_ROLE_OPTIONS,
  serviceAccountRoleLabel,
} from "@/components/settings/constants";

import BaseHeader, { BaseHeaderSize } from "@/layouts/components/BaseHeader";

import {
  useCreateServiceAccountMutation,
  useRemoveServiceAccountMutation,
  useRotateServiceAccountSecretMutation,
} from "@/api/mutations/auth";
import { useListServiceAccountsQuery } from "@/api/queries/auth";

import { useDeleteConfirm } from "@/hooks/useDeleteConfirm";

import type { AppSession } from "@/auth/session";
import { getErrorMessage } from "@/utils/errors";

const DialogWrapper = withTheme(
  styled.div<PropsWithTheme<{ $compact?: boolean; $panel?: boolean }>>`
    display: flex;
    flex-direction: column;
    width: ${({ $compact, $panel }) => ($panel ? "100%" : `${$compact ? 560 : 720}px`)};
    max-width: ${({ $panel }) => ($panel ? "none" : "calc(100vw - 32px)")};
    height: auto;
    background-color: ${({ theme, $panel }) =>
      $panel ? "transparent" : theme.color.background.primary};
    border: ${({ theme, $panel }) =>
      $panel ? "none" : `0.5px solid ${theme.color.border.primary}`};
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
  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 5px;
`);

const CredentialsContent = styled.div`
  display: flex;
  flex-direction: column;
  gap: 12px;
  width: 100%;
  max-width: 520px;
`;

const FormContent = styled.div`
  width: 100%;
  max-width: 520px;
`;

const CredentialCard = withTheme(styled.div<PropsWithTheme>`
  display: flex;
  flex-direction: column;
  width: 100%;
  background-color: ${({ theme }) => theme.color.background.primary};
  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 6px;
  overflow: hidden;
`);

const CredentialRow = withTheme(styled.div<PropsWithTheme>`
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px;

  &:not(:last-child) {
    border-bottom: 0.5px solid ${({ theme }) => theme.color.border.primary};
  }
`);

const CredentialValue = withTheme(styled.div<PropsWithTheme>`
  width: 100%;
  min-width: 0;
  padding: 8px;
  background-color: ${({ theme }) => theme.color.background.secondary};
  border-radius: 4px;

  p {
    overflow-wrap: anywhere;
  }
`);

const CopyCredentialButton = withTheme(styled.button<PropsWithTheme>`
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 24px;
  height: 24px;
  padding: 0;
  color: ${({ theme }) => theme.color.text.secondary};
  background-color: transparent;
  border: 0;
  border-radius: 4px;
  cursor: pointer;

  &:hover {
    color: ${({ theme }) => theme.color.text.primary};
    background-color: ${({ theme }) => theme.color.background.tertiary};
  }

  &:focus {
    background-color: transparent;
  }

  &:focus-visible {
    outline: 1px solid ${({ theme }) => theme.color.border.secondary};
    outline-offset: 1px;
  }
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

type CredentialsResult = "created" | "rotated";

const CopyCredentialRow = ({
  label,
  description,
  value,
}: {
  label: string;
  description?: string;
  value: string;
}) => {
  const [isCopied, setIsCopied] = useState(false);

  const handleCopy = () => {
    void navigator.clipboard.writeText(value).then(() => {
      setIsCopied(true);
      window.setTimeout(() => setIsCopied(false), 2000);
    });
  };

  return (
    <CredentialRow>
      <FlexWrapper
        alignItems={AlignItems.CENTER}
        justifyContent={JustifyContent.SPACE_BETWEEN}
        gap={12}
        fillWidth
      >
        <FlexWrapper direction={FlexDirection.COLUMN} gap={2} minWidth={0}>
          <Text size={TextSize.BODY_SM} weight={TextWeight.MEDIUM}>
            {label}
          </Text>
          {description && (
            <Text size={TextSize.CAPTION} variant={TextVariant.SECONDARY}>
              {description}
            </Text>
          )}
        </FlexWrapper>
        <CopyCredentialButton
          type="button"
          aria-label={isCopied ? `${label} copied` : `Copy ${label}`}
          title={isCopied ? "Copied" : `Copy ${label}`}
          onClick={handleCopy}
        >
          <Icon
            component={isCopied ? CheckIcon : CopyIcon}
            size={isCopied ? 16 : 14}
            variant={isCopied ? IconVariant.SUCCESS : IconVariant.SECONDARY}
          />
        </CopyCredentialButton>
      </FlexWrapper>
      <CredentialValue>
        <Text size={TextSize.BODY_SM} isMonospace isSelectable>
          {value}
        </Text>
      </CredentialValue>
    </CredentialRow>
  );
};

interface ServiceAccountsModalProps {
  session: AppSession;
  onClose?: () => void;
  onCreate?: () => void;
  isPanel?: boolean;
  defaultToCreate?: boolean;
}

const ServiceAccountsModal = ({
  session,
  onClose,
  onCreate,
  isPanel = false,
  defaultToCreate = false,
}: ServiceAccountsModalProps) => {
  const [isCreating, setIsCreating] = useState(defaultToCreate);
  const [name, setName] = useState("");
  const [role, setRole] = useState<SelectInputOption>(SERVICE_ACCOUNT_DEFAULT_ROLE);
  const [credentials, setCredentials] = useState<Credentials>();
  const [credentialsResult, setCredentialsResult] = useState<CredentialsResult>("created");
  const [error, setError] = useState<string>();
  const [openActionsFor, setOpenActionsFor] = useState<string>();
  const [accountToRotate, setAccountToRotate] = useState<ServiceAccount>();
  const [accountToRemove, setAccountToRemove] = useState<ServiceAccount>();

  const accountsQuery = useListServiceAccountsQuery({
    options: { enabled: !!session.accessToken },
  });
  const { mutate: createAccount, isPending: isCreatingAccount } = useCreateServiceAccountMutation();
  const { mutate: rotateSecret, isPending: isRotating } = useRotateServiceAccountSecretMutation();
  const { mutate: removeAccount, isPending: isRemoving } = useRemoveServiceAccountMutation();
  const canManage = accountsQuery.data?.canManage === true;

  const accountDeleteConfirm = useDeleteConfirm({
    entityLabel: "Service account",
    entityName: accountToRemove?.name ?? "",
    onDelete: ({ onSuccess, onError }) => {
      if (!accountToRemove) return;
      removeAccount({ userId: accountToRemove.userId }, { onSuccess, onError });
    },
    onDeleted: () => setAccountToRemove(undefined),
  });

  const showCredentials = (next: Credentials, result: CredentialsResult = "created") => {
    setCredentials(next);
    setCredentialsResult(result);
    setIsCreating(false);
    setError(undefined);
  };

  const handleCredentialsDone = () => {
    setCredentials(undefined);
    setName("");
    setRole(SERVICE_ACCOUNT_DEFAULT_ROLE);
    if (defaultToCreate) onClose?.();
  };

  const handleCreate = () => {
    const accountName = name.trim();
    const accountRole = optionRole(role);
    if (accountName === "" || accountRole === undefined) {
      setError("Name and role are required");
      return;
    }
    setError(undefined);
    createAccount(
      {
        name: accountName,
        description: `Filament service account: ${accountName}`,
        role: accountRole,
      },
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
    setError(undefined);
    setAccountToRotate(account);
  };

  const handleRotateConfirm = () => {
    if (!accountToRotate) return;
    rotateSecret(
      { userId: accountToRotate.userId },
      {
        onSuccess: ({ clientId, clientSecret }) => {
          setAccountToRotate(undefined);
          showCredentials({ clientId, clientSecret }, "rotated");
        },
        onError: (err) => setError(getErrorMessage(err, "Could not rotate client secret")),
      },
    );
  };

  const handleRemove = (account: ServiceAccount) => {
    setError(undefined);
    setAccountToRemove(account);
    accountDeleteConfirm.handleOpen();
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
          <Text weight={TextWeight.MEDIUM} isEllipsis>
            {row.original.name}
          </Text>
        </AccountIdentity>
      ),
    },
    {
      id: "clientId",
      header: "Client ID",
      accessorFn: (account) => account.clientId,
      size: 220,
      cellLoading: () => <TextShimmer width={140} height={16} />,
      cell: ({ row }) => (
        <Text variant={TextVariant.SECONDARY} isMonospace isEllipsis>
          {row.original.clientId}
        </Text>
      ),
    },
    {
      id: "role",
      header: "Permissions",
      size: 140,
      cell: ({ row }) => <Text>{serviceAccountRoleLabel(row.original.role) ?? "None"}</Text>,
    },
    {
      id: "actions",
      header: "",
      align: ColumnAlign.RIGHT,
      size: 44,
      cell: ({ row }) => (
        <Dropdown
          position={DropdownPosition.RIGHT_START}
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
    <>
      <DialogWrapper $compact={(!isPanel && !!credentials) || isCreating} $panel={isPanel}>
        <FlexWrapper padding={isPanel ? "0" : "16px"}>
          <BaseHeader
            title={
              credentials
                ? credentialsResult === "rotated"
                  ? "Client secret rotated"
                  : "Service account created"
                : isCreating
                  ? "Create service account"
                  : "Service accounts"
            }
            description={
              credentials
                ? "Copy these credentials now. The client secret is only shown once."
                : isCreating
                  ? "Create a machine identity for CLI, CI, and automation"
                  : "Manage machine access to your organization"
            }
            size={isPanel ? BaseHeaderSize.LARGE : undefined}
            actions={
              isPanel && !isCreating && canManage
                ? [
                    <Button
                      key="create-service-account"
                      label="Create service account"
                      icon={PlusIcon}
                      onClick={() => (onCreate ? onCreate() : setIsCreating(true))}
                    />,
                  ]
                : undefined
            }
            onClose={isPanel ? undefined : onClose}
          />
        </FlexWrapper>
        {!isPanel && <HorizontalDivider />}
        <BodyWrapper $panel={isPanel}>
          {credentials && !isPanel ? (
            <CredentialsContent>
              <CredentialCard>
                <CopyCredentialRow label="Client ID" value={credentials.clientId} />
                <CopyCredentialRow label="Client secret" value={credentials.clientSecret} />
                <CopyCredentialRow
                  label="CLI command"
                  description="Run this command, then enter the client secret when prompted."
                  value={`filament auth login --server ${window.location.origin} --client-id ${credentials.clientId}`}
                />
              </CredentialCard>
            </CredentialsContent>
          ) : isCreating ? (
            <FormContent>
              <FlexWrapper direction={FlexDirection.COLUMN} gap={12} fillWidth>
                <TextInput
                  label="Name"
                  value={name}
                  onChange={setName}
                  fillWidth
                  isRequired
                  autoFocus
                />
                <SelectInput
                  label="Role"
                  options={SERVICE_ACCOUNT_ROLE_OPTIONS}
                  value={role}
                  onChange={setRole}
                  fillWidth
                  isRequired
                />
              </FlexWrapper>
            </FormContent>
          ) : (
            <FlexWrapper direction={FlexDirection.COLUMN} gap={12} fillWidth>
              {!isPanel && (
                <FlexWrapper
                  alignItems={AlignItems.CENTER}
                  justifyContent={JustifyContent.SPACE_BETWEEN}
                  fillWidth
                >
                  <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
                    Create and revoke machine identities for your organization
                  </Text>
                  {canManage && (
                    <Button
                      label="Create service account"
                      icon={PlusIcon}
                      onClick={() => (onCreate ? onCreate() : setIsCreating(true))}
                    />
                  )}
                </FlexWrapper>
              )}
              <TableWrapper>
                <InfiniteTable<ServiceAccount>
                  columns={columns}
                  data={accountsQuery.data?.serviceAccounts ?? []}
                  getRowId={(account) => account.userId}
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
        {((credentials && !isPanel) || isCreating) && (
          <>
            {!isPanel && (
              <FlexItem grow={0} shrink={0}>
                <HorizontalDivider />
              </FlexItem>
            )}
            <FlexWrapper
              justifyContent={JustifyContent.END}
              padding={isPanel ? "16px 0 0" : "16px"}
              gap={8}
              fillWidth
            >
              {credentials ? (
                <Button size={ButtonSize.LARGE} label="Done" onClick={handleCredentialsDone} />
              ) : (
                <>
                  <Button
                    size={ButtonSize.LARGE}
                    label="Cancel"
                    variant={ButtonVariant.SECONDARY}
                    onClick={() => (defaultToCreate ? onClose?.() : setIsCreating(false))}
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
      <Modal
        open={!!accountToRotate}
        onClose={() => {
          if (!isRotating) setAccountToRotate(undefined);
        }}
      >
        <ConfirmDialog
          title="Rotate client secret"
          description={`Generate a new credential for ${accountToRotate?.name ?? "this service account"}`}
          bodyTitle="Current secret will be revoked"
          body="Any clients using the existing secret will immediately lose access and must be updated with the new secret."
          confirmLabel="Rotate secret"
          confirmVariant={ButtonVariant.ERROR}
          onClose={() => setAccountToRotate(undefined)}
          onConfirm={handleRotateConfirm}
          isPending={isRotating}
        />
      </Modal>
      <Modal open={isPanel && !!credentials} onClose={handleCredentialsDone}>
        <DialogWrapper $compact>
          <FlexWrapper padding="16px">
            <BaseHeader
              title="Client secret rotated"
              description="Copy the new credentials now. The client secret is only shown once."
              onClose={handleCredentialsDone}
            />
          </FlexWrapper>
          <HorizontalDivider />
          <BodyWrapper>
            {credentials && (
              <CredentialsContent>
                <CredentialCard>
                  <CopyCredentialRow label="Client ID" value={credentials.clientId} />
                  <CopyCredentialRow label="Client secret" value={credentials.clientSecret} />
                  <CopyCredentialRow
                    label="CLI command"
                    description="Run this command, then enter the client secret when prompted."
                    value={`filament auth login --server ${window.location.origin} --client-id ${credentials.clientId}`}
                  />
                </CredentialCard>
              </CredentialsContent>
            )}
          </BodyWrapper>
          <HorizontalDivider />
          <FlexWrapper justifyContent={JustifyContent.END} padding="16px" fillWidth>
            <Button label="Done" onClick={handleCredentialsDone} />
          </FlexWrapper>
        </DialogWrapper>
      </Modal>
      <Modal
        open={accountDeleteConfirm.isOpen}
        onClose={() => {
          accountDeleteConfirm.handleClose();
          setAccountToRemove(undefined);
        }}
      >
        <DeleteConfirmDialog
          open={accountDeleteConfirm.isOpen}
          onClose={() => {
            accountDeleteConfirm.handleClose();
            setAccountToRemove(undefined);
          }}
          onConfirm={accountDeleteConfirm.handleConfirm}
          title="Delete service account"
          body="This service account will be deleted and its credentials will stop working immediately."
          confirmationPhrase={accountToRemove?.name ?? ""}
          confirmLabel="Delete service account"
          isPending={isRemoving}
        />
      </Modal>
    </>
  );
};

export default ServiceAccountsModal;
