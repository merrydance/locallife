import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { PageShell, PageHeader, PageContent } from "@/components/merchant/layout/page-shell";

export function ComingSoon({
  title,
  description,
}: {
  title: string;
  description: string;
}) {
  return (
    <PageShell>
      <PageHeader title={title} description={description} />
      <PageContent>
        <div className="flex flex-1 items-center justify-center py-12">
          <Card className="w-full max-w-xl shadow-sm">
            <CardHeader>
              <CardTitle>{title}</CardTitle>
              <CardDescription>{description}</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4 text-sm text-muted-foreground">
              <p>该模块正在按小程序布局对齐与后端字段对齐中。</p>
              <Button variant="outline" asChild>
                <a href="/merchant/dashboard">返回工作台</a>
              </Button>
            </CardContent>
          </Card>
        </div>
      </PageContent>
    </PageShell>
  );
}
