import type { FC } from "react";

import { SquaresFourIcon } from "@phosphor-icons/react";

import Box from "@galaxy-io/dls/layout/Box";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import type { ResourceColumns } from "@/gen/ingestion/v1/connectors_pb";

import PipelineCanvasPanelSection from "@/pages/pipelines/canvas/panel/PipelineCanvasPanelSection";
import PipelineResourcePartitionsChip from "@/pages/pipelines/components/resource/PipelineResourcePartitionsChip";
import PipelineResourcePartitionsTable from "@/pages/pipelines/components/resource/PipelineResourcePartitionsTable";

interface PipelineCanvasPanelResourcePartitionsSectionProps {
  partitions: ResourceColumns["partitions"];
  selected: ResourceColumns["partitions"];
  isDisabled: boolean;
  error?: string;
  onChange: (selected: ResourceColumns["partitions"]) => void;
}

const PipelineCanvasPanelResourcePartitionsSection: FC<
  PipelineCanvasPanelResourcePartitionsSectionProps
> = ({ partitions, selected, isDisabled, error, onChange }) => (
  <PipelineCanvasPanelSection
    icon={SquaresFourIcon}
    header="Partitions"
    isEmpty={partitions.length === 0}
    emptyHeader="No partitions"
    emptyMessage="This resource is not partitioned."
    isOpenInitial={false}
    trailing={
      <PipelineResourcePartitionsChip
        selectedCount={selected.length}
        totalCount={partitions.length}
      />
    }
  >
    {error && (
      <Box padding={12}>
        <Text size={TextSize.BODY_SM} variant={TextVariant.ERROR}>
          {error}
        </Text>
      </Box>
    )}
    <PipelineResourcePartitionsTable
      partitions={partitions}
      selected={selected}
      isDisabled={isDisabled}
      onChange={onChange}
    />
  </PipelineCanvasPanelSection>
);

export default PipelineCanvasPanelResourcePartitionsSection;
