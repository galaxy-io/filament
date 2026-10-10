import type { FC } from "react";

import { create } from "@bufbuild/protobuf";
import { TrashIcon } from "@phosphor-icons/react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Alert, { AlertVariant } from "@galaxy-io/dls/feedback/Alert";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import ConfirmDialog from "@galaxy-io/dls/modal/ConfirmDialog";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import Widget from "@galaxy-io/dls/widget/Widget";

import { DeletePipelineRequestSchema, type Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import { formatPipelineName, isPipelineNameMatch } from "@/components/pipelines/utils";

import { getPipelineCdcSourceConnections } from "@/pages/pipelines/settings/utils";

import { useFilamentNavigate, usePipelineParams } from "@/module/hooks";
import { FilamentPath } from "@/module/paths";

import { useSuspenseListConnectionsQuery } from "@/api/queries/connections";
import {
  createGetPipelineInput,
  useDeletePipelineMutation,
  useSuspenseGetPipelineQuery,
} from "@/api/queries/pipelines";

import { useConfirm } from "@/hooks/useConfirm";

const PipelineSettingsPageDanger: FC = () => {
  const navigate = useFilamentNavigate();
  const { id } = usePipelineParams();

  const { data } = useSuspenseGetPipelineQuery({
    input: createGetPipelineInput(id),
  });
  const pipeline = data.pipeline;
  const { data: connectionsData } = useSuspenseListConnectionsQuery();

  const { mutate: deletePipeline } = useDeletePipelineMutation();

  const { handleOpen, isOpen, target, handleClose, handleConfirm } = useConfirm<Pipeline>({
    entityLabel: "Pipeline",
    entityName: formatPipelineName,
    onConfirm: (_, { onSuccess, onError }) =>
      deletePipeline(create(DeletePipelineRequestSchema, { id }), {
        onSuccess,
        onError,
      }),
    onConfirmed: () => navigate({ to: FilamentPath.PIPELINES }),
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
      <ConfirmDialog
        isOpen={isOpen}
        onOpenChange={(open) => {
          if (!open) handleClose();
        }}
        onConfirm={handleConfirm}
        header="Delete pipeline?"
        description="This is a destructive action and cannot be undone."
        confirmValue={target ? formatPipelineName(target) : undefined}
        isMatch={isPipelineNameMatch}
        label="Delete pipeline"
        isDestructive
      >
        {hasCdcSource && (
          <Alert variant={AlertVariant.WARNING} header="Replication resources are not removed">
            {`This pipeline streams changes from ${cdcConnectionNames} with CDC. Deleting it does not remove replication resources on the source database. Drop the replication slot after deleting or WAL will accumulate.`}
          </Alert>
        )}
      </ConfirmDialog>
    </>
  );
};

export default PipelineSettingsPageDanger;
