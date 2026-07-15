import { useCallback } from "react";

import { MagnifyingGlassIcon, XIcon } from "@phosphor-icons/react";

import FlexWrapper, {
  FlexDirection,
} from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import SelectInput, {
  type SelectInputOption,
} from "@galaxy-io/dls/inputs/SelectInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";

import BaseHeader from "@/layouts/components/BaseHeader";
import BaseToolbar from "@/layouts/components/BaseToolbar";

import {
  CONNECTOR_KIND_FILTER_WIDTH,
  SELECT_INPUT_OPTIONS_CONNECTOR_KIND,
} from "@/pages/connectors/constants";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

interface CreateConnectionSelectorHeaderProps {
  search: string;
  onSearchChange: (search: string) => void;
  connectorKindFilter: ConnectorKind;
  onConnectorKindFilterChange: (connectorKind: ConnectorKind) => void;
  onClose: () => void;
}

const CreateConnectionSelectorHeader = ({
  search,
  onSearchChange,
  connectorKindFilter,
  onConnectorKindFilterChange,
  onClose,
}: CreateConnectionSelectorHeaderProps) => {
  const handleSearchClear = useCallback(() => {
    onSearchChange("");
  }, [onSearchChange]);

  const selectedKindOption =
    SELECT_INPUT_OPTIONS_CONNECTOR_KIND.find(
      (opt) => opt.value === connectorKindFilter,
    ) ?? SELECT_INPUT_OPTIONS_CONNECTOR_KIND[0];

  const handleKindChange = useCallback(
    (selected: SelectInputOption | null) => {
      onConnectorKindFilterChange(
        (selected?.value as ConnectorKind) ?? ConnectorKind.UNSPECIFIED,
      );
    },
    [onConnectorKindFilterChange],
  );

  return (
    <FlexWrapper direction={FlexDirection.COLUMN} fillWidth>
      <FlexWrapper padding="16px" fillWidth>
        <BaseHeader
          title="New connection"
          description="Connect to a data source or sink."
          onClose={onClose}
        />
      </FlexWrapper>
      <HorizontalDivider />
      <FlexWrapper padding="8px 16px" fillWidth>
        <BaseToolbar
          leadingActions={[
            <TextInput
              key="search"
              placeholder="Search connectors..."
              leading={{ icon: MagnifyingGlassIcon }}
              trailing={
                search ? { icon: XIcon, onClick: handleSearchClear } : undefined
              }
              onChange={onSearchChange}
              value={search}
              width={240}
            />,
            <SelectInput
              key="kind-filter"
              options={SELECT_INPUT_OPTIONS_CONNECTOR_KIND}
              value={selectedKindOption}
              onChange={handleKindChange}
              width={CONNECTOR_KIND_FILTER_WIDTH}
            />,
          ]}
        />
      </FlexWrapper>
    </FlexWrapper>
  );
};

export default CreateConnectionSelectorHeader;
