import type { Metadata } from "next";
import { MerchantSessionProvider } from "@/components/providers/merchant-session-provider";
import "./globals.css";

export const metadata: Metadata = {
  title: "本地生活商户后台",
  description: "商户侧 Web 管理后台",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="zh-CN">
      <body className="antialiased">
        <MerchantSessionProvider>{children}</MerchantSessionProvider>
      </body>
    </html>
  );
}
