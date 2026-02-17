import { MerchantLayoutClient } from "@/components/merchant/merchant-layout-client";
import { MerchantSessionProvider } from "@/components/providers/merchant-session-provider";

export default function MerchantLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <MerchantSessionProvider>
      <MerchantLayoutClient>{children}</MerchantLayoutClient>
    </MerchantSessionProvider>
  );
}
