import type { PropsWithChildren, ReactNode } from "react";

import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import type { SpaceShorthand } from "@galaxy-io/dls/theme/enums";
import Widget, { WidgetVariant } from "@galaxy-io/dls/widget/Widget";

import EmptyLayout from "@/layouts/EmptyLayout";
import { LayoutSize } from "@/layouts/types";

interface PipelineCanvasPanelSectionProps {
  header: string;
  isEmpty: boolean;
  emptyHeader: string;
  emptyMessage: string;
  padding?: SpaceShorthand;
  isOpenInitial?: boolean;
  trailing?: ReactNode;
  isOpen?: boolean;
  onToggle?: () => void;
}

const PipelineCanvasPanelSection = ({
  header,
  isEmpty,
  emptyHeader,
  emptyMessage,
  padding = 0,
  isOpenInitial = true,
  trailing,
  isOpen,
  onToggle,
  children,
}: PropsWithChildren<PipelineCanvasPanelSectionProps>) => {
  return (
    <Widget
      isCollapsible
      header={header}
      variant={WidgetVariant.SECONDARY}
      isFlush
      defaultIsOpen={isOpenInitial}
      isOpen={isOpen}
      onOpenChange={onToggle}
      actions={trailing}
    >
      <Flex
        alignItems={AlignItems.STRETCH}
        direction={FlexDirection.COLUMN}
        padding={isEmpty ? 24 : padding}
        fillWidth
      >
        {isEmpty ? (
          <EmptyLayout size={LayoutSize.SMALL} header={emptyHeader} message={emptyMessage} />
        ) : (
          children
        )}
      </Flex>
    </Widget>
  );
};

export default PipelineCanvasPanelSection;
