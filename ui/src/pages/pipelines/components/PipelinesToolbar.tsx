import { useState } from "react";

import { MagnifyingGlassIcon, PlusIcon } from "@phosphor-icons/react";
import { styled } from "@linaria/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, { AlignItems, FlexGap, JustifyContent } from "@galaxy-io/dls/containers/FlexWrapper";
import Dropdown, { DropdownPosition } from "@galaxy-io/dls/dropdown/Dropdown";
import DropdownButton from "@galaxy-io/dls/dropdown/DropdownButton";
import DropdownItem from "@galaxy-io/dls/dropdown/DropdownItem";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import { InputSize } from "@galaxy-io/dls/inputs/Input";

import { PipelineGroup } from "@/pages/pipelines/types";

const SEARCH_WIDTH = 280;

const ToolbarWrapper = styled.div`
  width: 100%;

  padding: 8px 12px;

  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
`;

const GROUP_FILTER_LABELS: Record<PipelineGroup, string> = {
  [PipelineGroup.ACTIVE]: "Active",
  [PipelineGroup.NEEDS_ATTENTION]: "Needs attention",
  [PipelineGroup.PAUSED]: "Paused",
};

interface PipelinesToolbarProps {
  search: string;
  onSearchChange: (value: string) => void;
  groupFilter: PipelineGroup | null;
  onGroupFilterChange: (group: PipelineGroup | null) => void;
}

const PipelinesToolbar = ({
  search,
  onSearchChange,
  groupFilter,
  onGroupFilterChange,
}: PipelinesToolbarProps) => {
  const [isFiltersOpen, setIsFiltersOpen] = useState(false);

  const handleSelectGroupFilter = (group: PipelineGroup | null) => {
    onGroupFilterChange(group);
    setIsFiltersOpen(false);
  };

  return (
    <ToolbarWrapper>
      <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.XSMALL}>
        <TextInput
          value={search}
          onChange={onSearchChange}
          placeholder="Search"
          size={InputSize.SMALL}
          width={SEARCH_WIDTH}
          leading={{ icon: MagnifyingGlassIcon }}
        />
        <Dropdown
          isOpen={isFiltersOpen}
          onClose={() => setIsFiltersOpen(false)}
          position={DropdownPosition.BOTTOM_START}
          body={
            <>
              <DropdownItem
                label="All pipelines"
                onClick={() => handleSelectGroupFilter(null)}
              />
              {Object.values(PipelineGroup).map((group) => (
                <DropdownItem
                  key={group}
                  label={GROUP_FILTER_LABELS[group]}
                  onClick={() => handleSelectGroupFilter(group)}
                />
              ))}
            </>
          }
        >
          <DropdownButton
            label={groupFilter ? GROUP_FILTER_LABELS[groupFilter] : "Filters"}
            isOpen={isFiltersOpen}
            onClick={() => setIsFiltersOpen((prev) => !prev)}
            variant={ButtonVariant.SECONDARY}
            size={ButtonSize.SMALL}
          />
        </Dropdown>
      </FlexWrapper>
      <FlexWrapper alignItems={AlignItems.CENTER} justifyContent={JustifyContent.END}>
        <Button
          label="New pipeline"
          icon={PlusIcon}
          variant={ButtonVariant.PRIMARY}
          size={ButtonSize.SMALL}
          onClick={() => {
            // TODO: open the pipeline builder once it exists.
          }}
        />
      </FlexWrapper>
    </ToolbarWrapper>
  );
};

export default PipelinesToolbar;
