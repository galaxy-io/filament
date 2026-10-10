import { type FC, useCallback, useMemo } from "react";

import { create, type JsonValue } from "@bufbuild/protobuf";
import { ArrowLeftIcon, ArrowRightIcon, CheckIcon } from "@phosphor-icons/react";
import { match } from "ts-pattern";

import Beacon, { BeaconVariant } from "@galaxy-io/dls/beacons/Beacon";
import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import SelectInput from "@galaxy-io/dls/inputs/SelectInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import ErrorLayout from "@galaxy-io/dls/layout/ErrorLayout";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import PendingLayout from "@galaxy-io/dls/layout/PendingLayout";
import { ModalSize } from "@galaxy-io/dls/modal/Modal";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import type { ConfigField, ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import {
  type ConnectorSpec,
  GetConnectorRequestSchema,
  ValidateConfigRequestSchema,
} from "@/gen/ingestion/v1/connectors_pb";

import { CONNECTOR_KIND_TO_NOUN_MAP } from "@/components/connections/constants";
import { formatConnectorName, getConnectorFamily } from "@/components/connections/utils";
import Field from "@/components/fields/Field";
import {
  getConnectionScopedFields,
  getFieldDefaults,
  isFieldVisible,
} from "@/components/fields/utils";

import { getConnectorVersions } from "@/pages/connections/components/create/utils";
import ConnectionFormHeader from "@/pages/connections/components/form/ConnectionFormHeader";
import ConnectionFormMaturityAlert from "@/pages/connections/components/form/ConnectionFormMaturityAlert";
import {
  useConnectionFormActions,
  useConnectionFormState,
} from "@/pages/connections/components/form/ConnectionFormProvider";
import ConnectionFormWrapper from "@/pages/connections/components/form/ConnectionFormWrapper";
import { ConnectionFormPhase } from "@/pages/connections/components/form/types";
import { createRequiredFieldsValidationErrorMap } from "@/pages/connections/components/form/validation";

import {
  useGetConnectorQuery,
  useListConnectorsQuery,
  useValidateConfigMutation,
} from "@/api/queries/connectors";

import { getErrorMessage } from "@/utils/errors";
import { getNameError, isNameValid } from "@/utils/validation";

interface ConnectionFormProps {
  isOpen: boolean;
  connectorName: ConnectorSpec["name"];
  connectorKind: ConnectorKind;
  connectionId?: Connection["id"];
  secretRefs?: Connection["secretRefs"];
  onSubmit: () => void;
  onClose: () => void;
  onBack?: () => void;
  onConnectorChange?: (connectorName: string) => void;
}

const ConnectionForm: FC<ConnectionFormProps> = ({
  isOpen,
  connectorName,
  connectorKind,
  connectionId,
  secretRefs,
  onSubmit,
  onClose,
  onBack,
  onConnectorChange,
}) => {
  const state = useConnectionFormState();
  const { setConfigField, setName, setPhase, setShouldShowErrors, setValidationErrors } =
    useConnectionFormActions();
  const { toast } = useToast();

  const { data, isError } = useGetConnectorQuery({
    input: create(GetConnectorRequestSchema, { connector: connectorName, kind: connectorKind }),
    options: { retry: false },
  });
  const connector = data?.connector;
  const { data: catalogData } = useListConnectorsQuery({
    options: { enabled: !!onConnectorChange },
  });
  const catalog = catalogData?.connectors ?? [];
  const family = connector ? getConnectorFamily(connector, catalog) : undefined;
  const versions = family ? getConnectorVersions(family, catalog) : [];
  const versionOptions = versions.map((version) => ({
    id: version.name,
    label: `${version.apiVersion || version.version}${version.name === family?.aliasTarget ? " (default)" : ""}`,
  }));

  const submitLabel = connectionId ? "Save" : "Create";

  const { mutate: validateConfig } = useValidateConfigMutation();

  const fields = useMemo(
    () => getConnectionScopedFields(connector?.configSchema?.fields ?? []),
    [connector],
  );

  const isDisabled =
    state.phase === ConnectionFormPhase.VALIDATING ||
    state.phase === ConnectionFormPhase.SUBMITTING;

  const nameError = useMemo(
    () => getNameError(state.name, state.shouldShowErrors),
    [state.name, state.shouldShowErrors],
  );

  const errorMap = useMemo(
    () => createRequiredFieldsValidationErrorMap(state.validationErrors),
    [state.validationErrors],
  );

  const getFieldError = useCallback(
    (fieldName: ConfigField["name"]): string | undefined =>
      state.shouldShowErrors ? errorMap.get(fieldName) : undefined,
    [state.shouldShowErrors, errorMap],
  );

  const handleTestConnection = useCallback(() => {
    setShouldShowErrors(true);
    if (!isNameValid(state.name)) {
      setPhase(ConnectionFormPhase.ERROR);
      return;
    }

    setPhase(ConnectionFormPhase.VALIDATING);
    setValidationErrors([]);

    validateConfig(
      create(ValidateConfigRequestSchema, {
        kind: connectorKind,
        connector: connectorName,
        config: state.config,
        connectionId: connectionId ?? "",
      }),
      {
        onSuccess: (response) => {
          if (response.valid) {
            setPhase(ConnectionFormPhase.VALIDATED);
          } else {
            setValidationErrors(response.errors);
            setPhase(ConnectionFormPhase.ERROR);
            toast({
              variant: ToastVariant.ERROR,
              header: "Validation failed",
              description: response.errors[0]?.message ?? "Connection could not be validated.",
            });
          }
        },
        onError: (error) => {
          setPhase(ConnectionFormPhase.ERROR);
          toast({
            variant: ToastVariant.ERROR,
            header: "Validation failed",
            description: getErrorMessage(error, "Validation failed"),
          });
        },
      },
    );
  }, [
    state.name,
    state.config,
    connectorName,
    connectorKind,
    connectionId,
    validateConfig,
    setPhase,
    setShouldShowErrors,
    setValidationErrors,
    toast,
  ]);

  const handleNameChange = useCallback((name: Connection["name"]) => setName(name), [setName]);

  const handleFieldChange = useCallback(
    (fieldName: ConfigField["name"], value: JsonValue) =>
      setConfigField({ field: fieldName, value }),
    [setConfigField],
  );

  const fieldDefaults = useMemo(
    () => getFieldDefaults(fields, state.config),
    [fields, state.config],
  );

  const getFieldValue = (fieldName: ConfigField["name"]): JsonValue => {
    return state.config[fieldName] ?? fieldDefaults[fieldName] ?? null;
  };

  const renderBody = () => {
    const fieldValues = Object.fromEntries(fields.map((f) => [f.name, getFieldValue(f.name)]));
    return (
      <>
        <TextInput
          value={state.name}
          onChange={handleNameChange}
          placeholder="Enter connection name..."
          label="Name"
          isRequired
          error={nameError ?? undefined}
          isDisabled={isDisabled}
          fillWidth
          autoFocus
        />
        {onConnectorChange && versionOptions.length > 0 && (
          <SelectInput
            label="API version"
            options={versionOptions}
            value={connector?.name ?? null}
            onChange={(id) => {
              if (id) onConnectorChange(id);
            }}
            isDisabled={isDisabled || versionOptions.length === 1}
            fillWidth
          />
        )}
        {fields
          .filter((field) => isFieldVisible(field, fieldValues))
          .map((field) => (
            <Field
              key={field.name}
              field={field}
              value={getFieldValue(field.name)}
              onChange={(value) => handleFieldChange(field.name, value)}
              getError={getFieldError}
              isDisabled={isDisabled}
              storedSecretRefs={secretRefs}
            />
          ))}
      </>
    );
  };

  const renderFooter = () => {
    return match(state.phase)
      .with(ConnectionFormPhase.IDLE, ConnectionFormPhase.ERROR, () => (
        <Button
          label="Validate"
          icon={ArrowRightIcon}
          onClick={handleTestConnection}
          isDisabled={isDisabled}
          isIconTrailing
        />
      ))
      .with(ConnectionFormPhase.VALIDATING, () => <Button label="Validate" isLoading />)
      .with(ConnectionFormPhase.VALIDATED, () => (
        <Flex alignItems={AlignItems.CENTER} gap={16}>
          <Beacon variant={BeaconVariant.SUCCESS} label="Connected" />
          <Button label={submitLabel} icon={CheckIcon} onClick={onSubmit} />
        </Flex>
      ))
      .with(ConnectionFormPhase.SUBMITTING, () => <Button label={submitLabel} isLoading />)
      .exhaustive();
  };

  if (isError) {
    return (
      <ConnectionFormWrapper
        isOpen={isOpen}
        size={ModalSize.MEDIUM}
        header="Connector not found"
        onClose={onClose}
      >
        <ErrorLayout
          header="Connector not found"
          description={`No ${CONNECTOR_KIND_TO_NOUN_MAP[connectorKind]} connector named "${connectorName}" is available.`}
          actions={
            <Button
              label={onBack ? "Choose a connector" : "Close"}
              onClick={onBack ?? onClose}
              variant={ButtonVariant.SECONDARY}
            />
          }
        />
      </ConnectionFormWrapper>
    );
  }

  if (!connector) {
    return (
      <ConnectionFormWrapper
        isOpen={isOpen}
        size={ModalSize.MEDIUM}
        header={connectionId ? "Edit connection" : "New connection"}
        onClose={onClose}
      >
        <PendingLayout />
      </ConnectionFormWrapper>
    );
  }

  return (
    <ConnectionFormWrapper
      isOpen={isOpen}
      size={ModalSize.MEDIUM}
      header={
        <ConnectionFormHeader
          connectorName={connectorName}
          connectorKind={connectorKind}
          title={`${connectionId ? "Edit" : "New"} ${formatConnectorName(connector)} connection`}
        />
      }
      footer={
        <Flex
          alignItems={AlignItems.CENTER}
          justifyContent={onBack ? JustifyContent.SPACE_BETWEEN : JustifyContent.END}
          fillWidth
        >
          {onBack && (
            <Button
              onClick={onBack}
              icon={ArrowLeftIcon}
              label="Back"
              variant={ButtonVariant.SECONDARY}
            />
          )}
          {renderFooter()}
        </Flex>
      }
      onClose={onClose}
    >
      <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={16} fillWidth>
        <ConnectionFormMaturityAlert
          connectorName={connectorName}
          connectorKind={connectorKind}
          connectorMaturity={connector.maturity}
          connectorApiVersion={connector.apiVersion}
        />
        {renderBody()}
      </Flex>
    </ConnectionFormWrapper>
  );
};

export default ConnectionForm;
