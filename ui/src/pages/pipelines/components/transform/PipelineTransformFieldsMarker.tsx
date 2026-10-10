import type { FC } from "react";

import { styled } from "@linaria/react";
import { FunctionIcon } from "@phosphor-icons/react";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import { TRANSFORM_MARKER_SIZE } from "@/pages/pipelines/components/transform/constants";

const MarkerSlot = styled.span`
  display: inline-flex;
  flex-shrink: 0;

  color: ${t.color.text.blue};
`;

interface PipelineTransformFieldsMarkerProps {
  isInvalid?: boolean;
}

const PipelineTransformFieldsMarker: FC<PipelineTransformFieldsMarkerProps> = ({
  isInvalid = false,
}) => (
  <MarkerSlot>
    <Icon
      component={FunctionIcon}
      size={TRANSFORM_MARKER_SIZE}
      variant={isInvalid ? IconVariant.ERROR : IconVariant.INHERIT}
    />
  </MarkerSlot>
);

export default PipelineTransformFieldsMarker;
