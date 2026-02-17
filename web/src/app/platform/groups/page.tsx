"use client";

import { useCallback, useEffect, useState } from "react";
import { CheckCircle2, RefreshCw, XCircle } from "lucide-react";
import { toast } from "sonner";
import {
  PageContent,
  PageHeader,
  PageShell,
} from "@/components/merchant/layout/page-shell";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Textarea } from "@/components/ui/textarea";
import { apiGet, apiPost } from "@/lib/api";
import type {
  AdminGroupApplication,
  AdminGroupApplicationsResponse,
} from "@/types/platform-admin";

const PAGE_SIZE = 20;

function statusTag(status: string) {
  if (status === "submitted") return <Badge>待审核</Badge>;
  if (status === "approved") return <Badge variant="secondary">已通过</Badge>;
  if (status === "rejected") return <Badge variant="destructive">已驳回</Badge>;
  if (status === "draft") return <Badge variant="outline">草稿</Badge>;
  return <Badge variant="outline">{status}</Badge>;
}

export default function PlatformGroupApplicationsPage() {
  const [loading, setLoading] = useState(true);
  const [submittingId, setSubmittingId] = useState<number | null>(null);
  const [applications, setApplications] = useState<AdminGroupApplication[]>([]);
  const [status, setStatus] = useState<string>("submitted");
  const [rejectReasonMap, setRejectReasonMap] = useState<Record<number, string>>({});

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const response = await apiGet<AdminGroupApplicationsResponse>(
        "/admin/groups/applications",
        { page: 1, limit: PAGE_SIZE, status }
      );
      setApplications(response.applications ?? []);
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "加载集团申请失败";
      toast.error(message);
    } finally {
      setLoading(false);
    }
  }, [status]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  const review = async (id: number, nextStatus: "approved" | "rejected") => {
    if (nextStatus === "rejected") {
      const reason = (rejectReasonMap[id] || "").trim();
      if (reason.length === 0) {
        toast.warning("驳回时请填写原因");
        return;
      }
    }

    setSubmittingId(id);
    try {
      await apiPost(`/admin/groups/applications/${id}/review`, {
        status: nextStatus,
        reject_reason: nextStatus === "rejected" ? rejectReasonMap[id]?.trim() : undefined,
      });
      toast.success(nextStatus === "approved" ? "审核通过" : "已驳回申请");
      await loadData();
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "审核失败";
      toast.error(message);
    } finally {
      setSubmittingId(null);
    }
  };

  return (
    <PageShell>
      <PageHeader
        title="集团申请审核"
        description="平台管理员审核集团入驻申请"
        actions={
          <div className="flex items-center gap-2">
            <Select value={status} onValueChange={setStatus}>
              <SelectTrigger className="w-44">
                <SelectValue placeholder="筛选状态" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="submitted">待审核</SelectItem>
                <SelectItem value="approved">已通过</SelectItem>
                <SelectItem value="rejected">已驳回</SelectItem>
                <SelectItem value="draft">草稿</SelectItem>
              </SelectContent>
            </Select>
            <Button variant="outline" onClick={loadData}>
              <RefreshCw className="mr-2 h-4 w-4" /> 刷新
            </Button>
          </div>
        }
      />

      <PageContent>
        <Card>
          <CardHeader>
            <CardTitle>申请列表</CardTitle>
            <CardDescription>仅展示与筛选状态匹配的集团申请</CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>ID</TableHead>
                  <TableHead>集团名称</TableHead>
                  <TableHead>申请人</TableHead>
                  <TableHead>联系电话</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>创建时间</TableHead>
                  <TableHead className="w-65">驳回原因</TableHead>
                  <TableHead className="text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {loading ? (
                  Array.from({ length: 6 }).map((_, index) => (
                    <TableRow key={index}>
                      <TableCell colSpan={8}><Skeleton className="h-5 w-full" /></TableCell>
                    </TableRow>
                  ))
                ) : applications.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={8} className="text-center text-muted-foreground">暂无申请数据</TableCell>
                  </TableRow>
                ) : (
                  applications.map((item) => (
                    <TableRow key={item.id}>
                      <TableCell>{item.id}</TableCell>
                      <TableCell>{item.group_name}</TableCell>
                      <TableCell>{item.applicant_user_id}</TableCell>
                      <TableCell>{item.contact_phone}</TableCell>
                      <TableCell>{statusTag(item.status)}</TableCell>
                      <TableCell>{new Date(item.created_at).toLocaleString("zh-CN")}</TableCell>
                      <TableCell>
                        {item.status === "submitted" ? (
                          <div className="space-y-2">
                            <Label className="text-xs text-muted-foreground">仅驳回时必填</Label>
                            <Textarea
                              value={rejectReasonMap[item.id] || ""}
                              onChange={(event) =>
                                setRejectReasonMap((prev) => ({ ...prev, [item.id]: event.target.value }))
                              }
                              placeholder="填写驳回原因"
                              maxLength={200}
                            />
                          </div>
                        ) : (
                          <span className="text-sm text-muted-foreground">{item.reject_reason || "-"}</span>
                        )}
                      </TableCell>
                      <TableCell className="text-right">
                        {item.status === "submitted" ? (
                          <div className="flex justify-end gap-2">
                            <Button
                              size="sm"
                              onClick={() => review(item.id, "approved")}
                              disabled={submittingId === item.id}
                            >
                              <CheckCircle2 className="mr-1 h-4 w-4" /> 通过
                            </Button>
                            <Button
                              size="sm"
                              variant="destructive"
                              onClick={() => review(item.id, "rejected")}
                              disabled={submittingId === item.id}
                            >
                              <XCircle className="mr-1 h-4 w-4" /> 驳回
                            </Button>
                          </div>
                        ) : (
                          <span className="text-sm text-muted-foreground">已处理</span>
                        )}
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </PageContent>
    </PageShell>
  );
}
