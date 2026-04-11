"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Loader2, RefreshCw, CheckCircle2, XCircle } from "lucide-react";
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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
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
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { apiGet, apiPost } from "@/lib/api";
import type {
  AdminRegionExpansionApplication,
  AdminRegionExpansionApplicationsResponse,
} from "@/types/platform-admin";

const PAGE_SIZE = 20;

const STATUS_LABEL: Record<string, string> = {
  pending:  "待审核",
  approved: "已通过",
  rejected: "已驳回",
};

function statusBadge(status: string) {
  if (status === "pending") return <Badge>待审核</Badge>;
  if (status === "approved") return <Badge variant="secondary">已通过</Badge>;
  if (status === "rejected") return <Badge variant="destructive">已驳回</Badge>;
  return <Badge variant="outline">{STATUS_LABEL[status] ?? "未知状态"}</Badge>;
}

export default function PlatformRegionExpansionApplicationsPage() {
  const [loading, setLoading] = useState(true);
  const [isApproving, setIsApproving] = useState(false);
  const [isRejecting, setIsRejecting] = useState(false);
  const [rows, setRows] = useState<AdminRegionExpansionApplication[]>([]);
  const [total, setTotal] = useState(0);

  const [selected, setSelected] = useState<AdminRegionExpansionApplication | null>(null);
  const [rejectReason, setRejectReason] = useState("");
  const [rejectOpen, setRejectOpen] = useState(false);
  const [approveConfirmOpen, setApproveConfirmOpen] = useState(false);

  const loadList = useCallback(async () => {
    setLoading(true);
    try {
      const res = await apiGet<AdminRegionExpansionApplicationsResponse>(
        "/admin/operators/region-applications",
        { page: 1, limit: PAGE_SIZE }
      );
      setRows(res.applications ?? []);
      setTotal(res.total ?? 0);
    } catch (error: unknown) {
      toast.error(error instanceof Error ? error.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadList();
  }, [loadList]);

  const approve = async (id: number) => {
    setIsApproving(true);
    try {
      await apiPost(`/admin/operators/region-applications/${id}/approve`);
      toast.success("申请已通过");
      setSelected(null);
      setApproveConfirmOpen(false);
      await loadList();
    } catch (error: unknown) {
      toast.error(error instanceof Error ? error.message : "审批失败");
    } finally {
      setIsApproving(false);
    }
  };

  const reject = async (id: number) => {
    if (rejectReason.trim().length < 2) {
      toast.warning("请填写至少2个字的驳回原因");
      return;
    }
    setIsRejecting(true);
    try {
      await apiPost(`/admin/operators/region-applications/${id}/reject`, {
        reject_reason: rejectReason.trim(),
      });
      toast.success("申请已驳回");
      setRejectOpen(false);
      setSelected(null);
      setRejectReason("");
      await loadList();
    } catch (error: unknown) {
      toast.error(error instanceof Error ? error.message : "驳回失败");
    } finally {
      setIsRejecting(false);
    }
  };

  const stats = useMemo(() => ({
    pending:  rows.filter(r => r.status === "pending").length,
    approved: rows.filter(r => r.status === "approved").length,
    rejected: rows.filter(r => r.status === "rejected").length,
  }), [rows]);

  return (
    <PageShell>
      <PageHeader
        title="区域扩展申请"
        description="审核运营商申请管理更多区域的请求"
        actions={
          <Button variant="outline" onClick={loadList} disabled={loading}>
            <RefreshCw className="mr-2 h-4 w-4" /> 刷新
          </Button>
        }
      />
      <PageContent className="space-y-6">
        <div className="grid gap-4 md:grid-cols-4">
          <Card>
            <CardHeader className="pb-2"><CardTitle className="text-sm">待审核</CardTitle></CardHeader>
            <CardContent className="text-2xl font-semibold">{stats.pending}</CardContent>
          </Card>
          <Card>
            <CardHeader className="pb-2"><CardTitle className="text-sm">已通过</CardTitle></CardHeader>
            <CardContent className="text-2xl font-semibold">{stats.approved}</CardContent>
          </Card>
          <Card>
            <CardHeader className="pb-2"><CardTitle className="text-sm">已驳回</CardTitle></CardHeader>
            <CardContent className="text-2xl font-semibold">{stats.rejected}</CardContent>
          </Card>
          <Card>
            <CardHeader className="pb-2"><CardTitle className="text-sm">列表总数</CardTitle></CardHeader>
            <CardContent className="text-2xl font-semibold">{total}</CardContent>
          </Card>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>申请列表</CardTitle>
            <CardDescription>运营商提交的区域扩展申请，审批通过后自动关联区域</CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>ID</TableHead>
                  <TableHead>运营商</TableHead>
                  <TableHead>联系人</TableHead>
                  <TableHead>联系电话</TableHead>
                  <TableHead>申请区域</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>申请时间</TableHead>
                  <TableHead className="text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {loading ? (
                  Array.from({ length: 5 }).map((_, i) => (
                    <TableRow key={i}>
                      <TableCell colSpan={8}><Skeleton className="h-5 w-full" /></TableCell>
                    </TableRow>
                  ))
                ) : rows.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={8} className="text-center text-muted-foreground py-8">
                      暂无区域扩展申请
                    </TableCell>
                  </TableRow>
                ) : (
                  rows.map((item) => (
                    <TableRow key={item.id}>
                      <TableCell className="text-muted-foreground text-xs">{item.id}</TableCell>
                      <TableCell className="font-medium">{item.operator_name || "-"}</TableCell>
                      <TableCell>{item.contact_name || "-"}</TableCell>
                      <TableCell>{item.contact_phone || "-"}</TableCell>
                      <TableCell>{item.region_name || "-"}</TableCell>
                      <TableCell>{statusBadge(item.status)}</TableCell>
                      <TableCell className="text-sm text-muted-foreground">
                        {new Date(item.created_at).toLocaleString("zh-CN")}
                      </TableCell>
                      <TableCell className="text-right">
                        {item.status === "pending" ? (
                          <div className="flex justify-end gap-2">
                            <Button
                              size="sm"
                              variant="outline"
                              className="text-green-600 border-green-300 hover:bg-green-50"
                              onClick={() => { setSelected(item); setApproveConfirmOpen(true); }}
                            >
                              <CheckCircle2 className="mr-1 h-4 w-4" /> 通过
                            </Button>
                            <Button
                              size="sm"
                              variant="outline"
                              className="text-destructive border-destructive/30 hover:bg-destructive/5"
                              onClick={() => { setSelected(item); setRejectReason(""); setRejectOpen(true); }}
                            >
                              <XCircle className="mr-1 h-4 w-4" /> 驳回
                            </Button>
                          </div>
                        ) : item.status === "rejected" && item.reject_reason ? (
                          <span className="text-xs text-muted-foreground">驳回原因：{item.reject_reason}</span>
                        ) : null}
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </PageContent>

      {/* 通过确认 */}
      <ConfirmDialog
        open={approveConfirmOpen}
        onOpenChange={setApproveConfirmOpen}
        title="确认通过申请"
        description={
          selected
            ? `确认通过「${selected.operator_name}」申请管理「${selected.region_name}」的请求？通过后该运营商将自动关联该区域。`
            : ""
        }
        confirmText={isApproving ? "处理中…" : "确认通过"}
        onConfirm={() => { if (selected) void approve(selected.id) }}
      />

      {/* 驳回弹窗 */}
      <Dialog open={rejectOpen} onOpenChange={setRejectOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>驳回申请</DialogTitle>
            <DialogDescription>
              {selected && `驳回「${selected.operator_name}」申请管理「${selected.region_name}」`}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-3 py-2">
            <Label htmlFor="reject-reason">驳回原因 <span className="text-destructive">*</span></Label>
            <Textarea
              id="reject-reason"
              value={rejectReason}
              onChange={(e) => setRejectReason(e.target.value)}
              placeholder="请填写驳回原因（至少2字）"
              rows={3}
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRejectOpen(false)}>取消</Button>
            <Button
              variant="destructive"
              disabled={isRejecting || rejectReason.trim().length < 2}
              onClick={() => selected && reject(selected.id)}
            >
              {isRejecting ? <><Loader2 className="mr-2 h-4 w-4 animate-spin" />处理中…</> : "确认驳回"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </PageShell>
  );
}
