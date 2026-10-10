import { styled } from "@linaria/react";

import { t } from "@galaxy-io/dls/theme/tokens/t";

const CREATE_PIPELINE_MODAL_CONNECTION_ROW_GAP = 10;
const CREATE_PIPELINE_MODAL_CONNECTION_ROW_PADDING_Y = 6;

const CreatePipelineModalConnectionRowFrame = styled.div<{ $isDisabled?: boolean }>`
  display: flex;
  align-items: center;
  gap: ${CREATE_PIPELINE_MODAL_CONNECTION_ROW_GAP}px;
  padding: ${CREATE_PIPELINE_MODAL_CONNECTION_ROW_PADDING_Y}px ${t.space[8]};
  flex-shrink: 0;

  border-radius: ${t.radius.md};
  cursor: ${({ $isDisabled }) => ($isDisabled ? "default" : "pointer")};

  &:hover {
    background-color: ${({ $isDisabled }) =>
      $isDisabled ? "transparent" : t.color.background.hovered};
  }
`;

export default CreatePipelineModalConnectionRowFrame;
