import {
  createContext,
  PropsWithChildren,
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";
import { createPortal } from "react-dom";

import { AnimatePresence } from "framer-motion";

import Toast, { ToastVariant, type ToastProps } from "@/providers/toast/Toast";
import ToastContainer from "@/providers/toast/ToastContainer";
import ToastWrapper from "@/providers/toast/ToastWrapper";

interface ToastState {
  id: string;
  props: ToastProps;
  timeout: number;
  top?: number | string;
  left?: number | string;
  right?: number | string;
  bottom?: number | string;
}

export type ShowToastProps = Omit<ToastProps, "onClose"> & {
  timeout?: number;
  top?: number | string;
  left?: number | string;
  right?: number | string;
  bottom?: number | string;
};

export interface ToastContextProps {
  showToast: (options: ShowToastProps) => string;
  hideToast: (id?: string) => void;
}

export const ToastContext = createContext<ToastContextProps | undefined>(
  undefined,
);

const DEFAULT_TIMEOUT = 3000;

export const ToastProvider = ({ children }: PropsWithChildren) => {
  const [toasts, setToasts] = useState<ToastState[]>([]);
  const timeoutRefs = useRef<Map<string, ReturnType<typeof setTimeout>>>(
    new Map(),
  );

  const showToast = useCallback((options: ShowToastProps) => {
    const id = crypto.randomUUID();
    const { timeout, top, right, bottom, left, ...props } = options;
    const finalTimeout = timeout === undefined ? DEFAULT_TIMEOUT : timeout;
    setToasts((prev) => [
      ...prev,
      { id, props, timeout: finalTimeout, top, right, bottom, left },
    ]);
    return id;
  }, []);

  const hideToast = useCallback((id?: string) => {
    if (id) {
      const timeoutId = timeoutRefs.current.get(id);
      if (timeoutId) {
        clearTimeout(timeoutId);
        timeoutRefs.current.delete(id);
      }
      setToasts((prev) => prev.filter((toast) => toast.id !== id));
    } else {
      setToasts((prev) => {
        const firstToast = prev[0];
        if (firstToast) {
          const timeoutId = timeoutRefs.current.get(firstToast.id);
          if (timeoutId) {
            clearTimeout(timeoutId);
            timeoutRefs.current.delete(firstToast.id);
          }
        }
        return prev.slice(1);
      });
    }
  }, []);

  useEffect(() => {
    const currentToastIds = new Set(toasts.map((toast) => toast.id));

    toasts.forEach((toast) => {
      if (timeoutRefs.current.has(toast.id)) {
        return;
      }
      if (toast.timeout > 0) {
        const timeoutId = setTimeout(() => {
          setToasts((prev) => prev.filter((t) => t.id !== toast.id));
          timeoutRefs.current.delete(toast.id);
        }, toast.timeout);
        timeoutRefs.current.set(toast.id, timeoutId);
      }
    });

    timeoutRefs.current.forEach((timeoutId, toastId) => {
      if (!currentToastIds.has(toastId)) {
        clearTimeout(timeoutId);
        timeoutRefs.current.delete(toastId);
      }
    });
  }, [toasts]);

  useEffect(() => {
    const refs = timeoutRefs.current;
    return () => {
      refs.forEach((timeoutId) => {
        clearTimeout(timeoutId);
      });
      refs.clear();
    };
  }, []);

  const toastContent = (
    <ToastContainer
      top={toasts[0]?.top}
      right={toasts[0]?.right}
      bottom={toasts[0]?.bottom}
      left={toasts[0]?.left}
    >
      <AnimatePresence>
        {toasts.map((toast) => (
          <ToastWrapper key={toast.id}>
            <Toast {...toast.props} onClose={() => hideToast(toast.id)} />
          </ToastWrapper>
        ))}
      </AnimatePresence>
    </ToastContainer>
  );

  return (
    <ToastContext.Provider value={{ showToast, hideToast }}>
      {children}
      {typeof document !== "undefined" &&
        createPortal(toastContent, document.body)}
    </ToastContext.Provider>
  );
};

export { ToastVariant };
