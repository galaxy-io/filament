import type { FC } from "react";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import ConnectionsPage from "@/pages/connections/ConnectionsPage";

const SinksPage: FC = () => <ConnectionsPage kind={ConnectorKind.SINK} />;

export default SinksPage;
