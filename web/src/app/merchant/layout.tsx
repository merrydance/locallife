import { MerchantLayoutClient } from "@/components/merchant/merchant-layout-client";

export default function MerchantLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return <MerchantLayoutClient>{children}</MerchantLayoutClient>;
}
