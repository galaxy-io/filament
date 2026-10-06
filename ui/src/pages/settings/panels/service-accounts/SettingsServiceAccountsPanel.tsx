import { useState } from "react";

import { styled } from "@linaria/react";
import {
  ArrowsClockwiseIcon,
  DotsThreeVerticalIcon,
  PlusIcon,
  TrashIcon,
} from "@phosphor-icons/react";

import Beacon, { BeaconVariant } from "@galaxy-io/dls/beacons/Beacon";
import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Dropdown from "@galaxy-io/dls/dropdown/Dropdown";
import Skeleton from "@galaxy-io/dls/feedback/Skeleton";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Box, { BoxVariant } from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import InfiniteTable from "@galaxy-io/dls/table/InfiniteTable";
import type { TableColumn } from "@galaxy-io/dls/table/types";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { FontFamily, Placement } from "@galaxy-io/dls/theme/enums";
import { t } from "@galaxy-io/dls/theme/tokens/t";
import Tooltip from "@galaxy-io/dls/tooltip/Tooltip";

import type { ServiceAccount } from "@/gen/auth/v1/service_accounts_pb";

import Dialog, { DialogVariant } from "@/components/Dialog";

import EmptyLayout from "@/layouts/EmptyLayout";

import SettingsPanelLayout from "@/pages/settings/components/SettingsPanelLayout";
import {
  SETTINGS_SERVICE_ACCOUNTS_ACTIONS_DROPDOWN_WIDTH,
  SETTINGS_SERVICE_ACCOUNTS_TABLE_COLUMN_WIDTH_ACTIONS,
  SETTINGS_SERVICE_ACCOUNTS_TABLE_COLUMN_WIDTH_CLIENT_ID,
  SETTINGS_SERVICE_ACCOUNTS_TABLE_COLUMN_WIDTH_ROLE,
  SETTINGS_SERVICE_ACCOUNTS_TABLE_LOADING_ROW_COUNT,
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

const ActionsMenuItem = styled.button`
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  min-height: 32px;
  padding: 6px 8px;
  border: 0;
  border-radius: 4px;
  color: ${t.color.text.primary};
  background: transparent;
  text-align: left;
  cursor: pointer;

  &:hover,
  &:focus-visible {
    background-color: ${t.color.background.tertiary};
  }

  &:focus-visible {
    outline: 1px solid ${t.color.border.secondary};
  }
`;

interface SettingsServiceAccountsPanelProps {
  session: AppSession;
}

const SettingsServiceAccountsPanel = ({ session }: SettingsServiceAccountsPanelProps) => {
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [error, setError] = useState<string>();
  const [openActionsFor, setOpenActionsFor] = useState<string>();
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
      // @dls-migrate infinitetable.column.row-original: `row` is now the data object: `row.original` → `row`.
      cell: ({ row }) => (
        <Flex
          alignItems={
            AlignItems.CENTER
          } /* @dls-migrate layout.off-scale: Pick a value on the space scale (or a CSS-order tuple of them). */
          gap={10}
          fillWidth
          minWidth={0}
        >
          <FlexItem shrink={0}>
            <Beacon variant={BeaconVariant.SUCCESS} />
          </FlexItem>
          <FlexItem grow={1} minWidth={0}>
            <Text weight={TextWeight.MEDIUM} lineClamp={1}>
              {row.original.name}
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
      // @dls-migrate infinitetable.column.row-original: `row` is now the data object: `row.original` → `row`.
      cell: ({ row }) => (
        <Text variant={TextVariant.SECONDARY} family={FontFamily.MONO} lineClamp={1}>
          {row.original.clientId}
        </Text>
      ),
    },
    {
      id: "role",
      header: "Permissions",
      width: SETTINGS_SERVICE_ACCOUNTS_TABLE_COLUMN_WIDTH_ROLE,
      canSort: false,
      // @dls-migrate infinitetable.column.row-original: `row` is now the data object: `row.original` → `row`.
      cell: ({ row }) => <Text>{serviceAccountRoleLabel(row.original.role)}</Text>,
    },
    {
      id: "actions",
      header: "",
      align: "right",
      width: SETTINGS_SERVICE_ACCOUNTS_TABLE_COLUMN_WIDTH_ACTIONS,
      canSort: false,
      // @dls-migrate infinitetable.column.row-original: `row` is now the data object: `row.original` → `row`.
      cell: ({ row }) => (
        <Dropdown
          placement={Placement.RIGHT_START}
          /* @dls-migrate dropdown.minWidth: Size the `body` content, or use `shouldMatchTriggerWidth` / the trigger's own `fillWidth`. */ minWidth={
            SETTINGS_SERVICE_ACCOUNTS_ACTIONS_DROPDOWN_WIDTH
          }
          /* @dls-migrate dropdown.noPadding: A panel that hosts its own layout is a `Popover`. */ noPadding
          isOpen={openActionsFor === row.original.userId}
          /* @dls-migrate dropdown.onClose: A controlled 2.0 Dropdown also asks to open from its trigger: switch to `onOpenChange` and remove the trigger's own toggle. */ onClose={() =>
            setOpenActionsFor(undefined)
          }
          body={
            <Flex
              direction={FlexDirection.COLUMN}
              alignItems={AlignItems.STRETCH}
              padding={4}
              fillWidth
            >
              <ActionsMenuItem
                type="button"
                onClick={() => {
                  setOpenActionsFor(undefined);
                  setError(undefined);
                  rotateConfirm.handleOpen(row.original);
                }}
              >
                <Icon component={ArrowsClockwiseIcon} size={16} variant={IconVariant.SECONDARY} />
                <Text>Rotate secret</Text>
              </ActionsMenuItem>
              <ActionsMenuItem
                type="button"
                onClick={() => {
                  setOpenActionsFor(undefined);
                  handleRemove(row.original);
                }}
              >
                <Icon component={TrashIcon} size={16} variant={IconVariant.SECONDARY} />
                <Text>Delete</Text>
              </ActionsMenuItem>
            </Flex>
          }
        >
          <Tooltip body={`Actions for ${row.original.name}`} placement={Placement.BOTTOM}>
            <Button
              icon={DotsThreeVerticalIcon}
              variant={ButtonVariant.TERTIARY}
              size={ButtonSize.SMALL}
              ariaLabel={`Actions for ${row.original.name}`}
              onClick={() =>
                setOpenActionsFor((current) =>
                  current === row.original.userId ? undefined : row.original.userId,
                )
              }
              isDisabled={!canManage || isRotating || isRemoving}
            />
          </Tooltip>
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
          <Box variant={BoxVariant.BASE} height="100%">
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
