import { create } from "@bufbuild/protobuf";
import { useNavigate, useParams } from "@tanstack/react-router";

import { ButtonVariant } from "@galaxy-io/dls/buttons/Button";

import {
  DeletePipelineRequestSchema,
  GetPipelineRequestSchema,
  type Pipeline,
} from "@/gen/ingestion/v1/pipelines_pb";

import DangerZone from "@/components/DangerZone";
import Dialog, { DialogVariant } from "@/components/Dialog";

import { getPipelineCdcSourceConnections } from "@/pages/pipelines/settings/utils";
import { formatPipelineName } from "@/pages/pipelines/utils";

import { useSuspenseListConnectionsQuery } from "@/api/queries/connections";
import { useDeletePipelineMutation, useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";

import { useConfirm } from "@/hooks/useConfirm";

const PipelineSettingsPageDanger = () => {
  const navigate = useNavigate();
  const { id } = useParams({ from: "/_app/pipelines/$id" });

  const { data } = useSuspenseGetPipelineQuery({
    input: create(GetPipelineRequestSchema, { id, includeVersions: true }),
  });
  const pipeline = data.pipeline;
  const { data: connectionsData } = useSuspenseListConnectionsQuery();

  const { mutate: deletePipeline, isPending: isDeleting } = useDeletePipelineMutation();

  const { handleOpen, isOpen, target, handleClose, handleConfirm } = useConfirm<Pipeline>({
    entityLabel: "Pipeline",
    entityName: formatPipelineName,
    onConfirm: (_, { onSuccess, onError }) =>
      deletePipeline(create(DeletePipelineRequestSchema, { id }), {
        onSuccess,
        onError,
      }),
    onConfirmed: () => navigate({ to: "/pipelines" }),
  });

  if (!pipeline) return null;

  const cdcConnections = getPipelineCdcSourceConnections(pipeline, connectionsData.connections);
  const cdcConnectionNames = cdcConnections.map((connection) => `"${connection.name}"`).join(", ");
  const hasCdcSource = cdcConnections.length > 0;

  return (
    <>
      <DangerZone
        title="Delete pipeline"
        description="This will permanently delete this pipeline."
        onDelete={() => handleOpen(pipeline)}
      />
      <Dialog
        open={isOpen}
        onClose={handleClose}
        onConfirm={handleConfirm}
        title="Delete pipeline"
        description="This is a destructive action and cannot be undone."
        variant={hasCdcSource ? DialogVariant.WARNING : undefined}
        bodyTitle={hasCdcSource ? "Replication resources are not removed" : undefined}
        body={
          hasCdcSource
            ? `This pipeline streams changes from ${cdcConnectionNames} with CDC. Deleting it does not remove replication resources on the source database. Drop the replication slot after deleting or WAL will accumulate.`
            : "Are you sure you want to delete this pipeline?"
        }
        confirmationPhrase={target ? formatPipelineName(target) : undefined}
        confirmLabel="Delete pipeline"
        confirmVariant={ButtonVariant.ERROR}
        isPending={isDeleting}
      />
    </>
  );
};

export default PipelineSettingsPageDanger;
