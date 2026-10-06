import type { ComponentProps } from "react";

import CodeBlock, { CodeBlockLanguage } from "@galaxy-io/dls/text/CodeBlock";
import type Widget from "@galaxy-io/dls/widget/Widget";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import ConnectionDrawerSection from "@/pages/connectors/components/drawer/ConnectionDrawerSection";

interface ConnectionDrawerJsonSectionProps {
  header: string;
  icon: ComponentProps<typeof Widget>["icon"];
  data: Connection["config"] | Connection["secretRefs"] | undefined;
  emptyHeader: string;
  emptyMessage: string;
}

const ConnectionDrawerJsonSection = ({
  header,
  icon,
  data,
  emptyHeader,
  emptyMessage,
}: ConnectionDrawerJsonSectionProps) => {
  return (
    <ConnectionDrawerSection
      header={header}
      icon={icon}
      count={Object.keys(data ?? {}).length}
      emptyHeader={emptyHeader}
      emptyMessage={emptyMessage}
    >
      <CodeBlock
        content={JSON.stringify(data, null, 2)}
        language={CodeBlockLanguage.JSON}
        canCopy
      />
    </ConnectionDrawerSection>
  );
};

export default ConnectionDrawerJsonSection;
