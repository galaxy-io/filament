import { useEffect } from "react";

import { useInView } from "react-intersection-observer";

import FlexItem from "@galaxy-io/dls/layout/FlexItem";

interface InfiniteScrollSentinelProps {
  hasNextPage: boolean;
  isFetchingNextPage: boolean;
  fetchNextPage: () => void;
}

const InfiniteScrollSentinel = ({
  hasNextPage,
  isFetchingNextPage,
  fetchNextPage,
}: InfiniteScrollSentinelProps) => {
  const { ref, inView } = useInView();

  useEffect(() => {
    if (inView && hasNextPage && !isFetchingNextPage) {
      fetchNextPage();
    }
  }, [inView, hasNextPage, isFetchingNextPage, fetchNextPage]);

  if (!hasNextPage) {
    return null;
  }

  return <FlexItem ref={ref} shrink={0} fillWidth height={1} />;
};

export default InfiniteScrollSentinel;
