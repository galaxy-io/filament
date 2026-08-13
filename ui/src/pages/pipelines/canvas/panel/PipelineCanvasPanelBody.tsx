import { styled } from "@linaria/react";

const PipelineCanvasPanelBody = styled.div`
  flex: 1;
  min-height: 0;
  padding: 12px;

  display: flex;
  flex-direction: column;
  gap: 8px;

  overflow-y: auto;

  > * {
    flex-shrink: 0;
  }
`;

export default PipelineCanvasPanelBody;
