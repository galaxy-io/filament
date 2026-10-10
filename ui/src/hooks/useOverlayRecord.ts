import { useState } from "react";

export const useOverlayRecord = <TRecord extends string | number | undefined>(
  isOpen: boolean,
  record: TRecord,
) => {
  const [lastRecord, setLastRecord] = useState(record);
  if (isOpen && lastRecord !== record) {
    setLastRecord(record);
  }
  return isOpen ? record : lastRecord;
};
