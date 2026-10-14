import type { FC } from "react";

import PipelinePageEditActions from "@/pages/pipelines/components/header/PipelinePageEditActions";
import PipelinePageRunActions from "@/pages/pipelines/components/header/PipelinePageRunActions";
import { usePipelineHasUnsavedChanges } from "@/pages/pipelines/hooks/usePipelineHasUnsavedChanges";
import { usePipelinePreviewVersion } from "@/pages/pipelines/hooks/usePipelinePreviewVersion";

const PipelinePageActions: FC = () => {
  const isPreview = usePipelinePreviewVersion() !== undefined;
  const hasUnsavedChanges = usePipelineHasUnsavedChanges();

  if (isPreview) {
    return null;
  }

  return hasUnsavedChanges ? <PipelinePageEditActions /> : <PipelinePageRunActions />;
};

export default PipelinePageActions;
