import { useState } from "react";

import {
  ArrowsClockwiseIcon,
  DotsThreeVerticalIcon,
  PlusIcon,
  TrashIcon,
} from "@phosphor-icons/react";

import Beacon, { BeaconVariant } from "@galaxy-io/dls/beacons/Beacon";
import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Box, { BoxVariant } from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Menu, { MenuItem } from "@galaxy-io/dls/menu/Menu";
import InfiniteTable from "@galaxy-io/dls/table/InfiniteTable";
import type { TableColumn } from "@galaxy-io/dls/table/types";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { FontFamily, Placement } from "@galaxy-io/dls/theme/enums";

import type { ServiceAccount } from "@/gen/auth/v1/service_accounts_pb";

import Dialog, { DialogVariant } from "@/components/Dialog";

import EmptyLayout from "@/layouts/EmptyLayout";

import SettingsPanelLayout from "@/pages/settings/components/SettingsPanelLayout";
import {
  SETTINGS_SERVICE_ACCOUNTS_TABLE_COLUMN_WIDTH_ACTIONS,
  SETTINGS_SERVICE_ACCOUNTS_TABLE_COLUMN_WIDTH_CLIENT_ID,
  SETTINGS_SERVICE_ACCOUNTS_TABLE_COLUMN_WIDTH_ROLE,
} from "@/pages/settings/constants";
import SettingsServiceAccountsPanelCreateDialog from "@/pages/settings/panels/service-accounts/SettingsServiceAccountsPanelCreateDialog";
import SettingsServiceAccountsPanelCredentials from "@/pages/settings/panels/service-accounts/SettingsServiceAccountsPanelCredentials";
import type { ServiceAccountCredentials } from "@/pages/settings/types";
import { serviceAccountRoleLabel } from "@/pages/settings/utils";

import {
  useListServiceAccountsQuery,
  useRemoveServiceAccountMutation,
  useRotateServiceAccountSecretMutation,
} from "@/api/queries/auth";

import { useConfirm } from "@/hooks/useConfirm";

import type { AppSession } from "@/auth/types";
import { getErrorMessage } from "@/utils/errors";

interface SettingsServiceAccountsPanelProps {
  session: AppSession;
}

const SettingsServiceAccountsPanel = ({ session }: SettingsServiceAccountsPanelProps) => {
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [error, setError] = useState<string>();
  const [rotatedCredentials, setRotatedCredentials] = useState<ServiceAccountCredentials>();

  const accountsQuery = useListServiceAccountsQuery({
    options: { enabled: session.isAuthenticated },
  });
  const { mutate: rotateSecret, isPending: isRotating } = useRotateServiceAccountSecretMutation();
  const { mutate: removeAccount, isPending: isRemoving } = useRemoveServiceAccountMutation();
  const canManage = accountsQuery.data?.canManage === true;

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

  const handleRemove = (account: ServiceAccount) => {
    setError(undefined);
    accountConfirm.handleOpen(account);
  };

  const columns: TableColumn<ServiceAccount>[] = [
    {
      id: "name",
      header: "Service account",
      accessor: (account) => account.name,
      canSort: false,
      cell: ({ row }) => (
        <Flex alignItems={AlignItems.CENTER} gap={8} fillWidth minWidth={0}>
          <FlexItem shrink={0}>
            <Beacon variant={BeaconVariant.SUCCESS} />
          </FlexItem>
          <FlexItem grow={1} minWidth={0}>
            <Text weight={TextWeight.MEDIUM} lineClamp={1}>
              {row.name}
            </Text>
          </FlexItem>
        </Flex>
      ),
    },
    {
      id: "clientId",
      header: "Client ID",
      accessor: (account) => account.clientId,
      width: SETTINGS_SERVICE_ACCOUNTS_TABLE_COLUMN_WIDTH_CLIENT_ID,
      canSort: false,
      cell: ({ row }) => (
        <Text variant={TextVariant.SECONDARY} family={FontFamily.MONO} lineClamp={1}>
          {row.clientId}
        </Text>
      ),
    },
    {
      id: "role",
      header: "Permissions",
      width: SETTINGS_SERVICE_ACCOUNTS_TABLE_COLUMN_WIDTH_ROLE,
      canSort: false,
      cell: ({ row }) => <Text>{serviceAccountRoleLabel(row.role)}</Text>,
    },
    {
      id: "actions",
      header: "",
      align: "right",
      width: SETTINGS_SERVICE_ACCOUNTS_TABLE_COLUMN_WIDTH_ACTIONS,
      canSort: false,
      cell: ({ row }) => (
        <Menu
          placement={Placement.RIGHT_START}
          ariaLabel={`Actions for ${row.name}`}
          isDisabled={!canManage || isRotating || isRemoving}
          trigger={
            <Button
              icon={DotsThreeVerticalIcon}
              variant={ButtonVariant.TERTIARY}
              size={ButtonSize.SMALL}
              ariaLabel={`Actions for ${row.name}`}
              tooltip={`Actions for ${row.name}`}
              isDisabled={!canManage || isRotating || isRemoving}
            />
          }
        >
          <MenuItem
            label="Rotate secret"
            icon={ArrowsClockwiseIcon}
            onSelect={() => {
              setError(undefined);
              rotateConfirm.handleOpen(row);
            }}
          />
          <MenuItem label="Delete" icon={TrashIcon} onSelect={() => handleRemove(row)} />
        </Menu>
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
      <SettingsPanelLayout
        title="Service accounts"
        actions={
          canManage
            ? [
                <Button
                  key="create-service-account"
                  label="New service account"
                  icon={PlusIcon}
                  onClick={() => setIsCreateOpen(true)}
                />,
              ]
            : undefined
        }
      >
        <Flex alignItems={AlignItems.START} grow={1} basis={0} minHeight={0} fillWidth>
          <Box variant={BoxVariant.BASE} height="100%" fillWidth>
            <InfiniteTable<ServiceAccount>
              columns={columns}
              data={accountsQuery.data?.serviceAccounts ?? []}
              getRowId={(account) => account.userId}
              isLoading={accountsQuery.isLoading}
              emptyState={
                <EmptyLayout
                  header="No service accounts yet"
                  message="Create one to give CLI, CI, and automation access to your organization."
                />
              }
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
      {isCreateOpen && (
        <SettingsServiceAccountsPanelCreateDialog open onClose={() => setIsCreateOpen(false)} />
      )}
      <Dialog
        open={rotateConfirm.isOpen}
        onClose={rotateConfirm.handleClose}
        onConfirm={rotateConfirm.handleConfirm}
        title="Rotate client secret"
        description={`Generate a new credential for ${rotateConfirm.target?.name ?? "this service account"}`}
        variant={DialogVariant.WARNING}
        bodyTitle="Current secret will be revoked"
        body="Any clients using the existing secret will immediately lose access and must be updated with the new secret."
        confirmLabel="Rotate secret"
        confirmVariant={ButtonVariant.ERROR}
        isPending={isRotating}
      />
      <Dialog
        open={!!rotatedCredentials}
        onClose={() => setRotatedCredentials(undefined)}
        title="Client secret rotated"
        description="Copy the new credentials now. The client secret is only shown once."
        footer={
          <Button
            size={ButtonSize.LARGE}
            label="Done"
            onClick={() => setRotatedCredentials(undefined)}
          />
        }
      >
        {rotatedCredentials && (
          <SettingsServiceAccountsPanelCredentials credentials={rotatedCredentials} />
        )}
      </Dialog>
      <Dialog
        open={accountConfirm.isOpen}
        onClose={accountConfirm.handleClose}
        onConfirm={accountConfirm.handleConfirm}
        title="Delete service account"
        body="This service account will be deleted and its credentials will stop working immediately."
        confirmationPhrase={accountConfirm.target?.name}
        confirmLabel="Delete service account"
        confirmVariant={ButtonVariant.ERROR}
        isPending={isRemoving}
      />
    </>
  );
};

export default SettingsServiceAccountsPanel;
