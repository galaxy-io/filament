import type { ComponentProps, FC } from "react";

import { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import CopyButton from "@galaxy-io/dls/buttons/CopyButton";
import JsonViewer, { JsonViewerSize } from "@galaxy-io/dls/json/JsonViewer";
import type Widget from "@galaxy-io/dls/widget/Widget";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import ConnectionDrawerSection from "@/pages/connections/components/drawer/ConnectionDrawerSection";

const JSON_INDENT = 2;

interface ConnectionDrawerJsonSectionProps {
  header: string;
  icon: ComponentProps<typeof Widget>["icon"];
  data: Connection["config"] | Connection["secretRefs"] | undefined;
  emptyHeader: string;
  emptyMessage: string;
}

const ConnectionDrawerJsonSection: FC<ConnectionDrawerJsonSectionProps> = ({
  header,
  icon,
  data,
  emptyHeader,
  emptyMessage,
}) => {
  const document = data ?? {};

  return (
    <ConnectionDrawerSection
      header={header}
      icon={icon}
      count={Object.keys(document).length}
      emptyHeader={emptyHeader}
      emptyMessage={emptyMessage}
      actions={
        <CopyButton
          value={JSON.stringify(document, null, JSON_INDENT)}
          ariaLabel={`Copy ${header.toLowerCase()}`}
          variant={ButtonVariant.TERTIARY}
          size={ButtonSize.SMALL}
        />
      }
    >
      <JsonViewer ariaLabel={header} data={document} size={JsonViewerSize.SMALL} />
    </ConnectionDrawerSection>
  );
};

export default ConnectionDrawerJsonSection;
