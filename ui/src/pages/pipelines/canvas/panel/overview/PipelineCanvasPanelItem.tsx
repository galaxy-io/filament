import { styled } from "@linaria/react";

import { t } from "@galaxy-io/dls/theme/tokens/t";

const PipelineCanvasPanelItem = styled.button`
  width: 100%;
  padding: 12px;

  display: flex;
  align-items: center;
  gap: 8px;

  background-color: transparent;
  border: none;
  cursor: pointer;
  text-align: left;

  transition: background-color 100ms ease;

  &:not(:last-child) {
    border-bottom: 0.5px solid ${t.color.border.primary};
  }

  &:hover {
    background-color: ${t.color.background.tertiary};
  }
`;

export default PipelineCanvasPanelItem;
