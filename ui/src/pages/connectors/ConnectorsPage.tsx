import { useMemo } from "react";

import {
  BookOpenIcon,
  MagnifyingGlassIcon,
  PlusIcon,
  WarningCircleIcon,
} from "@phosphor-icons/react";
import { styled } from "@linaria/react";
import { useNavigate } from "@tanstack/react-router";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, {
  FlexDirection,
} from "@galaxy-io/dls/containers/FlexWrapper";
import GridWrapper from "@galaxy-io/dls/containers/GridWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Dropdown, { DropdownPosition } from "@galaxy-io/dls/dropdown/Dropdown";
import DropdownButton from "@galaxy-io/dls/dropdown/DropdownButton";
import DropdownItem from "@galaxy-io/dls/dropdown/DropdownItem";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import TextInput from "@galaxy-io/dls/inputs/TextInput";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import { useListConnectionsQuery } from "@/api/queries/connectors";

import { CONNECTORS_DOCS_URL } from "@/constants";

import BaseToolbar from "@/layouts/components/BaseToolbar";
import ErrorLayout from "@/layouts/ErrorLayout";

import ConnectionCard from "@/pages/connectors/components/ConnectionCard";
import ConnectionCardLoading from "@/pages/connectors/components/ConnectionCardLoading";
import {
  CONNECTOR_GRID_MIN_COLUMN_WIDTH,
  CONNECTOR_KIND_TO_LABEL_MAP,
  CONNECTOR_SEARCH_WIDTH,
} from "@/pages/connectors/constants";
import { useConnectorsPageState } from "@/pages/connectors/hooks";
import EmptyLayout from "@/layouts/EmptyLayout";
import ConnectorsEmptyDark from "@/assets/components/ConnectorsEmptyDark";

const LOADING_CARD_COUNT = 20;

const ConnectorListScrollArea = styled.div`
  flex: 1;
  width: 100%;
  min-height: 0;

  padding: 12px;

  overflow-y: auto;
`;

const ConnectorsPage = () => {
  const navigate = useNavigate();

  const {
    search,
    setSearch,
    kindFilter,
    isFiltersOpen,
    setIsFiltersOpen,
    handleSelectKindFilter,
  } = useConnectorsPageState();

  const { data, isLoading, isError } = useListConnectionsQuery({
    input: { kind: ConnectorKind.UNSPECIFIED },
  });

  const isToolbarDisabled = isLoading || isError;

  const filteredConnections = useMemo(() => {
    if (!data?.connections) return [];

    return data.connections.filter((connection) => {
      const matchesSearch = search
        ? connection.name.toLowerCase().includes(search.toLowerCase())
        : true;
      const matchesKind =
        kindFilter === ConnectorKind.UNSPECIFIED ||
        connection.kind === kindFilter;
      return matchesSearch && matchesKind;
    });
  }, [data?.connections, search, kindFilter]);

  const handleConnectionClick = (connectionId: string) => {
    void navigate({
      to: ".",
      search: (prev) => ({ ...prev, connectionId }),
    });
  };

  const handleReadTheDocs = () => {
    window.open(CONNECTORS_DOCS_URL, "_blank", "noopener,noreferrer");
  };

  const renderContent = () => {
    if (isLoading) {
      return (
        <GridWrapper
          columns={`repeat(auto-fill, minmax(${CONNECTOR_GRID_MIN_COLUMN_WIDTH}px, 1fr))`}
          gap={12}
        >
          {Array.from({ length: LOADING_CARD_COUNT }).map((_, index) => (
            <ConnectionCardLoading key={index} />
          ))}
        </GridWrapper>
      );
    }

    if (isError) {
      return (
        <ErrorLayout
          icon={
            <Icon
              component={WarningCircleIcon}
              size={20}
              variant={IconVariant.ERROR}
            />
          }
          message="Failed to load connectors. Please try again."
        />
      );
    }

    const hasConnections = (data?.connections?.length ?? 0) > 0;

    if (!hasConnections) {
      return (
        <EmptyLayout
          icon={<ConnectorsEmptyDark height={200} />}
          header="No connectors found"
          message="Connect data sources and destinations to move data in and out."
          actions={
            <FlexWrapper gap={8}>
              <Button
                label="New connector"
                icon={PlusIcon}
                variant={ButtonVariant.PRIMARY}
                onClick={() => {
                  // TODO: open the connector creation flow once it exists.
                }}
              />
              <Button
                label="Read the docs"
                icon={BookOpenIcon}
                variant={ButtonVariant.SECONDARY}
                onClick={handleReadTheDocs}
              />
            </FlexWrapper>
          }
        />
      );
    }

    if (!filteredConnections.length) {
      return (
        <EmptyLayout
          message={
            search
              ? "No connectors match your search"
              : "No connectors match your filters"
          }
        />
      );
    }

    return (
      <GridWrapper
        columns={`repeat(auto-fill, minmax(${CONNECTOR_GRID_MIN_COLUMN_WIDTH}px, 1fr))`}
        gap={12}
      >
        {filteredConnections.map((connection) => (
          <ConnectionCard
            key={connection.id}
            connection={connection}
            onClick={() => handleConnectionClick(connection.id)}
          />
        ))}
      </GridWrapper>
    );
  };

  return (
    <FlexWrapper fillWidth fillHeight direction={FlexDirection.COLUMN}>
      <FlexWrapper padding={"8px 12px"} fillWidth>
        <BaseToolbar
          leadingActions={[
            <TextInput
              key="search"
              value={search}
              onChange={setSearch}
              placeholder="Search"
              width={CONNECTOR_SEARCH_WIDTH}
              leading={{ icon: MagnifyingGlassIcon }}
              isDisabled={isToolbarDisabled}
            />,
            <Dropdown
              key="filters"
              isOpen={isFiltersOpen}
              onClose={() => setIsFiltersOpen(false)}
              position={DropdownPosition.BOTTOM_START}
              body={
                <>
                  <DropdownItem
                    label="All connectors"
                    onClick={() =>
                      handleSelectKindFilter(ConnectorKind.UNSPECIFIED)
                    }
                  />
                  <DropdownItem
                    label="Sources"
                    onClick={() => handleSelectKindFilter(ConnectorKind.SOURCE)}
                  />
                  <DropdownItem
                    label="Sinks"
                    onClick={() => handleSelectKindFilter(ConnectorKind.SINK)}
                  />
                </>
              }
            >
              <DropdownButton
                label={CONNECTOR_KIND_TO_LABEL_MAP[kindFilter]}
                isOpen={isFiltersOpen}
                onClick={() => setIsFiltersOpen((prev) => !prev)}
                variant={ButtonVariant.SECONDARY}
                isDisabled={isToolbarDisabled}
              />
            </Dropdown>,
          ]}
          trailingActions={[
            <Button
              key="new-connector"
              label="New connector"
              icon={PlusIcon}
              variant={ButtonVariant.PRIMARY}
              isDisabled={isToolbarDisabled}
              onClick={() => {
                // TODO: open the connector creation flow once it exists.
              }}
            />,
          ]}
        />
      </FlexWrapper>
      <FlexItem grow={0} shrink={0} fillWidth>
        <HorizontalDivider />
      </FlexItem>
      <ConnectorListScrollArea>{renderContent()}</ConnectorListScrollArea>
    </FlexWrapper>
  );
};

export default ConnectorsPage;
