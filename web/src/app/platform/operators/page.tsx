"use client";

import Image from "next/image";
import { useCallback, useEffect, useMemo, useState } from "react";
import {
  CheckCircle2,
  ExternalLink,
  FileText,
  Eye,
  RefreshCw,
  XCircle,
} from "lucide-react";
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
import { ScrollArea } from "@/components/ui/scroll-area";
import { Separator } from "@/components/ui/separator";
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
import { apiGet, apiPost, resolveProtectedMediaCandidates } from "@/lib/api";
import type {
  AdminOperatorApplicationItem,
  AdminOperatorApplicationsResponse,
  OperatorApplicationDetail,
} from "@/types/platform-admin";

const PAGE_SIZE = 20;

function statusBadge(status: string) {
  if (status === "submitted") return <Badge>待审核</Badge>;
  if (status === "approved") return <Badge variant="secondary">已通过</Badge>;
  if (status === "rejected") return <Badge variant="destructive">已驳回</Badge>;
  return <Badge variant="outline">未知状态</Badge>;
}

function formatDateTime(value?: string) {
  return value ? new Date(value).toLocaleString("zh-CN") : "-";
}

export default function PlatformOperatorApplicationsPage() {
  const [loading, setLoading] = useState(true);
  const [submittingId, setSubmittingId] = useState<number | null>(null);
  const [rows, setRows] = useState<AdminOperatorApplicationItem[]>([]);
  const [detail, setDetail] = useState<OperatorApplicationDetail | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);
  const [detailRejectReason, setDetailRejectReason] = useState("");
  const [signedAssetUrls, setSignedAssetUrls] = useState<
    Record<
      string,
      {
        url: string;
        state: "loading" | "ready" | "failed";
        candidates: string[];
        candidateIndex: number;
      }
    >
  >({});
  const [summary, setSummary] = useState({ total: 0, hasMore: false });
  const [approveConfirmOpen, setApproveConfirmOpen] = useState(false);
  const [rejectConfirmOpen, setRejectConfirmOpen] = useState(false);

  const loadList = useCallback(async () => {
    setLoading(true);
    try {
      const response = await apiGet<AdminOperatorApplicationsResponse>(
        "/admin/operators/applications",
        { page: 1, limit: PAGE_SIZE }
      );
      setRows(response.applications ?? []);
      setSummary({ total: response.total ?? 0, hasMore: !!response.has_more });
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "加载运营商申请失败";
      toast.error(message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadList();
  }, [loadList]);

  const openDetail = async (id: number) => {
    try {
      const response = await apiGet<OperatorApplicationDetail>(`/admin/operators/applications/${id}`);
      setDetail(response);
      setDetailRejectReason(response.reject_reason || "");
      setSignedAssetUrls({});
      setDetailOpen(true);
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "加载申请详情失败";
      toast.error(message);
    }
  };

  useEffect(() => {
    let cancelled = false;

    const resolveSignedAssets = async () => {
      if (!detailOpen || !detail) return;

      const targets = [
        detail.business_license_url,
        detail.id_card_front_url,
        detail.id_card_back_url,
      ].filter(Boolean) as string[];
      const uniqueTargets = Array.from(new Set(targets));

      if (uniqueTargets.length === 0) {
        setSignedAssetUrls({});
        return;
      }

      setSignedAssetUrls(
        Object.fromEntries(
          uniqueTargets.map((url) => [
            url,
            { url: "", state: "loading" as const, candidates: [], candidateIndex: 0 },
          ])
        )
      );

      const entries = await Promise.all(
        uniqueTargets.map(async (rawUrl) => {
          const candidates = await resolveProtectedMediaCandidates(rawUrl);
          const resolvedUrl = candidates[0] || "";
          if (resolvedUrl && candidates.length > 0) {
            return [
              rawUrl,
              {
                url: resolvedUrl,
                state: "ready" as const,
                candidates,
                candidateIndex: 0,
              },
            ] as const;
          }
          return [
            rawUrl,
            { url: "", state: "failed" as const, candidates: [], candidateIndex: 0 },
          ] as const;
        })
      );

      if (!cancelled) {
        setSignedAssetUrls(Object.fromEntries(entries));
      }
    };

    resolveSignedAssets();

    return () => {
      cancelled = true;
    };
  }, [detail, detailOpen]);

  const approve = async (id: number) => {
    setSubmittingId(id);
    try {
      await apiPost(`/admin/operators/applications/${id}/approve`);
      toast.success("申请已通过");
      await loadList();
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "审核通过失败";
      toast.error(message);
    } finally {
      setSubmittingId(null);
    }
  };

  const reject = async (id: number, rejectReason: string) => {
    if (rejectReason.trim().length < 2) {
      toast.warning("请填写至少2个字的驳回原因");
      return;
    }
    setSubmittingId(id);
    try {
      await apiPost(`/admin/operators/applications/${id}/reject`, {
        reject_reason: rejectReason.trim(),
      });
      toast.success("申请已驳回");
      if (detail?.id === id) {
        setDetailOpen(false);
        setDetail(null);
      }
      await loadList();
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "驳回失败";
      toast.error(message);
    } finally {
      setSubmittingId(null);
    }
  };

  const reviewInDetail = async (decision: "approve" | "reject") => {
    if (!detail || detail.status !== "submitted") return;
    if (decision === "approve") {
      setApproveConfirmOpen(true);
      return;
    }
    setRejectConfirmOpen(true);
  };

  const confirmApproveInDetail = async () => {
    if (!detail) return;
    await approve(detail.id);
    setDetailOpen(false);
    setDetail(null);
  };

  const confirmRejectInDetail = async () => {
    if (!detail) return;
    await reject(detail.id, detailRejectReason);
  };

  const stats = useMemo(() => {
    const submitted = rows.filter((item) => item.status === "submitted").length;
    const approved = rows.filter((item) => item.status === "approved").length;
    const rejected = rows.filter((item) => item.status === "rejected").length;
    return { submitted, approved, rejected };
  }, [rows]);

  const qualificationItems = detail
    ? [
        { label: "营业执照", url: detail.business_license_url },
        { label: "身份证正面", url: detail.id_card_front_url },
        { label: "身份证背面", url: detail.id_card_back_url },
      ]
    : [];

  const missingQualificationCount = qualificationItems.filter((item) => !item.url).length;

  return (
    <PageShell>
      <PageHeader
        title="运营商申请管理"
        description="平台管理员审核运营商入驻申请"
        actions={
          <Button variant="outline" onClick={loadList}>
            <RefreshCw className="mr-2 h-4 w-4" /> 刷新
          </Button>
        }
      />
      <PageContent className="space-y-6">
        <div className="grid gap-4 md:grid-cols-4">
          <Card>
            <CardHeader className="pb-2"><CardTitle className="text-sm">待审核</CardTitle></CardHeader>
            <CardContent className="text-2xl font-semibold">{stats.submitted}</CardContent>
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
            <CardContent className="text-2xl font-semibold">{summary.total}</CardContent>
          </Card>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>申请列表</CardTitle>
            <CardDescription>
              当前按后端默认规则返回最新申请{summary.hasMore ? "（有更多）" : ""}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>ID</TableHead>
                  <TableHead>企业名称</TableHead>
                  <TableHead>联系人</TableHead>
                  <TableHead>区域</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>提交时间</TableHead>
                  <TableHead className="text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {loading ? (
                  Array.from({ length: 6 }).map((_, index) => (
                    <TableRow key={index}>
                      <TableCell colSpan={7}><Skeleton className="h-5 w-full" /></TableCell>
                    </TableRow>
                  ))
                ) : rows.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={7} className="text-center text-muted-foreground">暂无申请数据</TableCell>
                  </TableRow>
                ) : (
                  rows.map((item) => (
                    <TableRow key={item.id}>
                      <TableCell>{item.id}</TableCell>
                      <TableCell>{item.name || "-"}</TableCell>
                      <TableCell>{item.contact_name || "-"}</TableCell>
                      <TableCell>{item.region_name || item.region_code || "-"}</TableCell>
                      <TableCell>{statusBadge(item.status)}</TableCell>
                      <TableCell>{item.submitted_at ? new Date(item.submitted_at).toLocaleString("zh-CN") : "-"}</TableCell>
                      <TableCell className="text-right">
                        <div className="flex justify-end gap-2">
                          <Button size="sm" variant="outline" onClick={() => openDetail(item.id)}>
                            <Eye className="mr-1 h-4 w-4" /> 审核详情
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </CardContent>
        </Card>

        <Dialog open={detailOpen} onOpenChange={setDetailOpen}>
          <DialogContent className="max-w-5xl">
            <DialogHeader>
              <DialogTitle>运营商申请审核详情</DialogTitle>
              <DialogDescription>
                先确认申请主体与区域，再核验资质材料，最后给出审核结论
              </DialogDescription>
            </DialogHeader>
            {detail ? (
              <ScrollArea className="h-[70vh] pr-4">
                <div className="space-y-6">
                  <Card className="border-primary/30">
                    <CardHeader className="pb-3">
                      <CardTitle className="text-base">申请主体摘要</CardTitle>
                      <CardDescription>主体信息是审核判定的核心依据</CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-4">
                      <div className="grid gap-3 sm:grid-cols-3">
                        <div className="rounded-md border border-primary/30 bg-primary/5 px-3 py-2">
                          <div className="text-xs text-muted-foreground">企业名称（关键）</div>
                          <div className="mt-1 text-lg font-semibold text-primary">{detail.name || "-"}</div>
                        </div>
                        <div className="rounded-md border border-primary/30 bg-primary/5 px-3 py-2">
                          <div className="text-xs text-muted-foreground">法人姓名（关键）</div>
                          <div className="mt-1 text-lg font-semibold text-primary">{detail.legal_person_name || "-"}</div>
                        </div>
                        <div className="rounded-md border border-primary/30 bg-primary/5 px-3 py-2">
                          <div className="text-xs text-muted-foreground">营业执照号（关键）</div>
                          <div className="mt-1 text-lg font-semibold text-primary">{detail.business_license_number || "-"}</div>
                        </div>
                      </div>

                      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
                        <div className="rounded-md border bg-muted/30 px-3 py-2">
                          <div className="text-xs text-muted-foreground">当前状态</div>
                          <div className="mt-1">{statusBadge(detail.status)}</div>
                        </div>
                        <div className="rounded-md border bg-muted/30 px-3 py-2">
                          <div className="text-xs text-muted-foreground">申请区域</div>
                          <div className="mt-1 text-sm font-medium">
                            {detail.region_name || detail.region_id || "-"}
                          </div>
                        </div>
                        <div className="rounded-md border bg-muted/30 px-3 py-2">
                          <div className="text-xs text-muted-foreground">提交时间</div>
                          <div className="mt-1 text-sm">{formatDateTime(detail.submitted_at)}</div>
                        </div>
                        <div className="rounded-md border bg-muted/30 px-3 py-2">
                          <div className="text-xs text-muted-foreground">审核时间</div>
                          <div className="mt-1 text-sm">{formatDateTime(detail.reviewed_at)}</div>
                        </div>
                      </div>

                      <div className="grid gap-4 lg:grid-cols-2">
                        <div className="space-y-3 rounded-md border bg-muted/10 p-4">
                          <div className="border-b pb-2 text-xs font-semibold tracking-wide text-muted-foreground">企业与联系人</div>
                          <div className="grid gap-3 sm:grid-cols-2">
                            <div className="rounded-md border bg-background px-3 py-2">
                              <div className="text-xs text-muted-foreground">企业名称</div>
                              <div className="mt-1 text-base font-semibold">{detail.name || "-"}</div>
                            </div>
                            <div className="rounded-md border bg-background px-3 py-2">
                              <div className="text-xs text-muted-foreground">联系人</div>
                              <div className="mt-1 text-base font-semibold">{detail.contact_name || "-"}</div>
                            </div>
                            <div className="sm:col-span-2 rounded-md border bg-background px-3 py-2">
                              <div className="text-xs text-muted-foreground">联系电话</div>
                              <div className="mt-1 text-base font-semibold">{detail.contact_phone || "-"}</div>
                            </div>
                            <div className="rounded-md border bg-background px-3 py-2">
                              <div className="text-xs text-muted-foreground">申请ID</div>
                              <div className="mt-1 text-xs text-muted-foreground">{detail.id}</div>
                            </div>
                            <div className="rounded-md border bg-background px-3 py-2">
                              <div className="text-xs text-muted-foreground">用户ID</div>
                              <div className="mt-1 text-xs text-muted-foreground">{detail.user_id}</div>
                            </div>
                          </div>
                        </div>

                        <div className="space-y-3 rounded-md border bg-muted/10 p-4">
                          <div className="border-b pb-2 text-xs font-semibold tracking-wide text-muted-foreground">法人与证照标识</div>
                          <div className="grid gap-3 sm:grid-cols-2">
                            <div className="sm:col-span-2 rounded-md border bg-background px-3 py-2">
                              <div className="text-xs text-muted-foreground">营业执照号</div>
                              <div className="mt-1 text-base font-semibold">{detail.business_license_number || "-"}</div>
                            </div>
                            <div className="rounded-md border bg-background px-3 py-2">
                              <div className="text-xs text-muted-foreground">法人姓名</div>
                              <div className="mt-1 text-base font-semibold">{detail.legal_person_name || "-"}</div>
                            </div>
                            <div className="rounded-md border bg-background px-3 py-2">
                              <div className="text-xs text-muted-foreground">合同年限</div>
                              <div className="mt-1 text-base font-semibold">{detail.requested_contract_years || 0} 年</div>
                            </div>
                            <div className="sm:col-span-2 rounded-md border bg-background px-3 py-2">
                              <div className="text-xs text-muted-foreground">法人身份证号</div>
                              <div className="mt-1 text-base font-semibold">{detail.legal_person_id_number || "-"}</div>
                            </div>
                          </div>
                        </div>
                      </div>
                    </CardContent>
                  </Card>

                  <Card>
                    <CardHeader className="pb-3">
                      <CardTitle className="text-base">资质核验清单</CardTitle>
                      <CardDescription>
                        {missingQualificationCount === 0
                          ? "材料齐全，可进行一致性核验"
                          : `仍缺失 ${missingQualificationCount} 项材料，建议先驳回补充`}
                      </CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-3">
                      {qualificationItems.map((item) => {
                        const asset = item.url ? signedAssetUrls[item.url] : undefined;

                        return (
                          <div
                            key={item.label}
                            className={`space-y-3 rounded-md border p-3 ${
                              item.url ? "bg-background" : "border-destructive/50 bg-destructive/5"
                            }`}
                          >
                            <div className="flex items-center justify-between">
                              <div className="flex items-center gap-2 text-sm">
                                <FileText className="h-4 w-4 text-muted-foreground" />
                                <span>{item.label}</span>
                                {item.url ? (
                                  <Badge variant="secondary">已上传</Badge>
                                ) : (
                                  <Badge variant="destructive">缺失</Badge>
                                )}
                              </div>
                              {item.url && asset?.state === "ready" && asset.url ? (
                                <Button variant="outline" size="sm" asChild>
                                  <a href={asset.url} target="_blank" rel="noreferrer">
                                    <ExternalLink className="mr-1 h-4 w-4" /> 查看原图
                                  </a>
                                </Button>
                              ) : item.url && asset?.state === "failed" ? (
                                <span className="text-xs text-destructive">签名/加载失败</span>
                              ) : item.url ? (
                                <span className="text-xs text-muted-foreground">链接生成中</span>
                              ) : (
                                <span className="text-xs text-muted-foreground">无文件</span>
                              )}
                            </div>

                            {item.url ? (
                              asset?.state === "ready" && asset.url ? (
                                <div className="overflow-hidden rounded-md border bg-muted/30">
                                  <Image
                                    src={asset.url}
                                    alt={item.label}
                                    width={960}
                                    height={420}
                                    unoptimized
                                    className="h-52 w-full object-contain"
                                    onError={() => {
                                      const key = item.url;
                                      if (!key) return;
                                      setSignedAssetUrls((prev) => {
                                        const current = prev[key];
                                        if (!current || current.state === "failed") return prev;

                                        const nextIndex = current.candidateIndex + 1;
                                        if (nextIndex < current.candidates.length) {
                                          return {
                                            ...prev,
                                            [key]: {
                                              ...current,
                                              url: current.candidates[nextIndex],
                                              candidateIndex: nextIndex,
                                              state: "ready",
                                            },
                                          };
                                        }

                                        return {
                                          ...prev,
                                          [key]: { ...current, state: "failed" },
                                        };
                                      });
                                    }}
                                  />
                                </div>
                              ) : asset?.state === "failed" ? (
                                <div className="flex h-52 items-center justify-center rounded-md border border-dashed border-destructive/50 px-4 text-xs text-destructive">
                                  图片加载失败，请检查签名权限或文件是否存在
                                </div>
                              ) : (
                                <Skeleton className="h-52 w-full" />
                              )
                            ) : (
                              <div className="flex h-52 items-center justify-center rounded-md border border-dashed border-destructive/50 text-xs text-destructive">
                                未上传该材料
                              </div>
                            )}
                          </div>
                        );
                      })}
                    </CardContent>
                  </Card>

                  <Card>
                    <CardHeader className="pb-3">
                      <CardTitle className="text-base">审核结论</CardTitle>
                    </CardHeader>
                    <CardContent className="space-y-3">
                      <div className="flex items-center gap-2 text-sm">
                        <span className="text-muted-foreground">当前状态</span>
                        {statusBadge(detail.status)}
                      </div>
                      <Separator />
                      {detail.status === "approved" && (
                        <div className="rounded-md border bg-muted/30 px-3 py-2 text-sm">
                          该申请已通过审核，运营商账号已创建。
                        </div>
                      )}
                      {detail.status === "rejected" && (
                        <div className="space-y-1">
                          <Label className="text-xs text-muted-foreground">驳回原因</Label>
                          <div className="rounded-md border bg-muted/30 px-3 py-2 text-sm">
                            {detail.reject_reason || "-"}
                          </div>
                        </div>
                      )}
                    </CardContent>
                  </Card>

                  {detail.status === "submitted" && (
                    <Card className="border-primary/40">
                      <CardHeader className="pb-3">
                        <CardTitle className="text-base">审核操作</CardTitle>
                        <CardDescription>
                          通过将立即生效；驳回请给出明确可执行的补充意见
                        </CardDescription>
                      </CardHeader>
                      <CardContent className="space-y-2">
                        <Label htmlFor="detail-reject-reason">驳回原因</Label>
                        <Textarea
                          id="detail-reject-reason"
                          value={detailRejectReason}
                          onChange={(event) => setDetailRejectReason(event.target.value)}
                          placeholder="请输入驳回原因（2-200字）"
                          maxLength={200}
                        />
                      </CardContent>
                    </Card>
                  )}
                </div>
              </ScrollArea>
            ) : (
              <Skeleton className="h-40 w-full" />
            )}
            <DialogFooter>
              <Button variant="outline" onClick={() => setDetailOpen(false)}>
                关闭
              </Button>
              {detail?.status === "submitted" && (
                <>
                  <Button
                    variant="destructive"
                    onClick={() => reviewInDetail("reject")}
                    disabled={submittingId === detail.id}
                  >
                    <XCircle className="mr-1 h-4 w-4" /> 驳回申请
                  </Button>
                  <Button onClick={() => reviewInDetail("approve")} disabled={submittingId === detail.id}>
                    <CheckCircle2 className="mr-1 h-4 w-4" /> 审核通过
                  </Button>
                </>
              )}
            </DialogFooter>
          </DialogContent>
        </Dialog>

        <ConfirmDialog
          open={approveConfirmOpen}
          onOpenChange={setApproveConfirmOpen}
          title="确认通过该申请？"
          description="通过后将生效并创建对应运营商账号。"
          confirmText="确认通过"
          onConfirm={confirmApproveInDetail}
        />

        <ConfirmDialog
          open={rejectConfirmOpen}
          onOpenChange={setRejectConfirmOpen}
          title="确认驳回该申请？"
          description="驳回后需申请方按驳回原因补充材料再提交。"
          confirmText="确认驳回"
          variant="destructive"
          onConfirm={confirmRejectInDetail}
        />
      </PageContent>
    </PageShell>
  );
}
