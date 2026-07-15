import GridWrapper from "@galaxy-io/dls/containers/GridWrapper";

import ConnectorCardLoading from "@/pages/connectors/components/ConnectorCardLoading";
import { CONNECTOR_GRID_MIN_COLUMN_WIDTH } from "@/pages/connectors/constants";

const LOADING_CARD_COUNT = 6;

const ConnectorsPageLoading = () => {
  return (
    <GridWrapper
      columns={`repeat(auto-fill, minmax(${CONNECTOR_GRID_MIN_COLUMN_WIDTH}px, 1fr))`}
      gap={12}
    >
      {Array.from({ length: LOADING_CARD_COUNT }).map((_, index) => (
        <ConnectorCardLoading key={index} />
      ))}
    </GridWrapper>
  );
};

export default ConnectorsPageLoading;
