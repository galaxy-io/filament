import Icon, { IconVariant, IconWeight } from "@galaxy-io/dls/icons/Icon";
import { FlowArrowIcon, XIcon } from "@phosphor-icons/react";

const PipelineIcon = ({
  isLinked,
  size,
}: {
  isLinked: boolean;
  size: number;
}) => {
  return (
    <Icon
      component={isLinked ? FlowArrowIcon : XIcon}
      size={size}
      variant={isLinked ? IconVariant.PRIMARY : IconVariant.ERROR}
      weight={IconWeight.REGULAR}
    />
  );
};

export default PipelineIcon;
