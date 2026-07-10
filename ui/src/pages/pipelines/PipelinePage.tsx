import { useParams } from "@tanstack/react-router";

const PipelinePage = () => {
  const { id } = useParams({ from: "/pipelines/$id" });

  return <div>Pipeline: {id}</div>;
};

export default PipelinePage;
