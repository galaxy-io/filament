import { useCallback, useMemo } from "react";

import { create, type JsonValue } from "@bufbuild/protobuf";
import { ArrowLeftIcon, ArrowRightIcon, CheckIcon } from "@phosphor-icons/react";
import { match } from "ts-pattern";

import Beacon, { BeaconVariant } from "@galaxy-io/dls/beacons/Beacon";
import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import SelectInput from "@galaxy-io/dls/inputs/SelectInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import { ModalSize } from "@galaxy-io/dls/modal/Modal";
import Text, { TextVariant } from "@galaxy-io/dls/text/Text";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import type { ConfigField, ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import {
  type ConnectorSpec,
  GetConnectorRequestSchema,
  ValidateConfigRequestSchema,
} from "@/gen/ingestion/v1/connectors_pb";

import Field from "@/components/fields/Field";
import FieldWrapper from "@/components/fields/FieldWrapper";
import {
  getConnectionScopedFields,
  getFieldDefaults,
  isFieldVisible,
} from "@/components/fields/utils";

import ErrorLayout from "@/layouts/ErrorLayout";
import PendingLayout from "@/layouts/PendingLayout";

import {
  getConnectorFamily,
  getConnectorVersions,
} from "@/pages/connectors/components/create/utils";
import { ConnectionFormActionType } from "@/pages/connectors/components/form/actions";
import ConnectionFormHeader from "@/pages/connectors/components/form/ConnectionFormHeader";
import ConnectionFormMaturityAlert from "@/pages/connectors/components/form/ConnectionFormMaturityAlert";
import { useConnectionFormContext } from "@/pages/connectors/components/form/ConnectionFormProvider";
import ConnectionFormWrapper from "@/pages/connectors/components/form/ConnectionFormWrapper";
import { ConnectionFormPhase } from "@/pages/connectors/components/form/types";
import {
  createRequiredFieldsValidationErrorMap,
  getNameError,
  isNameValid,
} from "@/pages/connectors/components/form/validation";
import { CONNECTOR_KIND_TO_LABEL_MAP } from "@/pages/connectors/constants";

import {
  useGetConnectorQuery,
  useListConnectorsQuery,
  useValidateConfigMutation,
} from "@/api/queries/connectors";

import { NOOP } from "@/constants";

import { getErrorMessage } from "@/utils/errors";

interface ConnectionFormProps {
  connectorName: ConnectorSpec["name"];
  connectorKind: ConnectorKind;
  connectionId?: Connection["id"];
  secretRefs?: Connection["secretRefs"];
  onSubmit: () => void;
  onClose: () => void;
  onBack?: () => void;
  onConnectorChange?: (connectorName: string) => void;
}

const ConnectionForm = ({
  connectorName,
  connectorKind,
  connectionId,
  secretRefs,
  onSubmit,
  onClose,
  onBack,
  onConnectorChange,
}: ConnectionFormProps) => {
  const { state, dispatch } = useConnectionFormContext();
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
  const submittingLabel = connectionId ? "Saving..." : "Creating...";

  const { mutate: validateConfig } = useValidateConfigMutation();

  const fields = useMemo(
    () => getConnectionScopedFields(connector?.configSchema?.fields ?? []),
    [connector],
  );

  const isDisabled =
    state.phase === ConnectionFormPhase.VALIDATING ||
    state.phase === ConnectionFormPhase.SUBMITTING;

  const isValidating = state.phase === ConnectionFormPhase.VALIDATING;
  const isSubmitting = state.phase === ConnectionFormPhase.SUBMITTING;

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
    dispatch({
      type: ConnectionFormActionType.SET_SHOULD_SHOW_ERRORS,
      payload: true,
    });
    if (!isNameValid(state.name)) {
      dispatch({
        type: ConnectionFormActionType.SET_PHASE,
        payload: ConnectionFormPhase.ERROR,
      });
      return;
    }

    dispatch({
      type: ConnectionFormActionType.SET_PHASE,
      payload: ConnectionFormPhase.VALIDATING,
    });
    dispatch({
      type: ConnectionFormActionType.SET_VALIDATION_ERRORS,
      payload: [],
    });

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
            dispatch({
              type: ConnectionFormActionType.SET_PHASE,
              payload: ConnectionFormPhase.VALIDATED,
            });
          } else {
            dispatch({
              type: ConnectionFormActionType.SET_VALIDATION_ERRORS,
              payload: response.errors,
            });
            dispatch({
              type: ConnectionFormActionType.SET_PHASE,
              payload: ConnectionFormPhase.ERROR,
            });
            toast({
              variant: ToastVariant.ERROR,
              header: "Validation failed",
              description: response.errors[0]?.message ?? "Connection could not be validated.",
            });
          }
        },
        onError: (error) => {
          dispatch({
            type: ConnectionFormActionType.SET_PHASE,
            payload: ConnectionFormPhase.ERROR,
          });
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
    dispatch,
    toast,
  ]);

  const handleNameChange = useCallback(
    (name: Connection["name"]) =>
      dispatch({
        type: ConnectionFormActionType.SET_NAME,
        payload: name,
      }),
    [dispatch],
  );

  const handleFieldChange = useCallback(
    (fieldName: ConfigField["name"], value: JsonValue) =>
      dispatch({
        type: ConnectionFormActionType.SET_CONFIG_FIELD,
        payload: { field: fieldName, value },
      }),
    [dispatch],
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
          <FieldWrapper label="API version">
            <SelectInput
              options={versionOptions}
              value={connector?.name ?? null}
              onChange={(id) => {
                if (id) onConnectorChange(id);
              }}
              isDisabled={isDisabled || versionOptions.length === 1}
              fillWidth
            />
          </FieldWrapper>
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
      .with(ConnectionFormPhase.VALIDATING, () => (
        <Button label="Testing..." onClick={NOOP} isLoading={isValidating} isDisabled />
      ))
      .with(ConnectionFormPhase.VALIDATED, () => (
        <Flex alignItems={AlignItems.CENTER} gap={16}>
          <Flex alignItems={AlignItems.CENTER} gap={4}>
            <Beacon variant={BeaconVariant.SUCCESS} />
            <Text variant={TextVariant.SUCCESS}>Connected</Text>
          </Flex>
          <Button label={submitLabel} icon={CheckIcon} onClick={onSubmit} />
        </Flex>
      ))
      .with(ConnectionFormPhase.SUBMITTING, () => (
        <Button label={submittingLabel} onClick={NOOP} isLoading={isSubmitting} isDisabled />
      ))
      .exhaustive();
  };

  if (isError) {
    return (
      <ConnectionFormWrapper size={ModalSize.MEDIUM} header="Connector not found" onClose={onClose}>
        <ErrorLayout
          header="Connector not found"
          message={`No ${CONNECTOR_KIND_TO_LABEL_MAP[connectorKind].toLowerCase()} connector named "${connectorName}" is available.`}
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
      size={ModalSize.MEDIUM}
      header={
        <ConnectionFormHeader
          connectorName={connectorName}
          connectorKind={connectorKind}
          title={`${connectionId ? "Edit" : "New"} ${connector.displayName || connector.name} connection`}
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
