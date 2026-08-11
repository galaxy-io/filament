import { useEffect } from "react";

import { styled } from "@linaria/react";
import { useInView } from "react-intersection-observer";

const SentinelWrapper = styled.div`
  width: 100%;
  height: 1px;
  flex-shrink: 0;
`;

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

  return <SentinelWrapper ref={ref} />;
};

export default InfiniteScrollSentinel;
