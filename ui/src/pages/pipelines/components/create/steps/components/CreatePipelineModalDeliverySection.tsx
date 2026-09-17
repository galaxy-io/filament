import FlexWrapper, { FlexDirection, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextWeight } from "@galaxy-io/dls/text/Text";

interface CreatePipelineModalDeliverySectionProps {
  header: string;
  children: React.ReactNode;
}

const CreatePipelineModalDeliverySection = ({
  header,
  children,
}: CreatePipelineModalDeliverySectionProps) => (
  <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.MEDIUM} fillWidth>
    <Text weight={TextWeight.MEDIUM}>{header}</Text>
    {children}
  </FlexWrapper>
);

export default CreatePipelineModalDeliverySection;
