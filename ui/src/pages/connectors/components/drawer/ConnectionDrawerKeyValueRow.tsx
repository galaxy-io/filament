import FlexWrapper, {
  AlignItems,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextWeight } from "@galaxy-io/dls/text/Text";

interface ConnectionDrawerKeyValueRowProps {
  label: string;
  value: React.ReactNode;
}

const ConnectionDrawerKeyValueRow = ({ label, value }: ConnectionDrawerKeyValueRowProps) => {
  return (
    <FlexWrapper
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.SPACE_BETWEEN}
      gap={FlexGap.SMALL}
      fillWidth
    >
      <Text weight={TextWeight.MEDIUM}>{label}</Text>
      {value}
    </FlexWrapper>
  );
};

export default ConnectionDrawerKeyValueRow;
