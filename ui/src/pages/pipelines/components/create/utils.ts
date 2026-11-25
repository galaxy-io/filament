import { match } from "ts-pattern";

import { ConnectorKind, ExecutionMode } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import { CreatePipelineModalActionType } from "@/pages/pipelines/components/create/actions";
import { CREATE_PIPELINE_MODAL_DEFAULT_CRON } from "@/pages/pipelines/components/create/constants";
import createPipelineModalReducer from "@/pages/pipelines/components/create/reducer";
import {
  type CreatePipelineModalSeed,
  type CreatePipelineModalSeedSearch,
  type CreatePipelineModalState,
  CreatePipelineModalStep,
} from "@/pages/pipelines/components/create/types";
import { PIPELINE_SCHEDULE_DEFAULT_STATE } from "@/pages/pipelines/components/schedule/constants";
import { PIPELINE_WORKER_CONFIGURATION_DEFAULT_TEXT } from "@/pages/pipelines/components/worker/constants";

export const createInitialCreatePipelineModalState = (): CreatePipelineModalState => ({
  executionMode: ExecutionMode.BOUNDED,
  manualResources: [],
  step: CreatePipelineModalStep.CONNECTIONS,
  activeSinkId: "",
  sourceConnection: null,
  sinkConnections: [],
  resourceSelection: {},
  resourceReadModes: {},
  resourceCursors: {},
  resourceColumns: {},
  resourcePartitions: {},
  resourceWriteModes: {},
  resourceSinkConfigs: {},
  sinkWriteModes: {},
  nodeConfigs: {},
  name: "",
  isNameTouched: false,
  description: "",
  schedule: {
    ...PIPELINE_SCHEDULE_DEFAULT_STATE,
    cron: CREATE_PIPELINE_MODAL_DEFAULT_CRON,
  },
  notifiers: [],
  workerConfiguration: PIPELINE_WORKER_CONFIGURATION_DEFAULT_TEXT,
  failOnError: false,
  isSubmitting: false,
});

export const isCreatePipelineModalConnectionSelectable = (
  connection: Connection,
  executionMode: ExecutionMode,
): boolean =>
  match(connection.kind)
    .with(ConnectorKind.SOURCE, () => connection.executionModes.length > 0)
    .with(ConnectorKind.SINK, () => connection.executionModes.includes(executionMode))
    .otherwise(() => false);

export const getCreatePipelineModalSeedIds = ({
  sourceId,
  sinkIds = [],
}: CreatePipelineModalSeedSearch): Connection["id"][] => [
  ...new Set(sourceId ? [sourceId, ...sinkIds] : sinkIds),
];

export const mapConnectionsToCreatePipelineModalSeed = (
  { sourceId, sinkIds = [] }: CreatePipelineModalSeedSearch,
  connections: Connection[],
): CreatePipelineModalSeed => {
  const liveConnections = connections.filter((connection) => !connection.deletedAt);
  return {
    sourceConnection:
      liveConnections.find(
        (connection) => connection.id === sourceId && connection.kind === ConnectorKind.SOURCE,
      ) ?? null,
    sinkConnections: liveConnections.filter(
      (connection) => sinkIds.includes(connection.id) && connection.kind === ConnectorKind.SINK,
    ),
  };
};

export const createSeededCreatePipelineModalState = ({
  sourceConnection,
  sinkConnections,
}: CreatePipelineModalSeed): CreatePipelineModalState => {
  const initialState = createInitialCreatePipelineModalState();
  const sourceState =
    sourceConnection &&
    isCreatePipelineModalConnectionSelectable(sourceConnection, initialState.executionMode)
      ? createPipelineModalReducer(initialState, {
          type: CreatePipelineModalActionType.SELECT_SOURCE,
          payload: sourceConnection,
        })
      : initialState;
  return sinkConnections
    .filter((sinkConnection) =>
      isCreatePipelineModalConnectionSelectable(sinkConnection, sourceState.executionMode),
    )
    .reduce(
      (state, sinkConnection) =>
        createPipelineModalReducer(state, {
          type: CreatePipelineModalActionType.TOGGLE_SINK,
          payload: sinkConnection,
        }),
      sourceState,
    );
};

export const mapCreatePipelineModalSeedToSearch = ({
  sourceConnection,
  sinkConnections,
}: CreatePipelineModalSeed): CreatePipelineModalSeedSearch => ({
  sourceId: sourceConnection?.id,
  sinkIds: sinkConnections.length ? sinkConnections.map((sink) => sink.id) : undefined,
});
