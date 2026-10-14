import { type FC, type RefObject, useState } from "react";

import { keepPreviousData } from "@tanstack/react-query";

import Skeleton, { SkeletonSize, SkeletonVariant } from "@galaxy-io/dls/feedback/Skeleton";
import { useEndReached } from "@galaxy-io/dls/hooks/useEndReached";
import SearchInput from "@galaxy-io/dls/inputs/SearchInput";
import Box from "@galaxy-io/dls/layout/Box";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import ScrollArea from "@galaxy-io/dls/layout/ScrollArea";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import { CONNECTOR_KIND_TO_PLURAL_NOUN_MAP } from "@/components/connections/constants";

import { CREATE_PIPELINE_MODAL_CONNECTION_GHOST_COUNT } from "@/pages/pipelines/components/create/constants";
import CreatePipelineModalConnectionRow from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalConnectionRow";
import CreatePipelineModalConnectionRowFrame from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalConnectionRowFrame";
import CreatePipelineModalConnectionsState from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalConnectionsState";

import {
  createListConnectionsInput,
  useListConnectionsInfiniteQuery,
} from "@/api/queries/connections";

import { LIST_SEARCH_DEBOUNCE_MS } from "@/constants";

interface CreatePipelineModalConnectionsPaneProps {
  kind: ConnectorKind;
}

interface CreatePipelineModalConnectionsPaneState {
  q: string;
}

const DEFAULT_STATE: CreatePipelineModalConnectionsPaneState = {
  q: "",
};

const CreatePipelineModalConnectionsPane: FC<CreatePipelineModalConnectionsPaneProps> = ({
  kind,
}) => {
  const [state, setState] = useState<CreatePipelineModalConnectionsPaneState>(DEFAULT_STATE);

  const handleSearch = (q: string) => {
    setState((prev) => ({ ...prev, q }));
  };

  const { data, isLoading, error, hasNextPage, isFetchingNextPage, fetchNextPage } =
    useListConnectionsInfiniteQuery({
      input: createListConnectionsInput(kind, { q: state.q }),
      options: { placeholderData: keepPreviousData },
    });

  const kindConnections = data?.pages.flatMap((page) => page.connections) ?? [];
  const endRef = useEndReached({
    onEndReached: hasNextPage ? () => void fetchNextPage() : undefined,
    isEnabled: !isFetchingNextPage,
    itemCount: kindConnections.length,
  });

  const renderList = () => {
    if (isLoading) {
      return (
        <Flex
          direction={FlexDirection.COLUMN}
          alignItems={AlignItems.STRETCH}
          gap={2}
          padding={8}
          grow={1}
          basis={0}
          minHeight={0}
        >
          {Array.from({ length: CREATE_PIPELINE_MODAL_CONNECTION_GHOST_COUNT }, (_, index) => (
            // biome-ignore lint/suspicious/noArrayIndexKey: static skeleton list
            <CreatePipelineModalConnectionRowFrame key={index}>
              <Box width={16}>
                <Skeleton />
              </Box>
              <Box width={24}>
                <Skeleton variant={SkeletonVariant.RECT} size={SkeletonSize.SMALL} />
              </Box>
              <Box width={140}>
                <Skeleton />
              </Box>
            </CreatePipelineModalConnectionRowFrame>
          ))}
        </Flex>
      );
    }

    if (error) {
      return (
        <CreatePipelineModalConnectionsState
          message="Failed to load connections"
          error={error}
          connectorKind={kind}
        />
      );
    }

    if (!kindConnections.length) {
      return (
        <CreatePipelineModalConnectionsState
          message={
            state.q
              ? "No connections match your search"
              : `No ${CONNECTOR_KIND_TO_PLURAL_NOUN_MAP[kind]} found`
          }
          connectorKind={kind}
        />
      );
    }

    return (
      <FlexItem grow={1} basis={0} minHeight={0}>
        <ScrollArea>
          <Flex
            direction={FlexDirection.COLUMN}
            alignItems={AlignItems.STRETCH}
            gap={2}
            padding={8}
          >
            {kindConnections.map((connection) => (
              <CreatePipelineModalConnectionRow
                key={connection.id}
                connection={connection}
                kind={kind}
              />
            ))}
            <div ref={endRef as RefObject<HTMLDivElement>} />
          </Flex>
        </ScrollArea>
      </FlexItem>
    );
  };

  return (
    <Flex
      direction={FlexDirection.COLUMN}
      alignItems={AlignItems.STRETCH}
      grow={1}
      basis={0}
      minWidth={0}
    >
      <Flex
        alignItems={AlignItems.START}
        direction={FlexDirection.COLUMN}
        gap={8}
        padding={8}
        fillWidth
      >
        <SearchInput
          ariaLabel={`Search ${CONNECTOR_KIND_TO_PLURAL_NOUN_MAP[kind]}`}
          placeholder={`Search ${CONNECTOR_KIND_TO_PLURAL_NOUN_MAP[kind]}...`}
          debounceMs={LIST_SEARCH_DEBOUNCE_MS}
          onSearch={handleSearch}
          fillWidth
        />
      </Flex>
      <Divider />
      {renderList()}
    </Flex>
  );
};

export default CreatePipelineModalConnectionsPane;
