import { type FC, useState } from "react";

import {
  ArrowsClockwiseIcon,
  CopyIcon,
  LockSimpleIcon,
  PlusIcon,
  TrashIcon,
} from "@phosphor-icons/react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Chip, { ChipSize } from "@galaxy-io/dls/chips/Chip";
import Alert, { AlertVariant } from "@galaxy-io/dls/feedback/Alert";
import { useClipboard } from "@galaxy-io/dls/hooks/useClipboard";
import EmptyLayout from "@galaxy-io/dls/layout/EmptyLayout";
import ErrorLayout from "@galaxy-io/dls/layout/ErrorLayout";
import Flex, { FlexDirection } from "@galaxy-io/dls/layout/Flex";
import PageLayout from "@galaxy-io/dls/layout/PageLayout";
import { MenuItem, MenuItemVariant, MenuSeparator } from "@galaxy-io/dls/menu/Menu";
import ConfirmDialog from "@galaxy-io/dls/modal/ConfirmDialog";
import Modal from "@galaxy-io/dls/modal/Modal";
import InfiniteTable from "@galaxy-io/dls/table/InfiniteTable";
import type { TableColumn } from "@galaxy-io/dls/table/types";
import Text, { TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import type { ServiceAccount } from "@/gen/auth/v1/service_accounts_pb";

import SettingsServiceAccountsCreateDialog from "@/pages/settings/components/service-accounts/SettingsServiceAccountsCreateDialog";
import SettingsServiceAccountsCredentials from "@/pages/settings/components/service-accounts/SettingsServiceAccountsCredentials";
import {
  SETTINGS_ROLE_TO_CHIP_PROPS_MAP,
  SETTINGS_SERVICE_ACCOUNT_ROLE_TO_LABEL_MAP,
  SETTINGS_SERVICE_ACCOUNTS_TABLE_COLUMN_MIN_WIDTH_NAME,
  SETTINGS_SERVICE_ACCOUNTS_TABLE_COLUMN_WIDTH_CLIENT_ID,
  SETTINGS_SERVICE_ACCOUNTS_TABLE_COLUMN_WIDTH_ROLE,
} from "@/pages/settings/constants";
import type { ServiceAccountCredentials } from "@/pages/settings/types";

import {
  useCanManageTeam,
  useListServiceAccountsQuery,
  useRemoveServiceAccountMutation,
  useRotateServiceAccountSecretMutation,
} from "@/api/queries/auth";

import { useConfirm } from "@/hooks/useConfirm";

import { getErrorMessage } from "@/utils/errors";

const ServiceAccountsPage: FC = () => {
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [rotatedCredentials, setRotatedCredentials] = useState<ServiceAccountCredentials>();
  const { toast } = useToast();
  const { copy } = useClipboard();

  const accountsQuery = useListServiceAccountsQuery();
  const { mutate: rotateSecret, isPending: isRotating } = useRotateServiceAccountSecretMutation();
  const { mutate: removeAccount, isPending: isRemoving } = useRemoveServiceAccountMutation();
  const canManage = useCanManageTeam();

  const accountConfirm = useConfirm<ServiceAccount>({
    entityLabel: "Service account",
    entityName: (account) => account.name,
    onConfirm: (account, { onSuccess, onError }) =>
      removeAccount({ userId: account.userId }, { onSuccess, onError }),
  });

  const rotateConfirm = useConfirm<ServiceAccount>({
    entityLabel: "Client secret",
    entityName: (account) => account.name,
    messages: {
      successHeader: "Client secret rotated",
      successSubheader: (account) => `A new client secret was generated for ${account.name}.`,
      errorHeader: "Rotate failed",
      errorFallback: "Could not rotate client secret",
    },
    onConfirm: (account, { onSuccess, onError }) =>
      rotateSecret(
        { userId: account.userId },
        {
          onSuccess: ({ clientId, clientSecret }) => {
            setRotatedCredentials({ clientId, clientSecret });
            onSuccess();
          },
          onError,
        },
      ),
  });

  const handleCopyClientId = async (account: ServiceAccount) => {
    const hasCopied = await copy(account.clientId);
    toast(
      hasCopied
        ? {
            header: "Client ID copied",
            description: `${account.name}'s client ID is on your clipboard.`,
            variant: ToastVariant.SUCCESS,
          }
        : {
            header: "Copy failed",
            description: "Your browser blocked clipboard access.",
            variant: ToastVariant.ERROR,
          },
    );
  };

  const columns: TableColumn<ServiceAccount>[] = [
    {
      id: "name",
      header: "Name",
      accessor: (account) => account.name,
      isRowHeader: true,
      canSort: true,
      minWidth: SETTINGS_SERVICE_ACCOUNTS_TABLE_COLUMN_MIN_WIDTH_NAME,
      cell: ({ row }) => (
        <Text lineClamp={1} shouldTooltipOnOverflow>
          {row.name}
        </Text>
      ),
    },
    {
      id: "clientId",
      header: "Client ID",
      accessor: (account) => account.clientId,
      minWidth: SETTINGS_SERVICE_ACCOUNTS_TABLE_COLUMN_WIDTH_CLIENT_ID,
      cell: ({ row }) => (
        <Text
          variant={TextVariant.SECONDARY}
          family={FontFamily.MONO}
          lineClamp={1}
          shouldTooltipOnOverflow
        >
          {row.clientId}
        </Text>
      ),
    },
    {
      id: "role",
      header: "Permissions",
      accessor: (account) => SETTINGS_SERVICE_ACCOUNT_ROLE_TO_LABEL_MAP[account.role],
      width: SETTINGS_SERVICE_ACCOUNTS_TABLE_COLUMN_WIDTH_ROLE,
      canSort: true,
      cell: ({ row }) => (
        <Chip
          label={SETTINGS_SERVICE_ACCOUNT_ROLE_TO_LABEL_MAP[row.role]}
          size={ChipSize.SMALL}
          {...SETTINGS_ROLE_TO_CHIP_PROPS_MAP[row.role]}
        />
      ),
    },
  ];

  const displayError = accountsQuery.error
    ? getErrorMessage(accountsQuery.error, "Could not load service accounts")
    : undefined;
  const isActionPending = isRotating || isRemoving;

  if (canManage === false) {
    return (
      <PageLayout header="Service accounts">
        <ErrorLayout
          icon={LockSimpleIcon}
          header="Admins only"
          description="Ask an admin of this organization to manage service accounts."
        />
      </PageLayout>
    );
  }

  return (
    <>
      <PageLayout
        header="Service accounts"
        actions={
          canManage && (
            <Button
              label="New service account"
              icon={PlusIcon}
              variant={ButtonVariant.PRIMARY}
              onClick={() => setIsCreateOpen(true)}
            />
          )
        }
      >
        <Flex direction={FlexDirection.COLUMN} grow={1} basis={0} minHeight={0} fillWidth>
          <InfiniteTable<ServiceAccount>
            columns={columns}
            data={accountsQuery.data?.serviceAccounts ?? []}
            getRowId={(account) => account.userId}
            isLoading={accountsQuery.isLoading}
            error={displayError}
            rowActions={(row) =>
              canManage ? (
                <>
                  <MenuItem
                    label="Copy client ID"
                    icon={CopyIcon}
                    onSelect={() => void handleCopyClientId(row)}
                  />
                  <MenuItem
                    label="Rotate secret"
                    icon={ArrowsClockwiseIcon}
                    onSelect={() => rotateConfirm.handleOpen(row)}
                    isDisabled={isActionPending}
                  />
                  <MenuSeparator />
                  <MenuItem
                    label="Delete"
                    icon={TrashIcon}
                    variant={MenuItemVariant.ERROR}
                    onSelect={() => accountConfirm.handleOpen(row)}
                    isDisabled={isActionPending}
                  />
                </>
              ) : null
            }
            ariaLabel="Service accounts"
            emptyState={
              <EmptyLayout
                header="No service accounts yet"
                description="Create one to give CLI, CI, and automation access to your organization."
              />
            }
          />
        </Flex>
      </PageLayout>
      {isCreateOpen && (
        <SettingsServiceAccountsCreateDialog isOpen onClose={() => setIsCreateOpen(false)} />
      )}
      <ConfirmDialog
        isOpen={rotateConfirm.isOpen}
        onOpenChange={(isOpen) => {
          if (!isOpen) rotateConfirm.handleClose();
        }}
        onConfirm={rotateConfirm.handleConfirm}
        header="Rotate client secret?"
        description={`Generate a new credential for ${rotateConfirm.target?.name ?? "this service account"}`}
        label="Rotate secret"
        isDestructive
      >
        <Alert variant={AlertVariant.WARNING} header="Current secret will be revoked">
          Any clients using the existing secret will immediately lose access and must be updated
          with the new secret.
        </Alert>
      </ConfirmDialog>
      <Modal
        header="Client secret rotated"
        isOpen={!!rotatedCredentials}
        onOpenChange={(isOpen) => {
          if (!isOpen) setRotatedCredentials(undefined);
        }}
        footer={<Button label="Done" onClick={() => setRotatedCredentials(undefined)} />}
      >
        <Flex direction={FlexDirection.COLUMN} gap={16}>
          <Text isProse variant={TextVariant.SECONDARY}>
            Copy the new credentials now. The client secret is only shown once.
          </Text>
          {rotatedCredentials && (
            <SettingsServiceAccountsCredentials credentials={rotatedCredentials} />
          )}
        </Flex>
      </Modal>
      <ConfirmDialog
        isOpen={accountConfirm.isOpen}
        onOpenChange={(isOpen) => {
          if (!isOpen) accountConfirm.handleClose();
        }}
        onConfirm={accountConfirm.handleConfirm}
        header="Delete service account?"
        description="This service account will be deleted and its credentials will stop working immediately."
        confirmValue={accountConfirm.target?.name}
        label="Delete service account"
        isDestructive
      />
    </>
  );
};

export default ServiceAccountsPage;
