import { useCallback, useMemo } from "react";

import { create, type JsonValue } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { ArrowLeftIcon, ArrowRightIcon, CheckIcon, WarningCircleIcon } from "@phosphor-icons/react";
import { match } from "ts-pattern";

import Beacon, { BeaconVariant } from "@galaxy-io/dls/beacons/Beacon";
import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { AlignItems, FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Text, { TextVariant } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import type { ConfigField, ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import {
  type ConnectorSpec,
  GetConnectorRequestSchema,
  ValidateConfigRequestSchema,
} from "@/gen/ingestion/v1/providers_pb";

import Field from "@/components/fields/Field";
import {
  getConnectionScopedFields,
  getFieldDefaults,
  isFieldVisible,
} from "@/components/fields/utils";

import ErrorLayout from "@/layouts/ErrorLayout";

import ConnectorTile, { ConnectorTileSize } from "@/pages/connectors/components/ConnectorTile";
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

import { useGetConnectorQuery, useValidateConfigMutation } from "@/api/queries/connectors";

import { NOOP } from "@/constants";

import { getErrorMessage } from "@/utils/errors";

const BodyWrapper = withTheme(styled.div<PropsWithTheme>`
  flex: 1;
  min-height: 0;
  width: 100%;
  overflow-y: auto;
  background-color: ${({ theme }) => theme.color.background.base};
  padding: 16px;
`);

const FooterWrapper = withTheme(styled.div<PropsWithTheme>`
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px;
  background-color: ${({ theme }) => theme.color.background.primary};
`);

interface ConnectionFormProps {
  connectorName: ConnectorSpec["name"];
  connectorKind: ConnectorKind;
  connectionId?: Connection["id"];
  onSubmit: () => void;
  onClose: () => void;
  onBack?: () => void;
}

const ConnectionForm = ({
  connectorName,
  connectorKind,
  connectionId,
  onSubmit,
  onClose,
  onBack,
}: ConnectionFormProps) => {
  const { state, dispatch } = useConnectionFormContext();
  const { showToast } = useToast();

  const { data, isError } = useGetConnectorQuery({
    input: create(GetConnectorRequestSchema, {
      connector: connectorName,
      kind: connectorKind,
    }),
    options: { retry: false },
  });
  const connector = data?.connector;

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
        live: true,
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
            showToast({
              variant: ToastVariant.ERROR,
              header: "Validation failed",
              subheader: response.errors[0]?.message ?? "Connection could not be validated.",
            });
          }
        },
        onError: (error) => {
          dispatch({
            type: ConnectionFormActionType.SET_PHASE,
            payload: ConnectionFormPhase.ERROR,
          });
          showToast({
            variant: ToastVariant.ERROR,
            header: "Validation failed",
            subheader: getErrorMessage(error, "Validation failed"),
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
    showToast,
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

  const fieldDefaults = useMemo(() => getFieldDefaults(fields), [fields]);

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
              hasStoredSecret={!!connectionId}
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
        <FlexWrapper alignItems={AlignItems.CENTER} gap={16}>
          <FlexWrapper alignItems={AlignItems.CENTER} gap={6}>
            <Beacon variant={BeaconVariant.SUCCESS} />
            <Text variant={TextVariant.SUCCESS}>Connected</Text>
          </FlexWrapper>
          <Button
            size={ButtonSize.LARGE}
            label={submitLabel}
            icon={CheckIcon}
            variant={ButtonVariant.SUCCESS}
            onClick={onSubmit}
          />
        </FlexWrapper>
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
          icon={<Icon component={WarningCircleIcon} size={24} variant={IconVariant.ERROR} />}
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
        <FlexItem grow={0} shrink={0}>
          <FlexWrapper alignItems={AlignItems.CENTER} padding="12px 16px" gap={12} fillWidth>
            <FlexItem shrink={0}>
              <ConnectorTile
                connector={connectorName}
                kind={connectorKind}
                size={ConnectorTileSize.LARGE}
              />
            </FlexItem>
            <TextShimmer height={20} width={220} />
          </FlexWrapper>
        </FlexItem>
        <FlexItem grow={0} shrink={0}>
          <HorizontalDivider />
        </FlexItem>
        <BodyWrapper>
          <FlexWrapper direction={FlexDirection.COLUMN} gap={16} fillWidth>
            <TextInput
              value={state.name}
              onChange={handleNameChange}
              size={InputSize.LARGE}
              placeholder="Enter connection name..."
              label="Name"
              isRequired
              fillWidth
              autoFocus
            />
            <TextShimmer height={32} width="100%" />
            <TextShimmer height={32} width="100%" />
            <TextShimmer height={32} width="100%" />
          </FlexWrapper>
        </BodyWrapper>
        <FlexItem grow={0} shrink={0}>
          <HorizontalDivider />
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
          <Button
            size={ButtonSize.LARGE}
            label="Validate"
            icon={ArrowRightIcon}
            onClick={NOOP}
            isDisabled
            isIconTrailing
          />
        </FooterWrapper>
      </ConnectionFormWrapper>
    );
  }

  return (
    <ConnectionFormWrapper width={CREATE_CONNECTION_MODAL_CONFIGURE_WIDTH}>
      <FlexItem grow={0} shrink={0}>
        <ConnectionFormHeader
          connectorName={connectorName}
          connectorKind={connectorKind}
          title={`${connectionId ? "Edit" : "New"} ${connector.displayName || connector.name} connection`}
          onClose={onClose}
        />
      </FlexItem>
      <FlexItem grow={0} shrink={0}>
        <HorizontalDivider />
      </FlexItem>

      <BodyWrapper>
        <FlexWrapper direction={FlexDirection.COLUMN} gap={16} fillWidth>
          {renderBody()}
        </FlexWrapper>
      </BodyWrapper>

      <FlexItem grow={0} shrink={0}>
        <HorizontalDivider />
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
