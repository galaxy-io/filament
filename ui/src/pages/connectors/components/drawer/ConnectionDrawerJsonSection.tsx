import type { ComponentProps } from "react";

import CodeEditor, { CodeEditorLanguage } from "@galaxy-io/dls/editor/CodeEditor";
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
      <CodeEditor
        value={JSON.stringify(data, null, 2)}
        language={CodeEditorLanguage.JSON}
        isReadOnly
        isGhost
        hasLineNumbers={false}
      />
    </ConnectionDrawerSection>
  );
};

export default ConnectionDrawerJsonSection;
