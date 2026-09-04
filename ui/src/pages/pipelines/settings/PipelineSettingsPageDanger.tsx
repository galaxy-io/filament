import { create } from "@bufbuild/protobuf";
import { useNavigate, useParams } from "@tanstack/react-router";

import { ButtonVariant } from "@galaxy-io/dls/buttons/Button";

import {
  DeletePipelineRequestSchema,
  GetPipelineRequestSchema,
  type Pipeline,
} from "@/gen/ingestion/v1/pipelines_pb";

import DangerZone from "@/components/DangerZone";
import Dialog from "@/components/Dialog";

import { formatPipelineName } from "@/pages/pipelines/utils";

import { useDeletePipelineMutation, useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";

import { useConfirm } from "@/hooks/useConfirm";

const PipelineSettingsPageDanger = () => {
  const navigate = useNavigate();
  const { id } = useParams({ from: "/_app/pipelines/$id" });

  const { data } = useSuspenseGetPipelineQuery({
    input: create(GetPipelineRequestSchema, { id }),
  });
  const pipeline = data.pipeline;

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
        body="Are you sure you want to delete this pipeline? This is a destructive action and cannot be undone."
        confirmationPhrase={target ? formatPipelineName(target) : undefined}
        confirmLabel="Delete pipeline"
        confirmVariant={ButtonVariant.ERROR}
        isPending={isDeleting}
      />
    </>
  );
};

export default PipelineSettingsPageDanger;
