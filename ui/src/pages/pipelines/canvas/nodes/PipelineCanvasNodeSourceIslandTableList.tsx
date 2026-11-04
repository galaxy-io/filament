import type { FC } from "react";

import Spinner, { SpinnerSize } from "@galaxy-io/dls/feedback/Spinner";
import EmptyLayout, { EmptyLayoutSize } from "@galaxy-io/dls/layout/EmptyLayout";
import ErrorLayout, { ErrorLayoutSize } from "@galaxy-io/dls/layout/ErrorLayout";
import Flex, { AlignItems, JustifyContent } from "@galaxy-io/dls/layout/Flex";

import PipelineCanvasNodeSourceIslandTableRow from "@/pages/pipelines/canvas/nodes/PipelineCanvasNodeSourceIslandTableRow";
import type { PipelineCanvasNodeTableInfo } from "@/pages/pipelines/canvas/types";

import { IS_DEBUG } from "@/constants";

interface PipelineCanvasNodeSourceIslandTableListProps {
  tables: PipelineCanvasNodeTableInfo[];
  error?: Error | null;
  isLoading?: boolean;
}

const PipelineCanvasNodeSourceIslandTableList: FC<PipelineCanvasNodeSourceIslandTableListProps> = ({
  tables,
  error,
  isLoading = false,
}) => {
  if (isLoading) {
    return (
      <Flex justifyContent={JustifyContent.CENTER} padding={16} fillWidth>
        <Spinner size={SpinnerSize.SMALL} ariaLabel="Loading resources" />
      </Flex>
    );
  }

  if (error) {
    return (
      <Flex alignItems={AlignItems.START} padding={16} fillWidth>
        <ErrorLayout
          size={ErrorLayoutSize.SMALL}
          header="Failed to load resources"
          detail={IS_DEBUG ? error.message : undefined}
        />
      </Flex>
    );
  }

  if (!tables.length) {
    return (
      <Flex alignItems={AlignItems.START} padding={16} fillWidth>
        <EmptyLayout size={EmptyLayoutSize.SMALL} header="No tables match your search" />
      </Flex>
    );
  }

  return tables.map((table) => (
    <PipelineCanvasNodeSourceIslandTableRow key={table.name} table={table} />
  ));
};

export default PipelineCanvasNodeSourceIslandTableList;
