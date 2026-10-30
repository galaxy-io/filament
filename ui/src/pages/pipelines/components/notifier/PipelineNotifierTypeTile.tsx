import type { FC } from "react";

import { WebhooksLogoIcon } from "@phosphor-icons/react";

import type { NotificationType } from "@/gen/ingestion/v1/notifiers_pb";

import ConnectorLogoTile from "@/components/connections/ConnectorLogoTile";
import { ConnectorTileSize } from "@/components/connections/types";

import {
  PIPELINE_NOTIFIER_TYPE_TO_LABEL_MAP,
  PIPELINE_NOTIFIER_TYPE_TO_LOGO_URLS_MAP,
} from "@/pages/pipelines/components/notifier/constants";

interface PipelineNotifierTypeTileProps {
  notificationType: NotificationType;
}

const PipelineNotifierTypeTile: FC<PipelineNotifierTypeTileProps> = ({ notificationType }) => (
  <ConnectorLogoTile
    name={PIPELINE_NOTIFIER_TYPE_TO_LABEL_MAP[notificationType]}
    darkLogoUrl={PIPELINE_NOTIFIER_TYPE_TO_LOGO_URLS_MAP[notificationType]?.dark}
    lightLogoUrl={PIPELINE_NOTIFIER_TYPE_TO_LOGO_URLS_MAP[notificationType]?.light}
    icon={WebhooksLogoIcon}
    size={ConnectorTileSize.SMALL}
  />
);

export default PipelineNotifierTypeTile;
