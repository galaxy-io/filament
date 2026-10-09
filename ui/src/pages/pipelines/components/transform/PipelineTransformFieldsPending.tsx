import { Fragment } from "react";

import Skeleton, { SkeletonSize } from "@galaxy-io/dls/feedback/Skeleton";
import Box from "@galaxy-io/dls/layout/Box";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";

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

const PipelineTransformFieldsPending = () => (
  <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} fillWidth>
    {PENDING_ROW_WIDTHS.map((width, index) => (
      <Fragment key={width}>
        {index > 0 && <Divider />}
        <Flex
          alignItems={AlignItems.START}
          padding={[TRANSFORM_HEADER_PADDING_Y, TRANSFORM_HEADER_PADDING_X]}
          fillWidth
        >
          <PipelineTransformFieldsRow
            variant={PipelineTransformFieldsRowVariant.HEADER}
            gutter={
              <Flex alignItems={AlignItems.START} justifyContent={JustifyContent.CENTER} fillWidth>
                <Box width={PENDING_HANDLE_SIZE}>
                  <Skeleton size={SkeletonSize.SMALL} />
                </Box>
              </Flex>
            }
          >
            <Flex alignItems={AlignItems.CENTER} height={TRANSFORM_ACTION}>
              <Box width={width}>
                <Skeleton size={SkeletonSize.SMALL} />
              </Box>
            </Flex>
          </PipelineTransformFieldsRow>
        </Flex>
      </Fragment>
    ))}
  </Flex>
);

export default PipelineTransformFieldsPending;
