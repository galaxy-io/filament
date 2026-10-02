import { Fragment } from "react";

import FlexWrapper, {
  AlignItems,
  FlexDirection,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";

import {
  TRANSFORM_ACTION,
  TRANSFORM_HEADER_PADDING_X,
  TRANSFORM_HEADER_PADDING_Y,
} from "@/pages/pipelines/components/transform/constants";
import PipelineTransformFieldsRow, {
  PipelineTransformFieldsRowVariant,
} from "@/pages/pipelines/components/transform/PipelineTransformFieldsRow";

const PENDING_ROW_WIDTHS = ["60%", "45%"];
const PENDING_HANDLE_SIZE = 16;
const PENDING_LINE_HEIGHT = 14;

const PipelineTransformFieldsPending = () => (
  <FlexWrapper direction={FlexDirection.COLUMN} fillWidth>
    {PENDING_ROW_WIDTHS.map((width, index) => (
      <Fragment key={width}>
        {index > 0 && <HorizontalDivider />}
        <FlexWrapper
          padding={`${TRANSFORM_HEADER_PADDING_Y}px ${TRANSFORM_HEADER_PADDING_X}px`}
          fillWidth
        >
          <PipelineTransformFieldsRow
            variant={PipelineTransformFieldsRowVariant.HEADER}
            gutter={
              <FlexWrapper justifyContent={JustifyContent.CENTER} fillWidth>
                <TextShimmer height={PENDING_HANDLE_SIZE} width={PENDING_HANDLE_SIZE} />
              </FlexWrapper>
            }
          >
            <FlexWrapper alignItems={AlignItems.CENTER} height={TRANSFORM_ACTION}>
              <TextShimmer height={PENDING_LINE_HEIGHT} width={width} />
            </FlexWrapper>
          </PipelineTransformFieldsRow>
        </FlexWrapper>
      </Fragment>
    ))}
  </FlexWrapper>
);

export default PipelineTransformFieldsPending;
