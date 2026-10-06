import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import Text, { TextWeight } from "@galaxy-io/dls/text/Text";

interface CreatePipelineModalDeliverySectionProps {
  header: string;
  children: React.ReactNode;
}

const CreatePipelineModalDeliverySection = ({
  header,
  children,
}: CreatePipelineModalDeliverySectionProps) => (
  <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={12} fillWidth>
    <Text weight={TextWeight.MEDIUM}>{header}</Text>
    {children}
  </Flex>
);

export default CreatePipelineModalDeliverySection;
