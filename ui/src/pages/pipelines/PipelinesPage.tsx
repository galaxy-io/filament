import { useMemo, useState } from "react";

import { styled } from "@linaria/react";

import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import PipelineGroupSection from "@/pages/pipelines/components/PipelineGroupSection";
import PipelinesToolbar from "@/pages/pipelines/components/PipelinesToolbar";
import { groupPipelineItems, usePipelineListItems } from "@/pages/pipelines/hooks";
import { PipelineGroup } from "@/pages/pipelines/types";

const PipelineListWrapper = styled.div`
  flex: 1;
  width: 100%;
  min-height: 0;

  display: flex;
  flex-direction: column;

  overflow-y: auto;
`;

const PipelinesPage = () => {
  const [search, setSearch] = useState("");
  const [groupFilter, setGroupFilter] = useState<PipelineGroup | null>(null);

  const { items, isLoading } = usePipelineListItems();

  const groups = useMemo(() => {
    const query = search.trim().toLowerCase();
    const filtered = query
      ? items.filter((item) => item.name.toLowerCase().includes(query))
      : items;
    return groupPipelineItems(filtered);
  }, [items, search]);

  const visibleGroups = groupFilter ? [groupFilter] : Object.values(PipelineGroup);

  return (
    <FlexWrapper fillWidth fillHeight direction={FlexDirection.COLUMN}>
      <PipelinesToolbar
        search={search}
        onSearchChange={setSearch}
        groupFilter={groupFilter}
        onGroupFilterChange={setGroupFilter}
      />
      <FlexItem grow={0} shrink={0} fillWidth>
        <HorizontalDivider />
      </FlexItem>
      <PipelineListWrapper>
        {isLoading ? (
          <FlexWrapper
            fillWidth
            fillHeight
            alignItems={AlignItems.CENTER}
            justifyContent={JustifyContent.CENTER}
          >
            <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
              Loading...
            </Text>
          </FlexWrapper>
        ) : (
          visibleGroups.map((group) => (
            <PipelineGroupSection
              key={group}
              group={group}
              pipelines={groups[group]}
              defaultExpanded={group !== PipelineGroup.PAUSED}
            />
          ))
        )}
      </PipelineListWrapper>
    </FlexWrapper>
  );
};

export default PipelinesPage;
