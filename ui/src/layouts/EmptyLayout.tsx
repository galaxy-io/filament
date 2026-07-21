import FlexWrapper, {
  AlignItems,
  FlexDirection,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Text, {
  TextSize,
  TextVariant,
  TextWeight,
} from "@galaxy-io/dls/text/Text";

export enum EmptyLayoutSize {
  SMALL = "SMALL",
  MEDIUM = "MEDIUM",
  LARGE = "LARGE",
}

interface EmptyLayoutProps {
  size?: EmptyLayoutSize;
  icon?: React.ReactNode;
  header?: string;
  message?: string;
  actions?: React.ReactNode;
}

const EMPTY_LAYOUT_SIZE_TO_GAP_MAP: Record<EmptyLayoutSize, number> = {
  [EmptyLayoutSize.SMALL]: 16,
  [EmptyLayoutSize.MEDIUM]: 24,
  [EmptyLayoutSize.LARGE]: 32,
};

const EMPTY_LAYOUT_SIZE_TO_HEADER_SIZE_MAP: Record<EmptyLayoutSize, TextSize> =
  {
    [EmptyLayoutSize.SMALL]: TextSize.BODY_MD,
    [EmptyLayoutSize.MEDIUM]: TextSize.BODY_LG,
    [EmptyLayoutSize.LARGE]: TextSize.HEADING_SM,
  };

const EMPTY_LAYOUT_SIZE_TO_MESSAGE_SIZE_MAP: Record<EmptyLayoutSize, TextSize> =
  {
    [EmptyLayoutSize.SMALL]: TextSize.BODY_SM,
    [EmptyLayoutSize.MEDIUM]: TextSize.BODY_MD,
    [EmptyLayoutSize.LARGE]: TextSize.BODY_LG,
  };

const EmptyLayout = ({
  size = EmptyLayoutSize.MEDIUM,
  icon,
  header,
  message,
  actions,
}: EmptyLayoutProps) => {
  return (
    <FlexWrapper
      fillWidth
      fillHeight
      direction={FlexDirection.COLUMN}
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.CENTER}
      gap={EMPTY_LAYOUT_SIZE_TO_GAP_MAP[size]}
    >
      {icon && icon}
      <FlexWrapper
        direction={FlexDirection.COLUMN}
        alignItems={AlignItems.CENTER}
        gap={8}
      >
        {header && (
          <Text
            size={EMPTY_LAYOUT_SIZE_TO_HEADER_SIZE_MAP[size]}
            weight={TextWeight.MEDIUM}
          >
            {header}
          </Text>
        )}
        {message && (
          <Text
            size={EMPTY_LAYOUT_SIZE_TO_MESSAGE_SIZE_MAP[size]}
            variant={TextVariant.SECONDARY}
          >
            {message}
          </Text>
        )}
      </FlexWrapper>
      {actions && actions}
    </FlexWrapper>
  );
};

export default EmptyLayout;
