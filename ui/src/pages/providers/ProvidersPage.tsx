import { create } from "@bufbuild/protobuf";

import FlexWrapper, { AlignItems, FlexDirection, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";

import { ProviderKind } from "@/gen/ingestion/v1/common_pb";
import { ListProvidersRequestSchema } from "@/gen/ingestion/v1/providers_pb";

import PageLayout from "@/layouts/PageLayout";

import ProviderTile from "@/pages/pipelines/components/ProviderTile";

import { useListProvidersQuery } from "@/api/queries/providers";

interface ProvidersPageProps {
  kind: ProviderKind;
}

/**
 * Catalog of registered providers, filtered to sources or sinks. Backs both
 * the /sources and /sinks tabs until they get dedicated designs.
 */
const ProvidersPage = ({ kind }: ProvidersPageProps) => {
  const { data, isLoading, isError } = useListProvidersQuery({
    input: create(ListProvidersRequestSchema, { kind }),
  });

  const title = kind === ProviderKind.SOURCE ? "Sources" : "Sinks";

  return (
    <PageLayout
      title={title}
      isLoading={isLoading}
      isError={isError}
      isEmpty={!data?.providers.length}
      emptyMessage={`No ${title.toLowerCase()} registered.`}
    >
      <FlexWrapper fillWidth direction={FlexDirection.COLUMN} gap={FlexGap.MEDIUM}>
        {data?.providers.map((provider) => (
          <FlexWrapper
            key={`${provider.kind}:${provider.name}`}
            fillWidth
            alignItems={AlignItems.CENTER}
            gap={FlexGap.SMALL}
          >
            <ProviderTile provider={provider.name} />
            <Text size={TextSize.BODY_MD} weight={TextWeight.MEDIUM}>
              {provider.displayName || provider.name}
            </Text>
            {provider.version && (
              <Text size={TextSize.CAPTION} variant={TextVariant.TERTIARY} isMonospace>
                v{provider.version}
              </Text>
            )}
            <Text size={TextSize.CAPTION} variant={TextVariant.SECONDARY}>
              {provider.configSchema?.fields.length ?? 0} config field
              {(provider.configSchema?.fields.length ?? 0) === 1 ? "" : "s"}
            </Text>
          </FlexWrapper>
        ))}
      </FlexWrapper>
    </PageLayout>
  );
};

export default ProvidersPage;
