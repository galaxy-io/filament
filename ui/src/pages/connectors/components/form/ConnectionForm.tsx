import { useCallback, useMemo } from "react";

import { create, type JsonValue } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { ArrowLeftIcon, ArrowRightIcon, CheckIcon } from "@phosphor-icons/react";
import { match } from "ts-pattern";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import type { ConnectorSpec } from "@/gen/ingestion/v1/providers_pb";
import { ValidateConfigRequestSchema } from "@/gen/ingestion/v1/providers_pb";

import Field from "@/components/fields/Field";
import { getConnectionScopedFields } from "@/components/fields/utils";

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
import { CREATE_CONNECTION_MODAL_CONFIGURE_WIDTH } from "@/pages/connectors/constants";

import { useValidateConfigMutation } from "@/api/queries/connectors";

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
  connector: ConnectorSpec;
  connectionId?: string;
  title: string;
  submitLabel: string;
  submittingLabel: string;
  onSubmit: () => void;
  onClose: () => void;
  onBack?: () => void;
}

const ConnectionForm = ({
  connector,
  connectionId,
  title,
  submitLabel,
  submittingLabel,
  onSubmit,
  onClose,
  onBack,
}: ConnectionFormProps) => {
  const { state, dispatch } = useConnectionFormContext();
  const { showToast } = useToast();

  const { mutate: validateConfig } = useValidateConfigMutation();

  const fields = useMemo(
    () => getConnectionScopedFields(connector.configSchema?.fields ?? []),
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
    (fieldName: string): string | undefined =>
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
        kind: connector.kind,
        connector: connector.name,
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
            showToast({
              variant: ToastVariant.SUCCESS,
              header: "Connection validated",
              subheader: "Your connection settings are valid.",
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
  }, [state.name, state.config, connector, connectionId, validateConfig, dispatch, showToast]);

  const handleNameChange = useCallback(
    (name: string) =>
      dispatch({
        type: ConnectionFormActionType.SET_NAME,
        payload: name,
      }),
    [dispatch],
  );

  const handleFieldChange = useCallback(
    (fieldName: string, value: JsonValue) =>
      dispatch({
        type: ConnectionFormActionType.SET_CONFIG_FIELD,
        payload: { field: fieldName, value },
      }),
    [dispatch],
  );

  const getFieldValue = (fieldName: string): JsonValue => {
    return state.config[fieldName] ?? null;
  };

  const renderBody = () => {
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
        {fields.map((field) => (
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
        <Button
          size={ButtonSize.LARGE}
          label={submitLabel}
          icon={CheckIcon}
          variant={ButtonVariant.SUCCESS}
          onClick={onSubmit}
        />
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

  return (
    <ConnectionFormWrapper width={CREATE_CONNECTION_MODAL_CONFIGURE_WIDTH}>
      <FlexItem grow={0} shrink={0}>
        <ConnectionFormHeader connector={connector} title={title} onClose={onClose} />
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
            variant={ButtonVariant.TERTIARY}
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
