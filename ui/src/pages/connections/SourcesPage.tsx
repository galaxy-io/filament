import type { FC } from "react";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import ConnectionsPage from "@/pages/connections/ConnectionsPage";

const SourcesPage: FC = () => <ConnectionsPage kind={ConnectorKind.SOURCE} />;

export default SourcesPage;
