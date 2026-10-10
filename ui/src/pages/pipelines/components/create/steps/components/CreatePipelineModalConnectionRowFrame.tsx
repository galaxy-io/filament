import { styled } from "@linaria/react";

import { t } from "@galaxy-io/dls/theme/tokens/t";

const CreatePipelineModalConnectionRowFrame = styled.div<{ $isDisabled?: boolean }>`
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 8px;
  flex-shrink: 0;

  border-radius: ${t.radius.md};
  cursor: ${({ $isDisabled }) => ($isDisabled ? "default" : "pointer")};

  &:hover {
    background-color: ${({ $isDisabled }) =>
      $isDisabled ? "transparent" : t.color.background.hovered};
  }
`;

export default CreatePipelineModalConnectionRowFrame;
