import { MerchantSidebar } from "@/components/merchant/sidebar";

export default function MerchantLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="min-h-screen bg-background text-foreground">
      <div className="flex min-h-screen">
        <MerchantSidebar />
        <div className="flex flex-1 flex-col">{children}</div>
      </div>
    </div>
  );
}
