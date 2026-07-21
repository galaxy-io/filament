import FlexWrapper, {
  AlignItems,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextSize, TextWeight } from "@galaxy-io/dls/text/Text";

interface ConnectionDrawerKeyValueRowProps {
  label: string;
  value: React.ReactNode;
  isMonospace?: boolean;
}

const ConnectionDrawerKeyValueRow = ({
  label,
  value,
  isMonospace = false,
}: ConnectionDrawerKeyValueRowProps) => {
  return (
    <FlexWrapper
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.SPACE_BETWEEN}
      gap={FlexGap.SMALL}
      fillWidth
    >
      <Text weight={TextWeight.MEDIUM}>{label}</Text>
      {typeof value === "string" ? (
        <Text size={TextSize.BODY_SM} isMonospace={isMonospace}>
          {value}
        </Text>
      ) : (
        value
      )}
    </FlexWrapper>
  );
};

export default ConnectionDrawerKeyValueRow;
