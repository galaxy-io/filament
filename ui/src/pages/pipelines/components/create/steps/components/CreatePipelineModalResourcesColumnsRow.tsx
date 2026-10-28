import { type FC, useMemo } from "react";

import Box, { BoxVariant } from "@galaxy-io/dls/layout/Box";

import { ReadMode } from "@/gen/ingestion/v1/common_pb";

import {
  useCreatePipelineModalActions,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import type { CreatePipelineModalResourceRow } from "@/pages/pipelines/components/create/types";
import PipelineResourceColumnsTable from "@/pages/pipelines/components/resource/PipelineResourceColumnsTable";

interface CreatePipelineModalResourcesColumnsRowProps {
  row: CreatePipelineModalResourceRow;
}

const CreatePipelineModalResourcesColumnsRow: FC<CreatePipelineModalResourcesColumnsRowProps> = ({
  row,
}) => {
  const { activeSinkId, hasReadLevers } = useCreatePipelineModalState();
  const { setResourceColumns } = useCreatePipelineModalActions();

  const locked = useMemo(
    () => [
      ...row.columns.filter((column) => column.isPrimaryKey).map((column) => column.name),
      ...(hasReadLevers && row.readMode === ReadMode.INCREMENTAL && row.cursorField
        ? [row.cursorField]
        : []),
    ],
    [row.columns, row.readMode, row.cursorField, hasReadLevers],
  );

  return (
    <Box variant={BoxVariant.PRIMARY} fillWidth>
      <PipelineResourceColumnsTable
        columns={row.columns}
        selected={row.selectedColumns}
        locked={locked}
        isDisabled={false}
        onChange={(columns) =>
          setResourceColumns({ sinkId: activeSinkId, resource: row.name, columns })
        }
      />
    </Box>
  );
};

export default CreatePipelineModalResourcesColumnsRow;
