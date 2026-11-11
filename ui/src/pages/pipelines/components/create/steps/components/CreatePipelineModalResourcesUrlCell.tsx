import type { FC } from "react";

import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";

import { createWebhookDeliveryUrl } from "@/components/webhooks/utils";

import { useCreatePipelineModalState } from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import type { CreatePipelineModalResourceRow } from "@/pages/pipelines/components/create/types";

import { createGetWebhookEndpointInput, useGetWebhookEndpointQuery } from "@/api/queries/webhooks";

interface CreatePipelineModalResourcesUrlCellProps {
  row: CreatePipelineModalResourceRow;
}

const CreatePipelineModalResourcesUrlCell: FC<CreatePipelineModalResourcesUrlCellProps> = ({
  row,
}) => {
  const { sourceConnection } = useCreatePipelineModalState();
  const sourceConnectionId = sourceConnection?.id ?? "";
  const { data } = useGetWebhookEndpointQuery({
    input: createGetWebhookEndpointInput(sourceConnectionId),
    options: { enabled: sourceConnectionId !== "", retry: false },
  });

  return (
    <Text
      size={TextSize.BODY_SM}
      variant={TextVariant.SECONDARY}
      family={FontFamily.MONO}
      lineClamp={1}
    >
      {createWebhookDeliveryUrl(sourceConnectionId, row.name, data?.endpoint?.deliveryUrl)}
    </Text>
  );
};

export default CreatePipelineModalResourcesUrlCell;
