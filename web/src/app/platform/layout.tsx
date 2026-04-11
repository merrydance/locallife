import { PlatformLayoutClient } from "@/components/platform/platform-layout-client";
import { PlatformAccessGate } from "@/components/platform/platform-access-gate";
import { PlatformSessionProvider } from "@/components/providers/platform-session-provider";

export default function PlatformLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <PlatformSessionProvider>
      <PlatformAccessGate>
        <PlatformLayoutClient>{children}</PlatformLayoutClient>
      </PlatformAccessGate>
    </PlatformSessionProvider>
  );
}
