import { type FC, useState } from "react";

import { ColumnsIcon } from "@phosphor-icons/react";

import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import Box from "@galaxy-io/dls/layout/Box";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import type { ResourceColumn } from "@/gen/ingestion/v1/connectors_pb";

import PipelineCanvasPanelSection from "@/pages/pipelines/canvas/panel/PipelineCanvasPanelSection";
import PipelineResourceColumnsTable from "@/pages/pipelines/components/resource/PipelineResourceColumnsTable";

interface PipelineCanvasPanelResourceColumnsSectionProps {
  columns: ResourceColumn[];
  selected: ResourceColumn["name"][];
  locked: ResourceColumn["name"][];
  isDisabled: boolean;
  error?: string;
  onChange: (selected: ResourceColumn["name"][]) => void;
}

const PipelineCanvasPanelResourceColumnsSection: FC<
  PipelineCanvasPanelResourceColumnsSectionProps
> = ({ columns, selected, locked, isDisabled, error, onChange }) => {
  const [isOpen, setIsOpen] = useState(false);

  return (
    <PipelineCanvasPanelSection
      icon={ColumnsIcon}
      header="Columns"
      isEmpty={columns.length === 0}
      emptyHeader="No columns"
      emptyMessage="The source reported no columns for this resource."
      isOpen={isOpen}
      onToggle={() => setIsOpen((prev) => !prev)}
      trailing={
        <Chip
          label={`${selected.length} of ${columns.length}`}
          tooltip="Selected columns"
          size={ChipSize.SMALL}
          variant={selected.length === columns.length ? ChipVariant.SECONDARY : ChipVariant.PRIMARY}
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
      <PipelineResourceColumnsTable
        columns={columns}
        selected={selected}
        locked={locked}
        isDisabled={isDisabled}
        onChange={onChange}
      />
    </PipelineCanvasPanelSection>
  );
};

export default PipelineCanvasPanelResourceColumnsSection;
