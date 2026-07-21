import type { JsonObject } from "@bufbuild/protobuf";
import { GearIcon } from "@phosphor-icons/react";

import Accordion from "@galaxy-io/dls/accordion/Accordion";
import Badge, { BadgeSize, BadgeVariant } from "@galaxy-io/dls/badge/Badge";
import FlexWrapper from "@galaxy-io/dls/containers/FlexWrapper";
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
      icon={GearIcon}
      metric={
        <Badge
          count={configEntries.length}
          size={BadgeSize.SMALL}
          variant={BadgeVariant.SECONDARY}
        />
      }
      isOpenInitial={false}
      padding="0"
    >
      {configEntries.length === 0 ? (
        <FlexWrapper padding="16px" fillWidth>
          <EmptyLayout
            size={EmptyLayoutSize.SMALL}
            icon={<Icon component={GearIcon} size={16} variant={IconVariant.TERTIARY} />}
            header="No configuration"
            message="This connection has no configuration values."
          />
        </FlexWrapper>
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
