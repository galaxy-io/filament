import { Fragment } from "react";

import Accordion, { AccordionSize } from "@galaxy-io/dls/accordion/Accordion";
import FlexWrapper, { FlexDirection, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Text, { TextWeight } from "@galaxy-io/dls/text/Text";
import Widget from "@galaxy-io/dls/widget/Widget";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import { CreatePipelineModalActionType } from "@/pages/pipelines/components/create/actions";
import {
  useCreatePipelineModalDispatch,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import CreatePipelineModalDeliverySink from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliverySink";
import PipelineNodeConfigFields, {
  usePipelineNodeConfig,
} from "@/pages/pipelines/components/node/PipelineNodeConfigFields";
import PipelineScheduleFields from "@/pages/pipelines/components/schedule/PipelineScheduleFields";
import PipelineWorkerConfigurationEditor from "@/pages/pipelines/components/worker/PipelineWorkerConfigurationEditor";

import { normalizeIdentifier } from "@/utils/naming";

interface CreatePipelineModalDeliverySectionProps {
  header: string;
  children: React.ReactNode;
}

const CreatePipelineModalDeliverySection = ({
  header,
  children,
}: CreatePipelineModalDeliverySectionProps) => (
  <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.MEDIUM} fillWidth>
    <FlexWrapper direction={FlexDirection.COLUMN} gap={2} fillWidth>
      <Text weight={TextWeight.MEDIUM}>{header}</Text>
    </FlexWrapper>
    {children}
  </FlexWrapper>
);

const CreatePipelineModalDeliverySchedule = () => {
  const { schedule } = useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();

  return (
    <PipelineScheduleFields
      state={schedule}
      onChange={(partial) =>
        dispatch({
          type: CreatePipelineModalActionType.SET_SCHEDULE,
          payload: partial,
        })
      }
    />
  );
};

const CreatePipelineModalDeliveryNodeConfig = ({
  header,
  connection,
  kind,
}: {
  header: string;
  connection: Connection;
  kind: ConnectorKind;
}) => {
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
    <Accordion header={header} padding="16px" size={AccordionSize.LARGE}>
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
  );
};

const CreatePipelineModalDeliveryAdvanced = () => {
  const { sourceConnection, sinks, workerConfiguration, workerConfigurationError } =
    useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();

  return (
    <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.MEDIUM} fillWidth>
      {sourceConnection && (
        <CreatePipelineModalDeliveryNodeConfig
          header="Source configuration"
          connection={sourceConnection}
          kind={ConnectorKind.SOURCE}
        />
      )}
      {sinks.map((sink) => (
        <CreatePipelineModalDeliveryNodeConfig
          key={sink.connection.id}
          header="Sink configuration"
          connection={sink.connection}
          kind={ConnectorKind.SINK}
        />
      ))}
      <Accordion header="Worker configuration" padding="16px" size={AccordionSize.LARGE}>
        <PipelineWorkerConfigurationEditor
          value={workerConfiguration}
          error={workerConfigurationError}
          onChange={(payload) =>
            dispatch({
              type: CreatePipelineModalActionType.SET_WORKER_CONFIGURATION,
              payload,
            })
          }
        />
      </Accordion>
    </FlexWrapper>
  );
};

const CreatePipelineModalDelivery = () => {
  const { sinks } = useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();

  return (
    <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.LARGE} fillWidth>
      {sinks.length > 0 && (
        <CreatePipelineModalDeliverySection header="Destinations">
          <Widget noPadding noHover fillWidth>
            <FlexWrapper direction={FlexDirection.COLUMN} fillWidth>
              {sinks.map((sink, index) => (
                <Fragment key={sink.connection.id}>
                  {index > 0 && <HorizontalDivider />}
                  <CreatePipelineModalDeliverySink
                    sink={sink}
                    onChange={(sinkId, writeMode) =>
                      dispatch({
                        type: CreatePipelineModalActionType.SET_SINK_WRITE_MODE,
                        payload: { sinkId, writeMode },
                      })
                    }
                  />
                </Fragment>
              ))}
            </FlexWrapper>
          </Widget>
        </CreatePipelineModalDeliverySection>
      )}
      <CreatePipelineModalDeliverySection header="Schedule">
        <CreatePipelineModalDeliverySchedule />
      </CreatePipelineModalDeliverySection>
      <CreatePipelineModalDeliverySection header="Advanced">
        <CreatePipelineModalDeliveryAdvanced />
      </CreatePipelineModalDeliverySection>
    </FlexWrapper>
  );
};

export default CreatePipelineModalDelivery;
