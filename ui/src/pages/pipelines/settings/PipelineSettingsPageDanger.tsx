import { create } from "@bufbuild/protobuf";
import { useNavigate } from "@tanstack/react-router";

import Modal from "@galaxy-io/dls/modal/Modal";

import { DeletePipelineRequestSchema, type Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import DangerZone from "@/components/DangerZone";
import DeleteConfirmDialog from "@/components/DeleteConfirmDialog";

import { formatPipelineName } from "@/pages/pipelines/utils";

import { useDeletePipelineMutation } from "@/api/queries/pipelines";

import { useDeleteConfirm } from "@/hooks/useDeleteConfirm";

interface PipelineSettingsPageDangerProps {
  pipeline: Pipeline;
}

const PipelineSettingsPageDanger = ({ pipeline }: PipelineSettingsPageDangerProps) => {
  const navigate = useNavigate();

  const { mutate: deletePipeline, isPending: isDeleting } = useDeletePipelineMutation();

  const { handleOpen, isOpen, handleClose, handleConfirm } = useDeleteConfirm({
    entityLabel: "Pipeline",
    entityName: formatPipelineName(pipeline),
    onDelete: ({ onSuccess, onError }) =>
      deletePipeline(create(DeletePipelineRequestSchema, { id: pipeline.id }), {
        onSuccess,
        onError,
      }),
    onDeleted: () => navigate({ to: "/pipelines" }),
  });

  return (
    <>
      <DangerZone
        title="Delete pipeline"
        description="This will permanently delete this pipeline and all of its data."
        onDelete={handleOpen}
      />
      <Modal open={isOpen} onClose={handleClose}>
        <DeleteConfirmDialog
          open={isOpen}
          onClose={handleClose}
          onConfirm={handleConfirm}
          title="Delete pipeline"
          body="This will permanently delete this pipeline and all associated data."
          confirmationPhrase={formatPipelineName(pipeline)}
          confirmLabel="Delete pipeline"
          isPending={isDeleting}
        />
      </Modal>
    </>
  );
};

export default PipelineSettingsPageDanger;
