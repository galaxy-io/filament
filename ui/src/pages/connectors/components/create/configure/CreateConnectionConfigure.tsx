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

import { FieldType } from "@/gen/ingestion/v1/common_pb";
import type { ConnectorSpec } from "@/gen/ingestion/v1/providers_pb";

import CreateConnectionModalHeader from "@/pages/connectors/components/create/components/CreateConnectionModalHeader";
import CreateConnectionModalWrapper from "@/pages/connectors/components/create/components/CreateConnectionModalWrapper";
import CreateConnectionNameInput from "@/pages/connectors/components/create/components/CreateConnectionNameInput";
import { CreateConnectionModalStep } from "@/pages/connectors/components/create/types";

import { NOOP } from "@/constants";

import { CreateConnectionActionType } from "@/pages/connectors/components/create/configure/actions";
import CreateConnectionConfigureProvider from "@/pages/connectors/components/create/configure/CreateConnectionConfigureProvider";
import CreateConnectionField from "@/pages/connectors/components/create/configure/fields/CreateConnectionField";
import { useCreateConnection } from "@/pages/connectors/components/create/configure/hooks";
import { CreateConnectionPhase } from "@/pages/connectors/components/create/configure/types";
import { getConnectorConfigSchemaConnectionFields } from "@/pages/connectors/components/create/configure/utils";

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
    isDisabled,
    isValidating,
    isCreating,
    nameError,
    getFieldError,
    testConnection,
    createConnection,
  } = useCreateConnection({ onSuccess: handleSuccess });

  const fields = getConnectorConfigSchemaConnectionFields(connector);

  const handleNameChange = useCallback(
    (name: string) => {
      dispatch({
        type: CreateConnectionActionType.SET_REQUEST_NAME,
        payload: name,
      });
    },
    [dispatch],
  );

  const handleFieldChange = useCallback(
    (fieldName: string, value: JsonValue, fieldType: FieldType) => {
      if (fieldType === FieldType.SECRET) {
        dispatch({
          type: CreateConnectionActionType.SET_SECRET_VALUE,
          payload: { field: fieldName, value: value as string },
        });
      } else {
        dispatch({
          type: CreateConnectionActionType.SET_REQUEST_CONFIG_FIELD,
          payload: { field: fieldName, value },
        });
      }
    },
    [dispatch],
  );

  const handleTest = useCallback(() => {
    void testConnection();
  }, [testConnection]);

  const handleCreate = useCallback(() => {
    void createConnection();
  }, [createConnection]);

  const getFieldValue = (fieldName: string, fieldType: FieldType): JsonValue => {
    return fieldType === FieldType.SECRET
      ? (state.request.secretRefs?.[fieldName] ?? null)
      : (state.request.config?.[fieldName] ?? null);
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
            value={getFieldValue(field.name, field.type)}
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
      .with(CreateConnectionPhase.IDLE, CreateConnectionPhase.ERROR, () => (
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
    <CreateConnectionModalWrapper step={CreateConnectionModalStep.CONFIGURE}>
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
