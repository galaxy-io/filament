import { useCallback, useMemo } from "react";

import { create, type JsonValue } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { ArrowLeftIcon, ArrowRightIcon, CheckIcon } from "@phosphor-icons/react";
import { match } from "ts-pattern";

import Beacon, { BeaconVariant } from "@galaxy-io/dls/beacons/Beacon";
import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import SelectInput, { SelectInputSize } from "@galaxy-io/dls/inputs/SelectInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text, { TextVariant } from "@galaxy-io/dls/text/Text";
import { t } from "@galaxy-io/dls/theme/tokens/t";
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
import { useConnectionFormContext } from "@/pages/connectors/components/form/ConnectionFormProvider";
import ConnectionFormWrapper from "@/pages/connectors/components/form/ConnectionFormWrapper";
import { ConnectionFormPhase } from "@/pages/connectors/components/form/types";
import {
  createRequiredFieldsValidationErrorMap,
  getNameError,
  isNameValid,
} from "@/pages/connectors/components/form/validation";
import {
  CONNECTOR_KIND_TO_LABEL_MAP,
  CREATE_CONNECTION_MODAL_CONFIGURE_WIDTH,
} from "@/pages/connectors/constants";

import {
  useGetConnectorQuery,
  useListConnectorsQuery,
  useValidateConfigMutation,
} from "@/api/queries/connectors";

import { NOOP } from "@/constants";

import { getErrorMessage } from "@/utils/errors";

const BodyWrapper = styled.div`
  flex: 1;
  min-height: 0;
  width: 100%;
  overflow-y: auto;
  background-color: ${t.color.background.base};
  padding: 16px;
`;

const FooterWrapper = styled.div`
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px;
  background-color: ${t.color.background.primary};
`;

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
    value: version.name,
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
          size={InputSize.LARGE}
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
              /* @dls-migrate selectinput.value: `value` and `onChange` now carry option ids, not option objects. */ value={
                versionOptions.find((option) => option.value === connector?.name) ?? null
              }
              onChange={(option) => onConnectorChange(option.value as string)}
              isDisabled={isDisabled || versionOptions.length === 1}
              size={SelectInputSize.LARGE}
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
          size={ButtonSize.LARGE}
          label="Validate"
          icon={ArrowRightIcon}
          onClick={handleTestConnection}
          isDisabled={isDisabled}
          isIconTrailing
        />
      ))
      .with(ConnectionFormPhase.VALIDATING, () => (
        <Button
          size={ButtonSize.LARGE}
          label="Testing..."
          onClick={NOOP}
          isLoading={isValidating}
          isDisabled
        />
      ))
      .with(ConnectionFormPhase.VALIDATED, () => (
        <Flex alignItems={AlignItems.CENTER} gap={16}>
          <Flex
            alignItems={
              AlignItems.CENTER
            } /* @dls-migrate layout.off-scale: Pick a value on the space scale (or a CSS-order tuple of them). */
            gap={6}
          >
            <Beacon variant={BeaconVariant.SUCCESS} />
            <Text variant={TextVariant.SUCCESS}>Connected</Text>
          </Flex>
          <Button
            size={ButtonSize.LARGE}
            label={submitLabel}
            icon={CheckIcon}
            /* @dls-migrate button.ButtonVariant.SUCCESS: Removed: use `PRIMARY`, and show success with `Chip` or a labeled `Beacon`. */ variant={
              ButtonVariant.SUCCESS
            }
            onClick={onSubmit}
          />
        </Flex>
      ))
      .with(ConnectionFormPhase.SUBMITTING, () => (
        <Button
          size={ButtonSize.LARGE}
          label={submittingLabel}
          onClick={NOOP}
          isLoading={isSubmitting}
          isDisabled
        />
      ))
      .exhaustive();
  };

  if (isError) {
    return (
      <ConnectionFormWrapper width={CREATE_CONNECTION_MODAL_CONFIGURE_WIDTH}>
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
      <ConnectionFormWrapper width={CREATE_CONNECTION_MODAL_CONFIGURE_WIDTH}>
        <PendingLayout />
      </ConnectionFormWrapper>
    );
  }

  return (
    <ConnectionFormWrapper width={CREATE_CONNECTION_MODAL_CONFIGURE_WIDTH}>
      <FlexItem grow={0} shrink={0}>
        <ConnectionFormHeader
          connectorName={connectorName}
          connectorKind={connectorKind}
          connectorMaturity={connector.maturity}
          connectorApiVersion={connector.apiVersion}
          title={`${connectionId ? "Edit" : "New"} ${connector.displayName || connector.name} connection`}
          onClose={onClose}
        />
      </FlexItem>
      <FlexItem grow={0} shrink={0}>
        <Divider />
      </FlexItem>

      <BodyWrapper>
        <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={16} fillWidth>
          {renderBody()}
        </Flex>
      </BodyWrapper>

      <FlexItem grow={0} shrink={0}>
        <Divider />
      </FlexItem>
      <FooterWrapper>
        {onBack ? (
          <Button
            size={ButtonSize.LARGE}
            onClick={onBack}
            icon={ArrowLeftIcon}
            label="Back"
            variant={ButtonVariant.SECONDARY}
          />
        ) : (
          <div />
        )}
        {renderFooter()}
      </FooterWrapper>
    </ConnectionFormWrapper>
  );
};

export default ConnectionForm;
