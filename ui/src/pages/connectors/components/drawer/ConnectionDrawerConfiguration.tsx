import type { JsonObject } from "@bufbuild/protobuf";
import { SlidersIcon } from "@phosphor-icons/react";

import Accordion from "@galaxy-io/dls/accordion/Accordion";
import Badge, { BadgeSize, BadgeVariant } from "@galaxy-io/dls/badge/Badge";
import CodeEditor from "@galaxy-io/dls/editor/CodeEditor";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";

import EmptyLayout, { EmptyLayoutSize } from "@/layouts/EmptyLayout";

interface ConnectionDrawerConfigurationProps {
  config?: JsonObject;
}

const ConnectionDrawerConfiguration = ({ config }: ConnectionDrawerConfigurationProps) => {
  const configEntries = config ? Object.entries(config) : [];

  return (
    <Accordion
      header="Configuration"
      icon={SlidersIcon}
      metric={
        <Badge
          count={configEntries.length}
          size={BadgeSize.SMALL}
          variant={BadgeVariant.SECONDARY}
        />
      }
      isOpenInitial={false}
      padding={configEntries.length > 0 ? 0 : "24px"}
    >
      {configEntries.length === 0 ? (
        <EmptyLayout
          size={EmptyLayoutSize.SMALL}
          icon={<Icon component={SlidersIcon} size={16} variant={IconVariant.TERTIARY} />}
          header="No configuration"
          message="This connection has no configuration values."
        />
      ) : (
        <CodeEditor
          content={JSON.stringify(config, null, 2)}
          lang="json"
          isReadOnly
          noBorder
          noLineNumbers
        />
      )}
    </Accordion>
  );
};

export default ConnectionDrawerConfiguration;
