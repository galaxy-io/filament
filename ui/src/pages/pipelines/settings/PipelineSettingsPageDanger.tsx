import { create } from "@bufbuild/protobuf";
import { TrashIcon } from "@phosphor-icons/react";
import { useNavigate, useParams } from "@tanstack/react-router";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import Widget from "@galaxy-io/dls/widget/Widget";

import {
  DeletePipelineRequestSchema,
  GetPipelineRequestSchema,
  type Pipeline,
} from "@/gen/ingestion/v1/pipelines_pb";

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
      <Widget>
        <Flex justifyContent={JustifyContent.SPACE_BETWEEN} alignItems={AlignItems.CENTER} gap={12}>
          <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={4}>
            <Text weight={TextWeight.MEDIUM}>Delete pipeline</Text>
            <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
              This will permanently delete this pipeline.
            </Text>
          </Flex>
          <Button
            label="Delete"
            icon={TrashIcon}
            variant={ButtonVariant.ERROR}
            onClick={() => handleOpen(pipeline)}
          />
        </Flex>
      </Widget>
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
