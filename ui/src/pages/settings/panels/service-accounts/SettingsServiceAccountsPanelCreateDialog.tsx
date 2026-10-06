import { useState } from "react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Alert, { AlertVariant } from "@galaxy-io/dls/feedback/Alert";
import SelectInput from "@galaxy-io/dls/inputs/SelectInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Box from "@galaxy-io/dls/layout/Box";

import type { CreateServiceAccountRequest } from "@/gen/auth/v1/service_accounts_pb";

import Dialog from "@/components/Dialog";

import {
  SERVICE_ACCOUNT_DEFAULT_ROLE,
  SERVICE_ACCOUNT_ROLE_OPTIONS,
} from "@/pages/settings/constants";
import SettingsServiceAccountsPanelCredentials from "@/pages/settings/panels/service-accounts/SettingsServiceAccountsPanelCredentials";
import type { ServiceAccountCredentials } from "@/pages/settings/types";
import { optionIdToRole, roleToOptionId } from "@/pages/settings/utils";

import { useCreateServiceAccountMutation } from "@/api/queries/auth";

import { getErrorMessage } from "@/utils/errors";

interface SettingsServiceAccountsPanelCreateDialogState {
  name: CreateServiceAccountRequest["name"];
  role: CreateServiceAccountRequest["role"];
  credentials: ServiceAccountCredentials | undefined;
  error: string | undefined;
}

const DEFAULT_STATE: SettingsServiceAccountsPanelCreateDialogState = {
  name: "",
  role: SERVICE_ACCOUNT_DEFAULT_ROLE,
  credentials: undefined,
  error: undefined,
};

interface SettingsServiceAccountsPanelCreateDialogProps {
  open: boolean;
  onClose: () => void;
}

const SettingsServiceAccountsPanelCreateDialog = ({
  open,
  onClose,
}: SettingsServiceAccountsPanelCreateDialogProps) => {
  const [state, setState] = useState<SettingsServiceAccountsPanelCreateDialogState>(DEFAULT_STATE);
  const { mutate: createAccount, isPending: isCreating } = useCreateServiceAccountMutation();

  const handleCreate = () => {
    const name = state.name.trim();
    if (name === "") {
      setState((prev) => ({ ...prev, error: "Name is required" }));
      return;
    }
    setState((prev) => ({ ...prev, error: undefined }));
    createAccount(
      { name, description: `Filament service account: ${name}`, role: state.role },
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

  if (state.credentials) {
    return (
      <Dialog
        open={open}
        title="Service account created"
        description="Copy these credentials now. The client secret is only shown once."
        onClose={onClose}
        footer={<Button label="Done" onClick={onClose} />}
      >
        <SettingsServiceAccountsPanelCredentials credentials={state.credentials} />
      </Dialog>
    );
  }

  return (
    <Dialog
      open={open}
      title="Create service account"
      description="Create a machine identity for CLI, CI, and automation"
      onClose={onClose}
      footer={
        <>
          <Button
            label="Cancel"
            variant={ButtonVariant.SECONDARY}
            onClick={onClose}
            isDisabled={isCreating}
          />
          <Button label="Create" onClick={handleCreate} isLoading={isCreating} />
        </>
      }
    >
      <TextInput
        label="Name"
        value={state.name}
        onChange={(name) => setState((prev) => ({ ...prev, name }))}
        fillWidth
        isRequired
        autoFocus
      />
      <SelectInput
        label="Role"
        options={SERVICE_ACCOUNT_ROLE_OPTIONS}
        value={roleToOptionId(state.role)}
        onChange={(id) => setState((prev) => ({ ...prev, role: optionIdToRole(id) }))}
        fillWidth
        isRequired
      />
      {state.error && (
        <Box fillWidth>
          <Alert variant={AlertVariant.ERROR}>{state.error}</Alert>
        </Box>
      )}
    </Dialog>
  );
};

export default SettingsServiceAccountsPanelCreateDialog;
