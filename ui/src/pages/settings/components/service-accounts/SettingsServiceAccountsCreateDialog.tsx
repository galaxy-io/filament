import { type FC, useState } from "react";

import { create } from "@bufbuild/protobuf";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Alert, { AlertVariant } from "@galaxy-io/dls/feedback/Alert";
import SelectInput from "@galaxy-io/dls/inputs/SelectInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Box from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import Modal from "@galaxy-io/dls/modal/Modal";
import Text, { TextVariant } from "@galaxy-io/dls/text/Text";

import { Role } from "@/gen/auth/v1/members_pb";
import {
  type CreateServiceAccountRequest,
  CreateServiceAccountRequestSchema,
} from "@/gen/auth/v1/service_accounts_pb";

import SettingsServiceAccountsCredentials from "@/pages/settings/components/service-accounts/SettingsServiceAccountsCredentials";
import {
  SETTINGS_SERVICE_ACCOUNT_DEFAULT_ROLE,
  SETTINGS_SERVICE_ACCOUNT_ROLE_OPTIONS,
} from "@/pages/settings/constants";
import type { ServiceAccountCredentials } from "@/pages/settings/types";

import { useCreateServiceAccountMutation } from "@/api/queries/auth";

import { getErrorMessage } from "@/utils/errors";
import { mapOptionIdToEnum } from "@/utils/select";
import { getNameError } from "@/utils/validation";

interface SettingsServiceAccountsCreateDialogState {
  name: CreateServiceAccountRequest["name"];
  role: CreateServiceAccountRequest["role"];
  credentials: ServiceAccountCredentials | undefined;
  error: string | undefined;
  shouldShowErrors: boolean;
}

const DEFAULT_STATE: SettingsServiceAccountsCreateDialogState = {
  name: "",
  role: SETTINGS_SERVICE_ACCOUNT_DEFAULT_ROLE,
  credentials: undefined,
  error: undefined,
  shouldShowErrors: false,
};

interface SettingsServiceAccountsCreateDialogProps {
  isOpen: boolean;
  onClose: () => void;
}

const SettingsServiceAccountsCreateDialog: FC<SettingsServiceAccountsCreateDialogProps> = ({
  isOpen,
  onClose,
}) => {
  const [state, setState] = useState<SettingsServiceAccountsCreateDialogState>(DEFAULT_STATE);
  const { mutate: createAccount, isPending: isCreating } = useCreateServiceAccountMutation();

  const nameError = getNameError(state.name, state.shouldShowErrors);

  const handleCreate = () => {
    const name = state.name.trim();
    if (name === "") {
      setState((prev) => ({ ...prev, error: undefined, shouldShowErrors: true }));
      return;
    }
    setState((prev) => ({ ...prev, error: undefined }));
    createAccount(
      create(CreateServiceAccountRequestSchema, {
        name,
        description: `Filament service account: ${name}`,
        role: state.role,
      }),
      {
        onSuccess: ({ serviceAccount, clientSecret }) => {
          if (serviceAccount) {
            setState((prev) => ({
              ...prev,
              credentials: { clientId: serviceAccount.clientId, clientSecret },
            }));
          }
        },
        onError: (err) => {
          setState((prev) => ({
            ...prev,
            error: getErrorMessage(err, "Could not create service account"),
          }));
        },
      },
    );
  };

  const handleOpenChange = (isOpen: boolean) => {
    if (!isOpen && !isCreating) onClose();
  };

  if (state.credentials) {
    return (
      <Modal
        header="Service account created"
        isOpen={isOpen}
        onOpenChange={handleOpenChange}
        footer={<Button label="Done" onClick={onClose} />}
      >
        <Flex alignItems={AlignItems.STRETCH} direction={FlexDirection.COLUMN} gap={16}>
          <Text isProse variant={TextVariant.SECONDARY}>
            Copy these credentials now. The client secret is only shown once.
          </Text>
          <SettingsServiceAccountsCredentials credentials={state.credentials} />
        </Flex>
      </Modal>
    );
  }

  return (
    <Modal
      header="Create service account"
      isOpen={isOpen}
      isDismissable={!isCreating}
      onOpenChange={handleOpenChange}
      footer={
        <>
          <Button
            label="Cancel"
            variant={ButtonVariant.SECONDARY}
            onClick={onClose}
            isDisabled={isCreating}
          />
          <Button label="Create service account" onClick={handleCreate} isLoading={isCreating} />
        </>
      }
    >
      <Flex alignItems={AlignItems.STRETCH} direction={FlexDirection.COLUMN} gap={16}>
        <Text isProse variant={TextVariant.SECONDARY}>
          Create a machine identity for CLI, CI, and automation
        </Text>
        <TextInput
          label="Name"
          value={state.name}
          onChange={(name) => setState((prev) => ({ ...prev, name }))}
          error={nameError}
          fillWidth
          isRequired
          autoFocus
        />
        <SelectInput
          label="Role"
          options={SETTINGS_SERVICE_ACCOUNT_ROLE_OPTIONS}
          value={String(state.role)}
          onChange={(id) =>
            setState((prev) => ({ ...prev, role: mapOptionIdToEnum(Role, id ?? "") }))
          }
          fillWidth
          isRequired
        />
        {state.error && (
          <Box fillWidth>
            <Alert variant={AlertVariant.ERROR}>{state.error}</Alert>
          </Box>
        )}
      </Flex>
    </Modal>
  );
};

export default SettingsServiceAccountsCreateDialog;
