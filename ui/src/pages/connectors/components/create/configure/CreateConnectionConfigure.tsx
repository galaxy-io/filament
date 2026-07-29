import { useCallback, useMemo } from "react";

import { create, type JsonValue } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { ArrowLeftIcon, ArrowRightIcon, CheckIcon } from "@phosphor-icons/react";
import { useNavigate } from "@tanstack/react-router";
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

import { CreateConnectionRequestSchema } from "@/gen/ingestion/v1/connections_pb";
import type { ConnectorSpec } from "@/gen/ingestion/v1/providers_pb";
import { ValidateConfigRequestSchema } from "@/gen/ingestion/v1/providers_pb";

import Field from "@/components/fields/Field";
import { getConnectionScopedFields } from "@/components/fields/utils";

import { CreateConnectionActionType } from "@/pages/connectors/components/create/configure/actions";
import CreateConnectionConfigureHeader from "@/pages/connectors/components/create/configure/CreateConnectionConfigureHeader";
import CreateConnectionConfigureProvider, {
  useCreateConnectionContext,
} from "@/pages/connectors/components/create/configure/CreateConnectionConfigureProvider";
import CreateConnectionConfigureWrapper from "@/pages/connectors/components/create/configure/CreateConnectionConfigureWrapper";
import { CreateConnectionPhase } from "@/pages/connectors/components/create/configure/types";
import {
  createRequiredFieldsValidationErrorMap,
  getNameError,
  isNameValid,
} from "@/pages/connectors/components/create/configure/validation";
import { CreateConnectionModalStep } from "@/pages/connectors/components/create/types";

import { useCreateConnectionMutation } from "@/api/queries/connections";
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

interface CreateConnectionConfigureProps {
  connector: ConnectorSpec;
  onClose: () => void;
  onBack: () => void;
}

const CreateConnectionConfigureContent = ({
  connector,
  onClose,
  onBack,
}: CreateConnectionConfigureProps) => {
  const navigate = useNavigate();
  const { state, dispatch } = useCreateConnectionContext();
  const { showToast } = useToast();

  const { mutate: validateConfig } = useValidateConfigMutation();
  const { mutate: createConnection } = useCreateConnectionMutation();

  const fields = useMemo(
    () => getConnectionScopedFields(connector.configSchema?.fields ?? []),
    [connector],
  );

  const isDisabled =
    state.phase === CreateConnectionPhase.VALIDATING ||
    state.phase === CreateConnectionPhase.CREATING;

  const isValidating = state.phase === CreateConnectionPhase.VALIDATING;
  const isCreating = state.phase === CreateConnectionPhase.CREATING;

  const nameError = useMemo(
    () => getNameError(state.request.name, state.shouldShowErrors),
    [state.request.name, state.shouldShowErrors],
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
      type: CreateConnectionActionType.SET_SHOULD_SHOW_ERRORS,
      payload: true,
    });
    if (!isNameValid(state.request.name)) {
      dispatch({
        type: CreateConnectionActionType.SET_PHASE,
        payload: CreateConnectionPhase.ERROR,
      });
      return;
    }

    dispatch({
      type: CreateConnectionActionType.SET_PHASE,
      payload: CreateConnectionPhase.VALIDATING,
    });
    dispatch({
      type: CreateConnectionActionType.SET_VALIDATION_ERRORS,
      payload: [],
    });

    validateConfig(
      create(ValidateConfigRequestSchema, {
        kind: state.request.kind,
        connector: state.request.connector,
        config: state.request.config,
        live: true,
      }),
      {
        onSuccess: (response) => {
          if (response.valid) {
            dispatch({
              type: CreateConnectionActionType.SET_PHASE,
              payload: CreateConnectionPhase.VALIDATED,
            });
            showToast({
              variant: ToastVariant.SUCCESS,
              header: "Connection validated",
              subheader: "Your connection settings are valid.",
            });
          } else {
            dispatch({
              type: CreateConnectionActionType.SET_VALIDATION_ERRORS,
              payload: response.errors,
            });
            dispatch({
              type: CreateConnectionActionType.SET_PHASE,
              payload: CreateConnectionPhase.ERROR,
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
            type: CreateConnectionActionType.SET_PHASE,
            payload: CreateConnectionPhase.ERROR,
          });
          showToast({
            variant: ToastVariant.ERROR,
            header: "Validation failed",
            subheader: getErrorMessage(error, "Validation failed"),
          });
        },
      },
    );
  }, [state.request, validateConfig, dispatch, showToast]);

  const handleCreateConnection = useCallback(() => {
    const name = state.request.name?.trim();
    if (!name) return;

    dispatch({
      type: CreateConnectionActionType.SET_PHASE,
      payload: CreateConnectionPhase.CREATING,
    });

    createConnection(
      create(CreateConnectionRequestSchema, {
        ...state.request,
        name,
      }),
      {
        onSuccess: (response) => {
          showToast({
            variant: ToastVariant.SUCCESS,
            header: "Connection created",
            subheader: `${name} has been created successfully.`,
          });

          if (response.connection?.id) {
            void navigate({
              to: ".",
              search: (prev) => ({
                ...prev,
                flow: undefined,
                connectionId: response.connection?.id,
              }),
            });
          }
        },
        onError: (error) => {
          dispatch({
            type: CreateConnectionActionType.SET_PHASE,
            payload: CreateConnectionPhase.ERROR,
          });
          showToast({
            variant: ToastVariant.ERROR,
            header: "Creation failed",
            subheader: getErrorMessage(error, "Creation failed"),
          });
        },
      },
    );
  }, [state.request, createConnection, showToast, navigate, dispatch]);

  const handleNameChange = useCallback(
    (name: string) =>
      dispatch({
        type: CreateConnectionActionType.SET_REQUEST_NAME,
        payload: name,
      }),
    [dispatch],
  );

  const handleFieldChange = useCallback(
    (fieldName: string, value: JsonValue) =>
      dispatch({
        type: CreateConnectionActionType.SET_REQUEST_CONFIG_FIELD,
        payload: { field: fieldName, value },
      }),
    [dispatch],
  );

  const getFieldValue = (fieldName: string): JsonValue => {
    return state.request.config?.[fieldName] ?? null;
  };

  const renderBody = () => {
    return (
      <>
        <TextInput
          value={state.request.name}
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
          />
        ))}
      </>
    );
  };

  const renderFooter = () => {
    return match(state.phase)
      .with(CreateConnectionPhase.IDLE, CreateConnectionPhase.ERROR, () => (
        <Button
          size={ButtonSize.LARGE}
          label="Validate"
          icon={ArrowRightIcon}
          onClick={handleTestConnection}
          isDisabled={isDisabled}
          isIconTrailing
        />
      ))
      .with(CreateConnectionPhase.VALIDATING, () => (
        <Button
          size={ButtonSize.LARGE}
          label="Testing..."
          onClick={NOOP}
          isLoading={isValidating}
          isDisabled
        />
      ))
      .with(CreateConnectionPhase.VALIDATED, () => (
        <Button
          size={ButtonSize.LARGE}
          label="Create"
          icon={CheckIcon}
          variant={ButtonVariant.SUCCESS}
          onClick={handleCreateConnection}
        />
      ))
      .with(CreateConnectionPhase.CREATING, () => (
        <Button
          size={ButtonSize.LARGE}
          label="Creating..."
          onClick={NOOP}
          isLoading={isCreating}
          isDisabled
        />
      ))
      .exhaustive();
  };

  return (
    <CreateConnectionConfigureWrapper step={CreateConnectionModalStep.CONFIGURE}>
      <FlexItem grow={0} shrink={0}>
        <CreateConnectionConfigureHeader connector={connector} onClose={onClose} />
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
        <Button
          size={ButtonSize.LARGE}
          onClick={onBack}
          icon={ArrowLeftIcon}
          label="Back"
          variant={ButtonVariant.TERTIARY}
        />
        {renderFooter()}
      </FooterWrapper>
    </CreateConnectionConfigureWrapper>
  );
};

const CreateConnectionConfigure = ({
  connector,
  onClose,
  onBack,
}: CreateConnectionConfigureProps) => (
  <CreateConnectionConfigureProvider connector={connector}>
    <CreateConnectionConfigureContent connector={connector} onClose={onClose} onBack={onBack} />
  </CreateConnectionConfigureProvider>
);

export default CreateConnectionConfigure;
