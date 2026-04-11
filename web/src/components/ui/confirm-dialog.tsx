"use client";

import * as React from "react";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";

interface ConfirmDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description?: React.ReactNode;
  confirmText?: string;
  cancelText?: string;
  variant?: "default" | "destructive";
  onConfirm: () => void | Promise<void>;
}

export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  confirmText = "确认",
  cancelText = "取消",
  variant = "default",
  onConfirm,
}: ConfirmDialogProps) {
  const [loading, setLoading] = React.useState(false);

  const handleConfirm = async () => {
    setLoading(true);
    try {
      await onConfirm();
      onOpenChange(false);
    } finally {
      setLoading(false);
    }
  };

  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{title}</AlertDialogTitle>
          {description && (
            <AlertDialogDescription>{description}</AlertDialogDescription>
          )}
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={loading}>{cancelText}</AlertDialogCancel>
          <AlertDialogAction
            onClick={(e) => {
              e.preventDefault();
              handleConfirm();
            }}
            disabled={loading}
            className={variant === "destructive" ? "bg-destructive text-destructive-foreground hover:bg-destructive/90" : ""}
          >
            {loading ? "处理中..." : confirmText}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

// Hook for easier usage with async confirm
interface UseConfirmOptions {
  title: string;
  description?: React.ReactNode;
  confirmText?: string;
  cancelText?: string;
  variant?: "default" | "destructive";
}

type ConfirmFunction = (options?: Partial<UseConfirmOptions>) => Promise<boolean>;

interface UseConfirmReturn {
  confirm: ConfirmFunction;
  ConfirmDialogComponent: React.FC;
}

export function useConfirm(defaultOptions: UseConfirmOptions): UseConfirmReturn {
  const [state, setState] = React.useState<{
    open: boolean;
    options: UseConfirmOptions;
  }>({
    open: false,
    options: defaultOptions,
  });
  const resolveRef = React.useRef<((value: boolean) => void) | null>(null);

  const confirm: ConfirmFunction = React.useCallback((overrideOptions = {}) => {
    return new Promise<boolean>((resolve) => {
      resolveRef.current = resolve;
      setState({
        open: true,
        options: { ...defaultOptions, ...overrideOptions },
      });
    });
  }, [defaultOptions]);

  const handleOpenChange = React.useCallback((open: boolean) => {
    if (!open && resolveRef.current) {
      resolveRef.current(false);
    }
    if (!open) {
      resolveRef.current = null;
    }
    setState((prev) => ({ ...prev, open }));
  }, []);

  const handleConfirm = React.useCallback(() => {
    if (resolveRef.current) {
      resolveRef.current(true);
    }
    resolveRef.current = null;
    setState((prev) => ({ ...prev, open: false }));
  }, []);

  const ConfirmDialogComponent = React.useCallback(() => (
    <ConfirmDialog
      open={state.open}
      onOpenChange={handleOpenChange}
      title={state.options.title}
      description={state.options.description}
      confirmText={state.options.confirmText}
      cancelText={state.options.cancelText}
      variant={state.options.variant}
      onConfirm={handleConfirm}
    />
  ), [state.open, state.options, handleOpenChange, handleConfirm]);

  return { confirm, ConfirmDialogComponent };
}
