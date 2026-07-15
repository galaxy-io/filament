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

import { CreateConnectionConfigureActionType } from "./actions";
import {
  CREATE_CONNECTION_FAILED_TO_CREATE_ERROR_HEADER,
  CREATE_CONNECTION_FAILED_TO_VALIDATE_ERROR_HEADER,
  CREATE_CONNECTION_UNEXPECTED_ERROR,
} from "./constants";
import {
  CreateConnectionConfigureContext,
  type CreateConnectionConfigureContextShape,
} from "./CreateConnectionConfigureProvider";
import { CreateConnectionPhase } from "./types";
import {
  buildConfigObject,
  generateSecretRef,
  getConnectionNameError,
  getConnectionScopedFields,
  isConnectionNameProvided,
  validateRequiredFields,
} from "./utils";

export function useCreateConnectionConfigureContext(): CreateConnectionConfigureContextShape {
  const context = useContext(CreateConnectionConfigureContext);
  if (!context) {
    throw new Error(
      "useCreateConnectionConfigureContext must be used within a CreateConnectionConfigureProvider",
    );
  }
  return context;
}

interface UseCreateConnectionConfigureOptions {
  onSuccess?: (connectionId: string) => void;
}

export function useCreateConnectionConfigure({ onSuccess }: UseCreateConnectionConfigureOptions) {
  const { state, dispatch } = useCreateConnectionConfigureContext();
  const queryClient = useQueryClient();
  const { showToast } = useToast();

  const { mutateAsync: validateConfigMutation, isPending: isValidating } =
    useValidateConfigMutation();
  const { mutateAsync: putSecretMutation } = usePutSecretMutation();
  const { mutateAsync: createConnectionMutation, isPending: isCreating } =
    useCreateConnectionMutation();

  const nameError = useMemo(() => getConnectionNameError(state), [state]);

  const isDisabled = useMemo(() => {
    return (
      state.phase === CreateConnectionPhase.VALIDATING ||
      state.phase === CreateConnectionPhase.CREATING
    );
  }, [state.phase]);

  const testConnection = useCallback(async () => {
    if (!state.connector) return;

    dispatch({ type: CreateConnectionConfigureActionType.SET_SHOULD_SHOW_ERRORS, payload: true });

    if (!isConnectionNameProvided(state)) {
      return;
    }

    const fields = getConnectionScopedFields(state.connector);
    const clientErrors = validateRequiredFields(fields, state.formValues, state.secretValues);

    if (clientErrors.length > 0) {
      dispatch({
        type: CreateConnectionConfigureActionType.SET_VALIDATION_ERRORS,
        payload: clientErrors.map((e) => ({
          field: e.field,
          message: e.message,
          $typeName: "ingestion.v1.ValidationError" as const,
        })),
      });
      dispatch({
        type: CreateConnectionConfigureActionType.SET_PHASE,
        payload: CreateConnectionPhase.ERROR,
      });
      return;
    }

    try {
      dispatch({
        type: CreateConnectionConfigureActionType.SET_PHASE,
        payload: CreateConnectionPhase.VALIDATING,
      });
      dispatch({
        type: CreateConnectionConfigureActionType.SET_VALIDATION_ERRORS,
        payload: [],
      });

      const config = buildConfigObject(fields, state.formValues, state.secretValues);

      const response = await validateConfigMutation(
        create(ValidateConfigRequestSchema, {
          kind: state.connector.kind,
          connector: state.connector.name,
          config,
          live: true,
        }),
      );

      if (response.valid) {
        dispatch({
          type: CreateConnectionConfigureActionType.SET_PHASE,
          payload: CreateConnectionPhase.VALIDATED,
        });
        showToast({
          variant: ToastVariant.SUCCESS,
          header: "Connection validated",
          subheader: "Your connection settings are valid.",
        });
      } else {
        dispatch({
          type: CreateConnectionConfigureActionType.SET_VALIDATION_ERRORS,
          payload: response.errors,
        });
        dispatch({
          type: CreateConnectionConfigureActionType.SET_PHASE,
          payload: CreateConnectionPhase.ERROR,
        });
      }
    } catch (error) {
      const errorMessage =
        error instanceof Error ? error.message : CREATE_CONNECTION_UNEXPECTED_ERROR;
      dispatch({ type: CreateConnectionConfigureActionType.SET_ERROR, payload: errorMessage });
      showToast({
        variant: ToastVariant.ERROR,
        header: CREATE_CONNECTION_FAILED_TO_VALIDATE_ERROR_HEADER,
        subheader: errorMessage,
      });
    }
  }, [state, dispatch, validateConfigMutation, showToast]);

  const createConnection = useCallback(async () => {
    if (!state.connector || !state.connectionName.trim()) return;

    try {
      dispatch({
        type: CreateConnectionConfigureActionType.SET_PHASE,
        payload: CreateConnectionPhase.CREATING,
      });

      const fields = getConnectionScopedFields(state.connector);
      const secretRefs: Record<string, string> = {};

      // Store secrets and collect refs
      for (const field of fields) {
        if (field.type === FieldType.SECRET) {
          const secretValue = state.secretValues[field.name];
          if (secretValue && secretValue.trim() !== "") {
            const secretRef = generateSecretRef(state.connectionName, field.name);
            await putSecretMutation(
              create(PutSecretRequestSchema, {
                ref: secretRef,
                value: new TextEncoder().encode(secretValue),
                meta: {},
              }),
            );
            secretRefs[field.name] = secretRef;
          }
        }
      }

      // Build config without secrets
      const config = buildConfigObject(
        fields.filter((f) => f.type !== FieldType.SECRET),
        state.formValues,
        {},
      );

      const response = await createConnectionMutation(
        create(CreateConnectionRequestSchema, {
          kind: state.connector.kind,
          name: state.connectionName.trim(),
          connector: state.connector.name,
          config,
          secretRefs,
        }),
      );

      void queryClient.invalidateQueries({ queryKey: createListConnectionsQueryKey() });

      showToast({
        variant: ToastVariant.SUCCESS,
        header: "Connection created",
        subheader: `${state.connectionName} has been created successfully.`,
      });

      const connectionId = response.connection?.id;
      if (connectionId && onSuccess) {
        onSuccess(connectionId);
      }
    } catch (error) {
      const errorMessage =
        error instanceof Error ? error.message : CREATE_CONNECTION_UNEXPECTED_ERROR;
      dispatch({ type: CreateConnectionConfigureActionType.SET_ERROR, payload: errorMessage });
      showToast({
        variant: ToastVariant.ERROR,
        header: CREATE_CONNECTION_FAILED_TO_CREATE_ERROR_HEADER,
        subheader: errorMessage,
      });
    }
  }, [
    state,
    dispatch,
    putSecretMutation,
    createConnectionMutation,
    queryClient,
    showToast,
    onSuccess,
  ]);

  return {
    state,
    dispatch,
    testConnection,
    createConnection,
    isDisabled,
    nameError,
    isValidating,
    isCreating,
  };
}
