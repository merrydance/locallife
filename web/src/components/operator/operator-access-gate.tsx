"use client";

import Link from "next/link";
import { useMemo } from "react";
import { Button } from "@/components/ui/button";
import { useOperatorSession } from "@/components/providers/operator-session-provider";
import { cn } from "@/lib/utils";

const OPERATOR_REQUIRED_ROLES = ["operator"];

export function OperatorAccessGate({
  children,
}: {
  children: React.ReactNode;
}) {
  const session = useOperatorSession();

  const state = useMemo(() => {
    if (!session) return { mode: "loading" } as const;
    if (!session.isReady) return { mode: "loading" } as const;
    if (!session.isAuthenticated) return { mode: "login" } as const;
    if (!session.isAuthorized) return { mode: "forbidden" } as const;
    return { mode: "ready" } as const;
  }, [session]);

  if (state.mode === "ready") {
    return <>{children}</>;
  }

  return (
    <div className="flex min-h-[70vh] items-center justify-center px-6 py-10">
      <div className="w-full max-w-md space-y-6 rounded-lg border bg-card p-6 text-center shadow-sm">
        <div className="space-y-2">
          <h2 className="text-lg font-semibold">
            {state.mode === "login" ? "需要登录" : "访问受限"}
          </h2>
          <p className="text-sm text-muted-foreground">
            {state.mode === "login"
              ? "请先登录运营商账号后继续。"
              : "当前账号暂无运营商控制台权限。"}
          </p>
        </div>
        {state.mode === "forbidden" && (
          <div className="rounded-md bg-muted/50 px-4 py-3 text-left text-xs text-muted-foreground">
            需要角色：{OPERATOR_REQUIRED_ROLES.join(" / ")}
          </div>
        )}
        <div className={cn("flex flex-col gap-3", state.mode === "login" && "pt-2")}>
          <Button asChild>
            <Link href="/merchant/login">前往登录</Link>
          </Button>
          <Button variant="outline" asChild>
            <Link href="/">返回首页</Link>
          </Button>
        </div>
      </div>
    </div>
  );
}
