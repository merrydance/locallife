export default function AnalyticsLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  // 直接渲染子内容，页面组件自带完整布局
  return <>{children}</>;
}
