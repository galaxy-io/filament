import type { FC, PropsWithChildren } from "react";

import { styled } from "@linaria/react";

import type { ObservabilityPipelinesColumn } from "@/pages/observability/components/pipelines/types";

const ObservabilityPipelinesColumnCellFrame = styled.div`
  display: block;
  min-width: 0;
`;

interface ObservabilityPipelinesColumnCellProps {
  column: ObservabilityPipelinesColumn;
}

const ObservabilityPipelinesColumnCell: FC<
  PropsWithChildren<ObservabilityPipelinesColumnCellProps>
> = ({ column, children }) => (
  <ObservabilityPipelinesColumnCellFrame data-pipelines-column={column}>
    {children}
  </ObservabilityPipelinesColumnCellFrame>
);

export default ObservabilityPipelinesColumnCell;
