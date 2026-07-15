import { useCallback, useContext, useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { useQueryClient } from "@tanstack/react-query";

import { ToastVariant } from "@/providers/toast/Toast";
import { useToast } from "@/providers/toast/useToast";

import {
  createListConnectionsQueryKey,
  useCreateConnectionMutation,
  usePutSecretMutation,
  useValidateConfigMutation,
} from "@/api/queries/connectors";

import { FieldType } from "@/gen/ingestion/v1/common_pb";
import { CreateConnectionRequestSchema } from "@/gen/ingestion/v1/connections_pb";
import { ValidateConfigRequestSchema } from "@/gen/ingestion/v1/providers_pb";
import { PutSecretRequestSchema } from "@/gen/ingestion/v1/secrets_pb";

import { CreateConnectionActionType } from "@/pages/connectors/components/create/configure/actions";
import {
  CreateConnectionConfigureContext,
  type CreateConnectionConfigureContextShape,
} from "@/pages/connectors/components/create/configure/CreateConnectionConfigureProvider";
import { CreateConnectionPhase } from "@/pages/connectors/components/create/configure/types";
import {
  generateSecretRef,
  getConnectorConfigSchemaConnectionFields,
} from "@/pages/connectors/components/create/configure/utils";
import {
  createRequiredFieldsValidationErrorMap,
  validateRequiredFields,
} from "@/pages/connectors/components/create/configure/validation";

export function useCreateConnectionContext(): CreateConnectionConfigureContextShape {
  const context = useContext(CreateConnectionConfigureContext);
  if (!context) {
    throw new Error(
      "useCreateConnectionContext must be used within CreateConnectionConfigureProvider",
    );
  }
  return context;
}

interface UseCreateConnectionOptions {
  onSuccess?: (connectionId: string) => void;
}

export function useCreateConnection(options?: UseCreateConnectionOptions) {
  const { state, dispatch } = useCreateConnectionContext();
  const { showToast } = useToast();
  const queryClient = useQueryClient();

  const { mutateAsync: validateConfig } = useValidateConfigMutation();
  const { mutateAsync: putSecret } = usePutSecretMutation();
  const { mutateAsync: createConnection } = useCreateConnectionMutation();

  // Computed
  const isNameProvided = useMemo(
    () => (state.request.name ?? "").trim().length > 0,
    [state.request.name],
  );

  const isValidating = state.phase === CreateConnectionPhase.VALIDATING;
  const isCreating = state.phase === CreateConnectionPhase.CREATING;
  const isValidated = state.phase === CreateConnectionPhase.VALIDATED;
  const hasError = state.phase === CreateConnectionPhase.ERROR;
  const isDisabled = isValidating || isCreating;

  const nameError = useMemo(
    () =>
      state.shouldShowErrors && !isNameProvided ? "Name is required" : null,
    [state.shouldShowErrors, isNameProvided],
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

  // Test Connection
  const testConnection = useCallback(async () => {
    dispatch({
      type: CreateConnectionActionType.SET_SHOULD_SHOW_ERRORS,
      payload: true,
    });

    if (!state.request.name?.trim()) return;

    const fields = getConnectorConfigSchemaConnectionFields(state.connector);
    const clientErrors = validateRequiredFields(
      fields,
      state.request.config ?? {},
      state.request.secretRefs ?? {},
    );

    if (clientErrors.length > 0) {
      dispatch({
        type: CreateConnectionActionType.SET_VALIDATION_ERRORS,
        payload: clientErrors,
      });
      dispatch({
        type: CreateConnectionActionType.SET_STEP,
        payload: CreateConnectionPhase.ERROR,
      });
      return;
    }

    dispatch({
      type: CreateConnectionActionType.SET_STEP,
      payload: CreateConnectionPhase.VALIDATING,
    });
    dispatch({
      type: CreateConnectionActionType.SET_VALIDATION_ERRORS,
      payload: [],
    });

    await validateConfig(
      create(ValidateConfigRequestSchema, {
        kind: state.request.kind,
        connector: state.request.connector,
        config: { ...state.request.config, ...state.request.secretRefs },
        live: true,
      }),
      {
        onSuccess: (response) => {
          if (response.valid) {
            dispatch({
              type: CreateConnectionActionType.SET_STEP,
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
              type: CreateConnectionActionType.SET_STEP,
              payload: CreateConnectionPhase.ERROR,
            });
          }
        },
        onError: (error) => {
          const message =
            error instanceof Error ? error.message : "Validation failed";
          dispatch({
            type: CreateConnectionActionType.SET_ERROR,
            payload: message,
          });
          showToast({
            variant: ToastVariant.ERROR,
            header: "Failed to validate connection",
            subheader: message,
          });
        },
      },
    );
  }, [
    state.request,
    state.connector,
    state.request.secretRefs,
    dispatch,
    validateConfig,
    showToast,
  ]);

  // Create Connection
  const handleCreateConnection = useCallback(async () => {
    const connectionName = state.request.name?.trim();
    if (!connectionName) return;

    dispatch({
      type: CreateConnectionActionType.SET_STEP,
      payload: CreateConnectionPhase.CREATING,
    });

    try {
      const fields = getConnectorConfigSchemaConnectionFields(state.connector);
      const secretRefs: Record<string, string> = {};

      for (const field of fields) {
        if (field.type !== FieldType.SECRET) continue;

        const value = state.request.secretRefs?.[field.name]?.trim();
        if (!value) continue;

        const ref = generateSecretRef(connectionName, field.name);
        await putSecret(
          create(PutSecretRequestSchema, {
            ref,
            value: new TextEncoder().encode(value),
          }),
        );
        secretRefs[field.name] = ref;
      }

      const response = await createConnection(
        create(CreateConnectionRequestSchema, {
          ...state.request,
          name: connectionName,
          secretRefs,
        }),
      );

      void queryClient.invalidateQueries({
        queryKey: createListConnectionsQueryKey(),
      });
      showToast({
        variant: ToastVariant.SUCCESS,
        header: "Connection created",
        subheader: `${connectionName} has been created successfully.`,
      });
      if (response.connection?.id) {
        options?.onSuccess?.(response.connection.id);
      }
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "Creation failed";
      dispatch({
        type: CreateConnectionActionType.SET_ERROR,
        payload: message,
      });
      showToast({
        variant: ToastVariant.ERROR,
        header: "Failed to create connection",
        subheader: message,
      });
    }
  }, [
    state.request,
    state.connector,
    dispatch,
    putSecret,
    createConnection,
    queryClient,
    showToast,
    options,
  ]);

  return {
    state,
    dispatch,
    isDisabled,
    isValidating,
    isCreating,
    isValidated,
    hasError,
    isNameProvided,
    nameError,
    getFieldError,
    testConnection,
    createConnection: handleCreateConnection,
  };
}
