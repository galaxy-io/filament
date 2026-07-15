import { type ReactNode } from "react";

import { motion } from "framer-motion";

interface ToastWrapperProps {
  children: ReactNode;
}

const ToastWrapper = ({ children }: ToastWrapperProps) => {
  return (
    <motion.div
      initial={{ opacity: 0, x: 50 }}
      animate={{ opacity: 1, x: 0 }}
      exit={{ opacity: 0, x: 50 }}
      transition={{ type: "tween", duration: 0.15, ease: "easeOut" }}
    >
      {children}
    </motion.div>
  );
};

export default ToastWrapper;
