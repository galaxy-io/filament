import { ConnectorKind, ExecutionMode } from "@/gen/ingestion/v1/common_pb";
import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";
import type { WebhookEndpointConfig } from "@/gen/ingestion/v1/webhooks_pb";

export interface ConnectionDrawerWebhookRoute {
  id: string;
  label: string;
  pipelineId: string;
  routeKey: string;
  resources: string[];
}

export interface ConnectionDrawerWebhookState {
  routeId: string;
  enabled: boolean;
  secretChoice: "keep" | "mint" | "import";
  importedSecret: string;
}

export const createConnectionDrawerWebhookState = (
  endpoint?: WebhookEndpointConfig,
): ConnectionDrawerWebhookState => ({
  routeId:
    endpoint?.pipelineId && endpoint.routeKey ? `${endpoint.pipelineId}|${endpoint.routeKey}` : "",
  enabled: endpoint?.enabled ?? false,
  secretChoice: endpoint?.hasSigningSecret ? "keep" : "mint",
  importedSecret: "",
});

export const connectionDrawerWebhookRoutes = (
  pipelines: Pipeline[],
  connectionId: string,
): ConnectionDrawerWebhookRoute[] => {
  const routes: ConnectionDrawerWebhookRoute[] = [];
  for (const pipeline of pipelines) {
    if (pipeline.executionMode !== ExecutionMode.CONTINUOUS) continue;
    const graph = pipeline.currentVersion?.graph;
    if (!graph) continue;
    const nodes = new Map(graph.nodes.map((node) => [node.id, node]));
    const sourceNodes = new Set(
      graph.nodes.filter((node) => node.connectionId === connectionId).map((node) => node.id),
    );
    const destinations = graph.nodes.filter((node) => node.kind === ConnectorKind.SINK);
    for (const edge of graph.edges) {
      if (!sourceNodes.has(edge.fromNode)) continue;
      const sink = nodes.get(edge.toNode);
      if (sink?.kind !== ConnectorKind.SINK) continue;
      const routeKey = `route/${edge.fromNode}/${edge.toNode}`;
      const id = `${pipeline.id}|${routeKey}`;
      const existing = routes.find((route) => route.id === id);
      if (existing) {
        if (edge.resource && !existing.resources.includes(edge.resource)) {
          existing.resources.push(edge.resource);
          existing.resources.sort();
        }
        continue;
      }
      const destinationNumber = destinations.findIndex((node) => node.id === sink.id) + 1;
      routes.push({
        id,
        label: `${pipeline.name} · destination ${destinationNumber}`,
        pipelineId: pipeline.id,
        routeKey,
        resources: edge.resource ? [edge.resource] : [],
      });
    }
  }
  return routes;
};
