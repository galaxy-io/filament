import type { FC, PropsWithChildren, ReactNode } from "react";

import type { Icon as PhosphorIcon } from "@phosphor-icons/react";

import EmptyLayout, { EmptyLayoutSize } from "@galaxy-io/dls/layout/EmptyLayout";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import Widget, { WidgetVariant } from "@galaxy-io/dls/widget/Widget";

interface PipelineCanvasPanelSectionProps {
  icon?: PhosphorIcon;
  header: string;
  isEmpty: boolean;
  emptyHeader: string;
  emptyMessage: string;
  hasInset?: boolean;
  isOpenInitial?: boolean;
  trailing?: ReactNode;
  isOpen?: boolean;
  onToggle?: () => void;
}

const PipelineCanvasPanelSection: FC<PropsWithChildren<PipelineCanvasPanelSectionProps>> = ({
  icon,
  header,
  isEmpty,
  emptyHeader,
  emptyMessage,
  hasInset = false,
  isOpenInitial = true,
  trailing,
  isOpen,
  onToggle,
  children,
}) => {
  return (
    <Widget
      isCollapsible
      icon={icon}
      header={header}
      variant={WidgetVariant.SECONDARY}
      isFlush={isEmpty || !hasInset}
      defaultIsOpen={isOpenInitial}
      isOpen={isOpen}
      onOpenChange={onToggle}
      actions={trailing}
    >
      <Flex
        alignItems={AlignItems.STRETCH}
        direction={FlexDirection.COLUMN}
        padding={isEmpty ? 24 : undefined}
        fillWidth
      >
        {isEmpty ? (
          <EmptyLayout
            size={EmptyLayoutSize.SMALL}
            header={emptyHeader}
            description={emptyMessage}
          />
        ) : (
          children
        )}
      </Flex>
    </Widget>
  );
};

export default PipelineCanvasPanelSection;
