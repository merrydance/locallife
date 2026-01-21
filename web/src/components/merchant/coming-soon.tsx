import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

export function ComingSoon({
  title,
  description,
}: {
  title: string;
  description: string;
}) {
  return (
    <>
      <header className="page-header">
        <div>
          <h1 className="text-xl font-semibold">{title}</h1>
          <p className="text-sm text-muted-foreground">{description}</p>
        </div>
      </header>
      <div className="page-content">
        <div className="flex flex-1 items-center justify-center">
          <Card className="panel w-full max-w-xl">
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
      </div>
    </>
  );
}
