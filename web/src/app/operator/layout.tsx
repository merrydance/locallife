import { OperatorLayoutClient } from "@/components/operator/operator-layout-client";
import { OperatorAccessGate } from "@/components/operator/operator-access-gate";
import { OperatorSessionProvider } from "@/components/providers/operator-session-provider";

export default function OperatorLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <OperatorSessionProvider>
      <OperatorAccessGate>
        <OperatorLayoutClient>{children}</OperatorLayoutClient>
      </OperatorAccessGate>
    </OperatorSessionProvider>
  );
}
