import { create } from "@bufbuild/protobuf";
import { useNavigate, useParams } from "@tanstack/react-router";

import Modal from "@galaxy-io/dls/modal/Modal";

import {
  DeletePipelineRequestSchema,
  GetPipelineRequestSchema,
} from "@/gen/ingestion/v1/pipelines_pb";

import DangerZone from "@/components/DangerZone";
import DeleteConfirmDialog from "@/components/DeleteConfirmDialog";

import { formatPipelineName } from "@/pages/pipelines/utils";

import { useDeletePipelineMutation, useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";

import { useDeleteConfirm } from "@/hooks/useDeleteConfirm";

const PipelineSettingsPageDanger = () => {
  const navigate = useNavigate();
  const { id } = useParams({ from: "/pipelines/$id" });

  const { data } = useSuspenseGetPipelineQuery({
    input: create(GetPipelineRequestSchema, { id }),
  });
  const pipeline = data.pipeline;

  const { mutate: deletePipeline, isPending: isDeleting } = useDeletePipelineMutation();

  const { handleOpen, isOpen, handleClose, handleConfirm } = useDeleteConfirm({
    entityLabel: "Pipeline",
    entityName: pipeline ? formatPipelineName(pipeline) : "",
    onDelete: ({ onSuccess, onError }) =>
      deletePipeline(create(DeletePipelineRequestSchema, { id }), {
        onSuccess,
        onError,
      }),
    onDeleted: () => navigate({ to: "/pipelines" }),
  });

  if (!pipeline) return null;

  return (
    <>
      <DangerZone
        title="Delete pipeline"
        description="This will permanently delete this pipeline."
        onDelete={handleOpen}
      />
      <Modal open={isOpen} onClose={handleClose}>
        <DeleteConfirmDialog
          open={isOpen}
          onClose={handleClose}
          onConfirm={handleConfirm}
          title="Delete pipeline"
          body="Are you sure you want to delete this pipeline? This is a destructive action and cannot be undone."
          confirmationPhrase={formatPipelineName(pipeline)}
          confirmLabel="Delete pipeline"
          isPending={isDeleting}
        />
      </Modal>
    </>
  );
};

export default PipelineSettingsPageDanger;
