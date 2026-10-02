import { styled } from "@linaria/react";

import Accordion, { AccordionSize } from "@galaxy-io/dls/accordion/Accordion";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import { CreatePipelineModalActionType } from "@/pages/pipelines/components/create/actions";
import {
  useCreatePipelineModalDispatch,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import PipelineNodeConfigFields, {
  usePipelineNodeConfig,
} from "@/pages/pipelines/components/node/PipelineNodeConfigFields";

import { normalizeIdentifier } from "@/utils/naming";

// Accordion accepts a string header; decorate its text instead of using
// the right-aligned trailing slot.
const SettingsWrapper = withTheme(styled.div<PropsWithTheme<{ $isRequired: boolean }>>`
  > div > div:first-child button p:first-child::after {
    content: ${({ $isRequired }) => ($isRequired ? '" *"' : "none")};
    color: ${({ theme }) => theme.color.text.error};
  }
`);

interface CreatePipelineModalNodeConfigProps {
  header: string;
  connection: Connection;
  kind: ConnectorKind;
}

const CreatePipelineModalNodeConfig = ({
  header,
  connection,
  kind,
}: CreatePipelineModalNodeConfigProps) => {
  const { nodeConfigs, sourceConnection } = useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();
  const config = nodeConfigs[connection.id] ?? {};
  const defaultSchema =
    kind === ConnectorKind.SINK
      ? normalizeIdentifier(sourceConnection?.name ?? "") || undefined
      : undefined;
  const nodeConfig = usePipelineNodeConfig(connection, kind, config, defaultSchema);

  if (nodeConfig.fields.length === 0) return null;

  return (
    <SettingsWrapper $isRequired={nodeConfig.fields.some((field) => field.required)}>
      <Accordion
        header={header}
        padding="16px"
        size={AccordionSize.LARGE}
        isOpenInitial={nodeConfig.fields.some(
          (field) => field.required && !nodeConfig.displayValue[field.name],
        )}
      >
        <PipelineNodeConfigFields
          {...nodeConfig}
          config={config}
          onChange={(payload) =>
            dispatch({
              type: CreatePipelineModalActionType.SET_NODE_CONFIG,
              payload: { connectionId: connection.id, config: payload },
            })
          }
        />
      </Accordion>
    </SettingsWrapper>
  );
};

export default CreatePipelineModalNodeConfig;
