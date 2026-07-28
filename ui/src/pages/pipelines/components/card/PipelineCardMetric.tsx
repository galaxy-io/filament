import type { PropsWithChildren } from "react";

import { styled } from "@linaria/react";

import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

const PipelineCardMetricWrapper = styled.div<{ $width: number }>`
  width: ${({ $width }) => $width}px;

  display: flex;
  align-items: center;
  gap: 8px;
`;

interface PipelineCardMetricProps {
  width: number;
  label?: string;
  value?: string;
}

const PipelineCardMetric = ({
  width,
  label,
  value,
  children,
}: PropsWithChildren<PipelineCardMetricProps>) => {
  return (
    <PipelineCardMetricWrapper $width={width}>
      {label && (
        <Text size={TextSize.CAPTION} variant={TextVariant.SECONDARY}>
          {label}
        </Text>
      )}
      {value !== undefined && (
        <Text size={TextSize.BODY_SM} isMonospace>
          {value}
        </Text>
      )}
      {children}
    </PipelineCardMetricWrapper>
  );
};

export default PipelineCardMetric;
