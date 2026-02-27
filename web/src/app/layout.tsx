import type { Metadata } from "next";
import { Toaster } from "@/components/ui/sonner";
import { WebErrorReporter } from "@/components/providers/web-error-reporter";
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
        <WebErrorReporter />
        {children}
        <Toaster />
      </body>
    </html>
  );
}
