import type { ComponentProps } from "react";

import { ArrowLeftIcon, TrashIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";

import BaseHeader, { BaseHeaderSize } from "@/layouts/components/BaseHeader";

interface PipelineCanvasPanelHeaderProps {
  title: string;
  description?: string;
  icon?: ComponentProps<typeof BaseHeader>["icon"];
  tile?: React.ReactNode;
  onBack?: () => void;
  onClose: () => void;
  onDelete?: () => void;
}

const PipelineCanvasPanelHeader = ({
  title,
  description,
  icon,
  tile,
  onBack,
  onClose,
  onDelete,
}: PipelineCanvasPanelHeaderProps) => (
  <>
    <Flex alignItems={AlignItems.CENTER} gap={8} padding={[8, 12]} shrink={0} fillWidth>
      {onBack && (
        <FlexItem shrink={0}>
          <Button
            icon={ArrowLeftIcon}
            variant={ButtonVariant.SECONDARY}
            size={ButtonSize.SMALL}
            onClick={onBack}
            ariaLabel="Back to configuration"
          />
        </FlexItem>
      )}
      {tile && <FlexItem shrink={0}>{tile}</FlexItem>}
      <FlexItem grow={1} minWidth={0}>
        <BaseHeader
          size={BaseHeaderSize.SMALL}
          title={title}
          description={description}
          icon={icon}
          actions={
            onDelete
              ? [
                  <Button
                    key="delete"
                    icon={TrashIcon}
                    variant={ButtonVariant.SECONDARY}
                    size={ButtonSize.SMALL}
                    onClick={onDelete}
                    ariaLabel="Delete"
                  />,
                ]
              : undefined
          }
          onClose={onClose}
        />
      </FlexItem>
    </Flex>
    <Divider />
  </>
);

export default PipelineCanvasPanelHeader;
