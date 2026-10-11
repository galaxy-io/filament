import type { FC } from "react";

import { ArrowUUpLeftIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Alert, { AlertVariant } from "@galaxy-io/dls/feedback/Alert";

import { usePipelineCanvasNavigate } from "@/pages/pipelines/hooks/usePipelineCanvasNavigate";
import { usePipelinePreviewVersion } from "@/pages/pipelines/hooks/usePipelinePreviewVersion";

const PipelinePagePreviewBanner: FC = () => {
  const previewed = usePipelinePreviewVersion();
  const navigateCanvas = usePipelineCanvasNavigate();

  return (
    <Alert
      isBanner
      variant={AlertVariant.WARNING}
      header={`Viewing version ${previewed?.version}`}
      actions={
        <Button
          label="Back to latest"
          icon={ArrowUUpLeftIcon}
          size={ButtonSize.SMALL}
          variant={ButtonVariant.SECONDARY}
          onClick={() => navigateCanvas({ version: undefined })}
        />
      }
    >
      This version is read-only. Go back to the latest version to make changes.
    </Alert>
  );
};

export default PipelinePagePreviewBanner;
