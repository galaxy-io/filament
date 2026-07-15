import { useCallback, useMemo } from "react";

import { create, type JsonValue } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { ArrowRightIcon, CheckIcon } from "@phosphor-icons/react";
import { useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { match } from "ts-pattern";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import CreateConnectionModalHeader from "@/pages/connectors/components/create/components/CreateConnectionModalHeader";
import CreateConnectionModalWrapper from "@/pages/connectors/components/create/components/CreateConnectionModalWrapper";
import CreateConnectionNameInput from "@/pages/connectors/components/create/components/CreateConnectionNameInput";
import { CreateConnectionActionType } from "@/pages/connectors/components/create/configure/actions";
import CreateConnectionConfigureProvider from "@/pages/connectors/components/create/configure/CreateConnectionConfigureProvider";
import CreateConnectionField from "@/pages/connectors/components/create/configure/fields/CreateConnectionField";
import { useCreateConnectionContext } from "@/pages/connectors/components/create/configure/hooks";
import { CreateConnectionPhase } from "@/pages/connectors/components/create/configure/types";
import { getConnectorConfigSchemaConnectionFields } from "@/pages/connectors/components/create/configure/utils";
import {
  createRequiredFieldsValidationErrorMap,
  getNameError,
  isNameValid,
  validateRequiredFields,
} from "@/pages/connectors/components/create/configure/validation";
import { CreateConnectionModalStep } from "@/pages/connectors/components/create/types";

import { ToastVariant } from "@/providers/toast/Toast";
import { useToast } from "@/providers/toast/useToast";

import {
  createListConnectionsQueryKey,
  useCreateConnectionMutation,
  useValidateConfigMutation,
} from "@/api/queries/connectors";

import { CreateConnectionRequestSchema } from "@/gen/ingestion/v1/connections_pb";
import type { ConnectorSpec } from "@/gen/ingestion/v1/providers_pb";
import { ValidateConfigRequestSchema } from "@/gen/ingestion/v1/providers_pb";

import { NOOP } from "@/constants";

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
  const queryClient = useQueryClient();

  const { mutate: validateConfig } = useValidateConfigMutation();
  const { mutate: createConnection } = useCreateConnectionMutation();

  const fields = useMemo(() => getConnectorConfigSchemaConnectionFields(connector), [connector]);

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

  const validateAndShowErrors = useCallback((): boolean => {
    const errors = validateRequiredFields(fields, state.request.config ?? {});
    dispatch({
      type: CreateConnectionActionType.SET_SHOULD_SHOW_ERRORS,
      payload: true,
    });
    if (errors.length > 0) {
      dispatch({
        type: CreateConnectionActionType.SET_VALIDATION_ERRORS,
        payload: errors,
      });
    }
    return isNameValid(state.request.name) && errors.length === 0;
  }, [fields, state.request.config, state.request.name, dispatch]);

  const handleTestConnection = useCallback(() => {
    if (!validateAndShowErrors()) {
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
          }
        },
        onError: (error) => {
          const message = error instanceof Error ? error.message : "Validation failed";
          dispatch({
            type: CreateConnectionActionType.SET_ERROR,
            payload: message,
          });
          showToast({
            variant: ToastVariant.ERROR,
            header: "Validation failed",
            subheader: message,
          });
        },
      },
    );
  }, [state.request, validateAndShowErrors, validateConfig, dispatch, showToast]);

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
          void queryClient.invalidateQueries({
            queryKey: createListConnectionsQueryKey(),
          });
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
          const message = error instanceof Error ? error.message : "Creation failed";
          dispatch({
            type: CreateConnectionActionType.SET_ERROR,
            payload: message,
          });
          showToast({
            variant: ToastVariant.ERROR,
            header: "Creation failed",
            subheader: message,
          });
        },
      },
    );
  }, [state.request, createConnection, queryClient, showToast, navigate, dispatch]);

  // Field handlers (dispatch directly - GX style)
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

  const handleTest = useCallback(() => {
    handleTestConnection();
  }, [handleTestConnection]);

  const handleCreate = useCallback(() => {
    handleCreateConnection();
  }, [handleCreateConnection]);

  const getFieldValue = (fieldName: string): JsonValue => {
    return state.request.config?.[fieldName] ?? null;
  };

  const renderBody = () => {
    return (
      <>
        <CreateConnectionNameInput
          value={state.request.name}
          onChange={handleNameChange}
          error={nameError}
          isDisabled={isDisabled}
        />

        {fields.map((field) => (
          <CreateConnectionField
            key={field.name}
            field={field}
            value={getFieldValue(field.name)}
            onChange={(value) => handleFieldChange(field.name, value)}
            error={getFieldError(field.name)}
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
          label="Test Connection"
          icon={ArrowRightIcon}
          size={ButtonSize.LARGE}
          onClick={handleTest}
          isDisabled={isDisabled}
          isIconTrailing
          fillWidth
        />
      ))
      .with(CreateConnectionPhase.VALIDATING, () => (
        <Button
          label="Testing..."
          size={ButtonSize.LARGE}
          onClick={NOOP}
          isLoading={isValidating}
          isDisabled
          fillWidth
        />
      ))
      .with(CreateConnectionPhase.VALIDATED, () => (
        <Button
          label="Create Connection"
          icon={CheckIcon}
          variant={ButtonVariant.SUCCESS}
          size={ButtonSize.LARGE}
          onClick={handleCreate}
          fillWidth
        />
      ))
      .with(CreateConnectionPhase.CREATING, () => (
        <Button
          label="Creating..."
          size={ButtonSize.LARGE}
          onClick={NOOP}
          isLoading={isCreating}
          isDisabled
          fillWidth
        />
      ))
      .exhaustive();
  };

  return (
    <CreateConnectionModalWrapper step={CreateConnectionModalStep.CONFIGURE}>
      <FlexItem shrink={0}>
        <CreateConnectionModalHeader connector={connector} onClose={onClose} onBack={onBack} />
      </FlexItem>
      <FlexItem shrink={0}>
        <HorizontalDivider />
      </FlexItem>

      <BodyWrapper>
        <FlexWrapper direction={FlexDirection.COLUMN} gap={16} fillWidth>
          {renderBody()}
        </FlexWrapper>
      </BodyWrapper>

      <FlexItem shrink={0}>
        <HorizontalDivider />
      </FlexItem>
      <FooterWrapper>{renderFooter()}</FooterWrapper>
    </CreateConnectionModalWrapper>
  );
};

const CreateConnectionConfigure = (props: CreateConnectionConfigureProps) => (
  <CreateConnectionConfigureProvider connector={props.connector}>
    <CreateConnectionConfigureContent {...props} />
  </CreateConnectionConfigureProvider>
);

export default CreateConnectionConfigure;
