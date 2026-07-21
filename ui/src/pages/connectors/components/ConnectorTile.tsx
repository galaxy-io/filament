import type { ComponentType } from "react";

import { styled } from "@linaria/react";
import { XIcon } from "@phosphor-icons/react";
import { match } from "ts-pattern";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import GithubLogomark from "@galaxy-io/dls/icons/sources/GithubLogomark";
import GoogleBigqueryLogomark from "@galaxy-io/dls/icons/sources/GoogleBigqueryLogomark";
import HubspotLogomark from "@galaxy-io/dls/icons/sources/HubspotLogomark";
import JiraLogomark from "@galaxy-io/dls/icons/sources/JiraLogomark";
import LinearLogomark from "@galaxy-io/dls/icons/sources/LinearLogomark";
import MySQLLogomark from "@galaxy-io/dls/icons/sources/MySQLLogomark";
import NotionLogomark from "@galaxy-io/dls/icons/sources/NotionLogomark";
import PostgresLogomark from "@galaxy-io/dls/icons/sources/PostgresLogomark";
import S3Logomark from "@galaxy-io/dls/icons/sources/S3Logomark";
import SalesforceLogomark from "@galaxy-io/dls/icons/sources/SalesforceLogomark";
import SlackLogoIconmark from "@galaxy-io/dls/icons/sources/SlackLogoIconmark";
import SnowflakeLogomark from "@galaxy-io/dls/icons/sources/SnowflakeLogomark";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

export enum ConnectorTileSize {
  SMALL = "SMALL",
  MEDIUM = "MEDIUM",
  LARGE = "LARGE",
}

const CONNECTOR_TO_LOGOMARK_MAP: Record<string, ComponentType<{ height: number }>> = {
  bigquery: GoogleBigqueryLogomark,
  github: GithubLogomark,
  hubspot: HubspotLogomark,
  jira: JiraLogomark,
  linear: LinearLogomark,
  mysql: MySQLLogomark,
  notion: NotionLogomark,
  postgres: PostgresLogomark,
  s3: S3Logomark,
  object: S3Logomark,
  salesforce: SalesforceLogomark,
  slack: SlackLogoIconmark,
  snowflake: SnowflakeLogomark,
};

const getTileSize = (size: ConnectorTileSize): number =>
  match(size)
    .with(ConnectorTileSize.SMALL, () => 24)
    .with(ConnectorTileSize.MEDIUM, () => 32)
    .with(ConnectorTileSize.LARGE, () => 40)
    .exhaustive();

const getTileRadius = (size: ConnectorTileSize): number =>
  match(size)
    .with(ConnectorTileSize.SMALL, () => 4)
    .with(ConnectorTileSize.MEDIUM, () => 5)
    .with(ConnectorTileSize.LARGE, () => 6)
    .exhaustive();

const getLogoHeight = (size: ConnectorTileSize): number =>
  match(size)
    .with(ConnectorTileSize.SMALL, () => 16)
    .with(ConnectorTileSize.MEDIUM, () => 20)
    .with(ConnectorTileSize.LARGE, () => 24)
    .exhaustive();

const getTextSize = (size: ConnectorTileSize): TextSize =>
  match(size)
    .with(ConnectorTileSize.SMALL, () => TextSize.CAPTION)
    .with(ConnectorTileSize.MEDIUM, () => TextSize.BODY_MD)
    .with(ConnectorTileSize.LARGE, () => TextSize.BODY_LG)
    .exhaustive();

const TileWrapper = withTheme(styled.div<
  PropsWithTheme<{ $size: ConnectorTileSize; $isClickable: boolean }>
>`
  width: ${({ $size }) => getTileSize($size)}px;
  height: ${({ $size }) => getTileSize($size)}px;

  display: flex;
  align-items: center;
  justify-content: center;

  background-color: ${({ theme }) => theme.color.background.secondary};

  border-radius: ${({ $size }) => getTileRadius($size)}px;

  overflow: hidden;

  cursor: ${({ $isClickable }) => ($isClickable ? "pointer" : "inherit")};

  transition: opacity 100ms ease;

  &:hover {
    opacity: ${({ $isClickable }) => ($isClickable ? 0.8 : 1)};
  }
`);

const EmptyTileWrapper = withTheme(styled.div<PropsWithTheme<{ $size: ConnectorTileSize }>>`
  width: ${({ $size }) => getTileSize($size)}px;
  height: ${({ $size }) => getTileSize($size)}px;

  display: flex;
  align-items: center;
  justify-content: center;

  background-color: transparent;

  border: 0.5px solid ${({ theme }) => theme.color.border.error};
  border-radius: ${({ $size }) => getTileRadius($size)}px;

  overflow: hidden;
`);

interface EmptyConnectorTileProps {
  size?: ConnectorTileSize;
}

export const ConnectorTileEmpty = ({
  size = ConnectorTileSize.MEDIUM,
}: EmptyConnectorTileProps) => {
  return (
    <EmptyTileWrapper $size={size}>
      <Icon component={XIcon} size={10} variant={IconVariant.ERROR} />
    </EmptyTileWrapper>
  );
};

interface ConnectorTileProps {
  connector: string;
  size?: ConnectorTileSize;
  onClick?: (e: React.MouseEvent) => void;
}

const ConnectorTile = ({
  connector,
  size = ConnectorTileSize.MEDIUM,
  onClick,
}: ConnectorTileProps) => {
  const Logomark = CONNECTOR_TO_LOGOMARK_MAP[connector.toLowerCase()];

  return (
    <TileWrapper $size={size} $isClickable={!!onClick} onClick={onClick}>
      {Logomark ? (
        <Logomark height={getLogoHeight(size)} />
      ) : (
        <Text size={getTextSize(size)} variant={TextVariant.SECONDARY} isMonospace>
          {connector.charAt(0).toUpperCase()}
        </Text>
      )}
    </TileWrapper>
  );
};

export const ConnectorOverflowTile = ({ count }: { count: number }) => {
  return (
    <TileWrapper $size={ConnectorTileSize.SMALL} $isClickable={false}>
      <Text size={TextSize.CAPTION} variant={TextVariant.SECONDARY} isMonospace>
        +{count}
      </Text>
    </TileWrapper>
  );
};

export default ConnectorTile;
