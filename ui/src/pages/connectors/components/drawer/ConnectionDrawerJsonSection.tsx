import type { ComponentProps } from "react";

import type Accordion from "@galaxy-io/dls/accordion/Accordion";
import CodeEditor from "@galaxy-io/dls/editor/CodeEditor";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import ConnectionDrawerSection from "@/pages/connectors/components/drawer/ConnectionDrawerSection";

interface ConnectionDrawerJsonSectionProps {
  header: string;
  icon: ComponentProps<typeof Accordion>["icon"];
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
      <CodeEditor
        content={JSON.stringify(data, null, 2)}
        lang="json"
        isReadOnly
        noBorder
        noLineNumbers
      />
    </ConnectionDrawerSection>
  );
};

export default ConnectionDrawerJsonSection;
