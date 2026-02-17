import { Suspense } from "react";
import { WebLoginPageClient } from "@/components/auth/web-login-page-client";

export default function UnifiedLoginPage() {
  return (
    <Suspense
      fallback={<div className="min-h-screen flex items-center justify-center">Loading...</div>}
    >
      <WebLoginPageClient />
    </Suspense>
  );
}
