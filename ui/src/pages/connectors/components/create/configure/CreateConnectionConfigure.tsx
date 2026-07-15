import { useCallback } from "react";

import { styled } from "@linaria/react";
import { ArrowRightIcon, CheckIcon } from "@phosphor-icons/react";
import { useNavigate } from "@tanstack/react-router";
import { match } from "ts-pattern";

import type { JsonValue } from "@bufbuild/protobuf";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import type { ConnectorSpec } from "@/gen/ingestion/v1/providers_pb";

import CreateConnectionModalHeader from "@/pages/connectors/components/create/components/CreateConnectionModalHeader";
import CreateConnectionModalWrapper from "@/pages/connectors/components/create/components/CreateConnectionModalWrapper";
import CreateConnectionNameInput from "@/pages/connectors/components/create/components/CreateConnectionNameInput";
import { CreateConnectionStep } from "@/pages/connectors/components/create/types";

import { CreateConnectionConfigureActionType } from "./actions";
import CreateConnectionConfigureProvider from "./CreateConnectionConfigureProvider";
import DynamicField from "./DynamicField";
import { useCreateConnectionConfigure } from "./hooks";
import { CreateConnectionPhase } from "./types";
import { getConnectionScopedFields, getFieldValue } from "./utils";
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
  justify-content: flex-end;
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

  const handleSuccess = useCallback(
    (connectionId: string) => {
      void navigate({
        to: ".",
        search: (prev) => ({
          ...prev,
          flow: undefined,
          connectionId,
        }),
      });
    },
    [navigate],
  );

  const {
    state,
    dispatch,
    testConnection,
    createConnection,
    isDisabled,
    nameError,
    isValidating,
    isCreating,
  } = useCreateConnectionConfigure({ onSuccess: handleSuccess });

  const fields = getConnectionScopedFields(connector);

  const handleConnectionNameChange = useCallback(
    (name: string) => {
      dispatch({
        type: CreateConnectionConfigureActionType.SET_CONNECTION_NAME,
        payload: name,
      });
    },
    [dispatch],
  );

  const handleFieldChange = useCallback(
    (fieldName: string, value: JsonValue, fieldType: number) => {
      dispatch({
        type: CreateConnectionConfigureActionType.SET_FIELD_VALUE,
        payload: { field: fieldName, value, fieldType },
      });
    },
    [dispatch],
  );

  const handleTest = useCallback(() => {
    void testConnection();
  }, [testConnection]);

  const handleCreate = useCallback(() => {
    void createConnection();
  }, [createConnection]);

  const getFieldError = (fieldName: string): string | undefined => {
    if (!state.shouldShowErrors) return undefined;
    const error = state.validationErrors.find((e) => e.field === fieldName);
    return error?.message;
  };

  const renderBody = () => {
    return (
      <>
        <CreateConnectionNameInput
          value={state.connectionName}
          onChange={handleConnectionNameChange}
          error={nameError}
          isDisabled={isDisabled}
        />

        {fields.map((field) => (
          <DynamicField
            key={field.name}
            field={field}
            value={getFieldValue(field, state.formValues, state.secretValues)}
            onChange={(value) => handleFieldChange(field.name, value, field.type)}
            error={getFieldError(field.name)}
            isDisabled={isDisabled}
          />
        ))}
      </>
    );
  };

  const renderFooter = () => {
    return match(state.phase)
      .with(CreateConnectionPhase.CONFIGURE, CreateConnectionPhase.ERROR, () => (
        <Button
          label="Test Connection"
          icon={ArrowRightIcon}
          size={ButtonSize.LARGE}
          onClick={handleTest}
          isDisabled={isDisabled}
        />
      ))
      .with(CreateConnectionPhase.VALIDATING, () => (
        <Button
          label="Testing..."
          size={ButtonSize.LARGE}
          onClick={NOOP}
          isLoading={isValidating}
          isDisabled
        />
      ))
      .with(CreateConnectionPhase.VALIDATED, () => (
        <Button
          label="Create Connection"
          icon={CheckIcon}
          variant={ButtonVariant.SUCCESS}
          size={ButtonSize.LARGE}
          onClick={handleCreate}
        />
      ))
      .with(CreateConnectionPhase.CREATING, () => (
        <Button
          label="Creating..."
          size={ButtonSize.LARGE}
          onClick={NOOP}
          isLoading={isCreating}
          isDisabled
        />
      ))
      .exhaustive();
  };

  return (
    <CreateConnectionModalWrapper step={CreateConnectionStep.CONFIGURE}>
      <FlexItem grow={0} shrink={0}>
        <CreateConnectionModalHeader connector={connector} onClose={onClose} onBack={onBack} />
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
      <FlexItem grow={0} shrink={0}>
        <FooterWrapper>{renderFooter()}</FooterWrapper>
      </FlexItem>
    </CreateConnectionModalWrapper>
  );
};

const CreateConnectionConfigure = (props: CreateConnectionConfigureProps) => (
  <CreateConnectionConfigureProvider connector={props.connector}>
    <CreateConnectionConfigureContent {...props} />
  </CreateConnectionConfigureProvider>
);

export default CreateConnectionConfigure;
