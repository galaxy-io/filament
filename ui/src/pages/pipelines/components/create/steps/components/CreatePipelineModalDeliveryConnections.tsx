import { type FC, Fragment } from "react";

import Divider from "@galaxy-io/dls/layout/Divider";
import Widget from "@galaxy-io/dls/widget/Widget";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import { useCreatePipelineModalState } from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import CreatePipelineModalDeliveryConnection from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliveryConnection";

const CreatePipelineModalDeliveryConnections: FC = () => {
  const { sourceConnection, sinks } = useCreatePipelineModalState();

  return (
    <Widget header="Connections" isFlush gap={0}>
      {sourceConnection && (
        <CreatePipelineModalDeliveryConnection
          connection={sourceConnection}
          kind={ConnectorKind.SOURCE}
        />
      )}
      {sinks.map((sink) => (
        <Fragment key={sink.connection.id}>
          <Divider />
          <CreatePipelineModalDeliveryConnection
            connection={sink.connection}
            kind={ConnectorKind.SINK}
            sink={sink}
          />
        </Fragment>
      ))}
    </Widget>
  );
};

export default CreatePipelineModalDeliveryConnections;
