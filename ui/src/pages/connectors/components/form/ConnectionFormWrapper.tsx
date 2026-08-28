import type { PropsWithChildren } from "react";

import { styled } from "@linaria/react";

import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { CREATE_CONNECTION_MODAL_HEIGHT } from "@/pages/connectors/constants";

interface ConnectionFormWrapperProps {
  width: number;
}

const Wrapper = withTheme(styled.div<PropsWithTheme & { $width: number }>`
  display: flex;
  flex-direction: column;

  height: ${CREATE_CONNECTION_MODAL_HEIGHT}px;
  width: ${({ $width }) => $width}px;

  background-color: ${({ theme }) => theme.color.background.primary};
  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 8px;
  overflow: hidden;
`);

const ConnectionFormWrapper = ({
  children,
  width,
}: PropsWithChildren<ConnectionFormWrapperProps>) => {
  return <Wrapper $width={width}>{children}</Wrapper>;
};

export default ConnectionFormWrapper;
