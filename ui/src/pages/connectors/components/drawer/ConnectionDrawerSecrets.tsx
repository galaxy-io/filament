import { KeyIcon } from "@phosphor-icons/react";

import Accordion from "@galaxy-io/dls/accordion/Accordion";
import Badge, { BadgeSize, BadgeVariant } from "@galaxy-io/dls/badge/Badge";
import FlexWrapper from "@galaxy-io/dls/containers/FlexWrapper";
import CodeEditor from "@galaxy-io/dls/editor/CodeEditor";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";

import EmptyLayout, { EmptyLayoutSize } from "@/layouts/EmptyLayout";

interface ConnectionDrawerSecretsProps {
  secretRefs: { [key: string]: string };
}

const ConnectionDrawerSecrets = ({ secretRefs }: ConnectionDrawerSecretsProps) => {
  const secretRefEntries = Object.entries(secretRefs || {});

  return (
    <Accordion
      header="Secrets"
      icon={KeyIcon}
      metric={
        <Badge
          count={secretRefEntries.length}
          size={BadgeSize.SMALL}
          variant={BadgeVariant.SECONDARY}
        />
      }
      isOpenInitial={false}
      padding={secretRefEntries.length > 0 ? 0 : "24px"}
    >
      {secretRefEntries.length === 0 ? (
        <EmptyLayout
          size={EmptyLayoutSize.SMALL}
          icon={<Icon component={KeyIcon} size={16} variant={IconVariant.TERTIARY} />}
          header="No secrets"
          message="This connection has no secret references."
        />
      ) : (
        <CodeEditor
          content={JSON.stringify(secretRefs, null, 2)}
          lang="json"
          isReadOnly
          noBorder
          noLineNumbers
        />
      )}
    </Accordion>
  );
};

export default ConnectionDrawerSecrets;
