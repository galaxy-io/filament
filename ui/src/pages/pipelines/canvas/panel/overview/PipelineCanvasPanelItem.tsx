import { styled } from "@linaria/react";

import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

const PipelineCanvasPanelItem = withTheme(styled.button<PropsWithTheme>`
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
    border-bottom: 0.5px solid ${({ theme }) => theme.color.border.primary};
  }

  &:hover {
    background-color: ${({ theme }) => theme.color.background.tertiary};
  }
`);

export default PipelineCanvasPanelItem;
