"use client";

import Image from "next/image";
import { useCallback, useEffect, useMemo, useState } from "react";
import {
  CheckCircle2,
  ExternalLink,
  Eye,
  FileText,
  Loader2,
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
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
import { apiGet, apiPost, getPrivateMediaUrl } from "@/lib/api";
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
  return <Badge variant="outline">未知状态</Badge>;
}

function ocrStatusTag(status?: string) {
  if (status === "done") return <Badge variant="secondary">已识别</Badge>;
  if (status === "processing") return <Badge>识别中</Badge>;
  if (status === "failed") return <Badge variant="destructive">识别失败</Badge>;
  return <Badge variant="outline">未回填</Badge>;
}

function formatDateTime(value?: string) {
  return value ? new Date(value).toLocaleString("zh-CN") : "-";
}

function firstNonEmpty(...values: Array<string | undefined>) {
  for (const value of values) {
    if (value && value.trim()) {
      return value;
    }
  }
  return "-";
}

export default function PlatformGroupApplicationsPage() {
  const [loading, setLoading] = useState(true);
  const [detailLoading, setDetailLoading] = useState(false);
  const [approvingId, setApprovingId] = useState<number | null>(null);
  const [rejectingId, setRejectingId] = useState<number | null>(null);
  const [applications, setApplications] = useState<AdminGroupApplication[]>([]);
  const [status, setStatus] = useState<string>("submitted");
  const [detail, setDetail] = useState<AdminGroupApplication | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);
  const [detailRejectReason, setDetailRejectReason] = useState("");
  const [signedAssetUrls, setSignedAssetUrls] = useState<
    Record<
      string,
      {
        url: string;
        state: "loading" | "ready" | "failed";
      }
    >
  >({});

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

  const openDetail = useCallback(async (id: number) => {
    setDetailOpen(true);
    setDetailLoading(true);
    setSignedAssetUrls({});

    try {
      const response = await apiGet<AdminGroupApplication>(`/admin/groups/applications/${id}`);
      setDetail(response);
      setDetailRejectReason(response.reject_reason || "");
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "加载集团申请详情失败";
      toast.error(message);
      setDetailOpen(false);
    } finally {
      setDetailLoading(false);
    }
  }, []);

  useEffect(() => {
    let cancelled = false;

    const resolveSignedAssets = async () => {
      if (!detailOpen || !detail) return;

      const targets = [
        detail.license_image_asset_id,
        detail.id_card_front_asset_id,
        detail.id_card_back_asset_id,
      ].filter((assetId): assetId is number => typeof assetId === "number" && assetId > 0);
      const uniqueTargets = Array.from(new Set(targets));

      if (uniqueTargets.length === 0) {
        setSignedAssetUrls({});
        return;
      }

      setSignedAssetUrls(
        Object.fromEntries(
          uniqueTargets.map((assetId) => [
            String(assetId),
            { url: "", state: "loading" as const },
          ])
        )
      );

      const entries = await Promise.all(
        uniqueTargets.map(async (assetId) => {
          try {
            const resolvedUrl = await getPrivateMediaUrl(assetId);
            if (resolvedUrl) {
              return [
                String(assetId),
                {
                  url: resolvedUrl,
                  state: "ready" as const,
                },
              ] as const;
            }
          } catch {
            // fall through to failed state
          }

          return [
            String(assetId),
            { url: "", state: "failed" as const },
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

  const review = async (id: number, nextStatus: "approved" | "rejected") => {
    if (nextStatus === "rejected" && detailRejectReason.trim().length === 0) {
      toast.warning("驳回时请填写原因");
      return;
    }

    if (nextStatus === "approved") {
      setApprovingId(id);
    } else {
      setRejectingId(id);
    }

    try {
      await apiPost(`/admin/groups/applications/${id}/review`, {
        status: nextStatus,
        reject_reason: nextStatus === "rejected" ? detailRejectReason.trim() : undefined,
      });
      toast.success(nextStatus === "approved" ? "审核通过" : "已驳回申请");
      setDetailOpen(false);
      setDetail(null);
      await loadData();
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "审核失败";
      toast.error(message);
    } finally {
      setApprovingId(null);
      setRejectingId(null);
    }
  };

  const stats = useMemo(() => {
    const submitted = applications.filter((item) => item.status === "submitted").length;
    const approved = applications.filter((item) => item.status === "approved").length;
    const rejected = applications.filter((item) => item.status === "rejected").length;
    return { submitted, approved, rejected };
  }, [applications]);

  const qualificationItems = detail
    ? [
        {
          label: "营业执照原图",
          assetId: detail.license_image_asset_id,
        },
        {
          label: "负责人身份证正面",
          assetId: detail.id_card_front_asset_id,
        },
        {
          label: "负责人身份证反面",
          assetId: detail.id_card_back_asset_id,
        },
      ]
    : [];

  const missingQualificationCount = qualificationItems.filter((item) => !item.assetId).length;
  const legalPersonName = firstNonEmpty(detail?.legal_person_name, detail?.id_card_front_ocr?.name);
  const legalPersonIDNumber = firstNonEmpty(detail?.legal_person_id_number, detail?.id_card_front_ocr?.id_number);
  const licenseNumber = firstNonEmpty(
    detail?.license_number,
    detail?.business_license_ocr?.credit_code,
    detail?.business_license_ocr?.reg_num
  );

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

      <PageContent className="space-y-6">
        <div className="grid gap-4 md:grid-cols-3">
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm">待审核</CardTitle>
            </CardHeader>
            <CardContent className="text-2xl font-semibold">{stats.submitted}</CardContent>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm">已通过</CardTitle>
            </CardHeader>
            <CardContent className="text-2xl font-semibold">{stats.approved}</CardContent>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm">已驳回</CardTitle>
            </CardHeader>
            <CardContent className="text-2xl font-semibold">{stats.rejected}</CardContent>
          </Card>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>申请列表</CardTitle>
            <CardDescription>先看详情核验证件和 OCR 结果，再给出审核结论</CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>ID</TableHead>
                  <TableHead>集团名称</TableHead>
                  <TableHead>负责人</TableHead>
                  <TableHead>联系电话</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>提交时间</TableHead>
                  <TableHead className="text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {loading ? (
                  Array.from({ length: 6 }).map((_, index) => (
                    <TableRow key={index}>
                      <TableCell colSpan={7}>
                        <Skeleton className="h-5 w-full" />
                      </TableCell>
                    </TableRow>
                  ))
                ) : applications.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={7} className="text-center text-muted-foreground">
                      暂无申请数据
                    </TableCell>
                  </TableRow>
                ) : (
                  applications.map((item) => (
                    <TableRow key={item.id}>
                      <TableCell>{item.id}</TableCell>
                      <TableCell>{item.group_name}</TableCell>
                      <TableCell>{firstNonEmpty(item.legal_person_name, item.id_card_front_ocr?.name)}</TableCell>
                      <TableCell>{item.contact_phone}</TableCell>
                      <TableCell>{statusTag(item.status)}</TableCell>
                      <TableCell>{formatDateTime(item.created_at)}</TableCell>
                      <TableCell className="text-right">
                        <Button size="sm" variant="outline" onClick={() => openDetail(item.id)}>
                          <Eye className="mr-1 h-4 w-4" />
                          {item.status === "submitted" ? "审核详情" : "查看详情"}
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </CardContent>
        </Card>

        <Dialog
          open={detailOpen}
          onOpenChange={(open) => {
            setDetailOpen(open);
            if (!open) {
              setDetail(null);
              setDetailLoading(false);
              setSignedAssetUrls({});
            }
          }}
        >
          <DialogContent className="max-w-5xl">
            <DialogHeader>
              <DialogTitle>集团申请审核详情</DialogTitle>
              <DialogDescription>
                先核验负责人身份和证照识别结果，再决定是否通过集团入驻申请
              </DialogDescription>
            </DialogHeader>

            {detailLoading ? (
              <div className="space-y-4 py-4">
                <Skeleton className="h-32 w-full" />
                <Skeleton className="h-40 w-full" />
                <Skeleton className="h-60 w-full" />
              </div>
            ) : detail ? (
              <ScrollArea className="h-[70vh] pr-4">
                <div className="space-y-6">
                  <Card className="border-primary/30">
                    <CardHeader className="pb-3">
                      <CardTitle className="text-base">申请主体摘要</CardTitle>
                      <CardDescription>先确认集团主体、负责人和核心证照号是否一致</CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-4">
                      <div className="grid gap-3 sm:grid-cols-3">
                        <div className="rounded-md border border-primary/30 bg-primary/5 px-3 py-2">
                          <div className="text-xs text-muted-foreground">集团名称</div>
                          <div className="mt-1 text-lg font-semibold text-primary">{detail.group_name || "-"}</div>
                        </div>
                        <div className="rounded-md border border-primary/30 bg-primary/5 px-3 py-2">
                          <div className="text-xs text-muted-foreground">负责人姓名</div>
                          <div className="mt-1 text-lg font-semibold text-primary">{legalPersonName}</div>
                        </div>
                        <div className="rounded-md border border-primary/30 bg-primary/5 px-3 py-2">
                          <div className="text-xs text-muted-foreground">营业执照号</div>
                          <div className="mt-1 text-lg font-semibold text-primary">{licenseNumber}</div>
                        </div>
                      </div>

                      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
                        <div className="rounded-md border bg-muted/30 px-3 py-2">
                          <div className="text-xs text-muted-foreground">当前状态</div>
                          <div className="mt-1">{statusTag(detail.status)}</div>
                        </div>
                        <div className="rounded-md border bg-muted/30 px-3 py-2">
                          <div className="text-xs text-muted-foreground">联系电话</div>
                          <div className="mt-1 text-sm font-medium">{detail.contact_phone || "-"}</div>
                        </div>
                        <div className="rounded-md border bg-muted/30 px-3 py-2">
                          <div className="text-xs text-muted-foreground">提交时间</div>
                          <div className="mt-1 text-sm">{formatDateTime(detail.created_at)}</div>
                        </div>
                        <div className="rounded-md border bg-muted/30 px-3 py-2">
                          <div className="text-xs text-muted-foreground">审核时间</div>
                          <div className="mt-1 text-sm">{formatDateTime(detail.reviewed_at)}</div>
                        </div>
                      </div>

                      <div className="grid gap-4 lg:grid-cols-2">
                        <div className="space-y-3 rounded-md border bg-muted/10 p-4">
                          <div className="border-b pb-2 text-xs font-semibold tracking-wide text-muted-foreground">申请基础信息</div>
                          <div className="grid gap-3 sm:grid-cols-2">
                            <div className="rounded-md border bg-background px-3 py-2">
                              <div className="text-xs text-muted-foreground">申请人用户 ID</div>
                              <div className="mt-1 text-base font-semibold">{detail.applicant_user_id}</div>
                            </div>
                            <div className="rounded-md border bg-background px-3 py-2">
                              <div className="text-xs text-muted-foreground">区域 ID</div>
                              <div className="mt-1 text-base font-semibold">{detail.region_id || "-"}</div>
                            </div>
                            <div className="sm:col-span-2 rounded-md border bg-background px-3 py-2">
                              <div className="text-xs text-muted-foreground">办公地址</div>
                              <div className="mt-1 text-base font-semibold">{detail.address || "-"}</div>
                            </div>
                          </div>
                        </div>

                        <div className="space-y-3 rounded-md border bg-muted/10 p-4">
                          <div className="border-b pb-2 text-xs font-semibold tracking-wide text-muted-foreground">负责人身份信息</div>
                          <div className="grid gap-3 sm:grid-cols-2">
                            <div className="rounded-md border bg-background px-3 py-2">
                              <div className="text-xs text-muted-foreground">负责人姓名</div>
                              <div className="mt-1 text-base font-semibold">{legalPersonName}</div>
                            </div>
                            <div className="rounded-md border bg-background px-3 py-2">
                              <div className="text-xs text-muted-foreground">身份证号</div>
                              <div className="mt-1 text-base font-semibold">{legalPersonIDNumber}</div>
                            </div>
                            <div className="rounded-md border bg-background px-3 py-2">
                              <div className="text-xs text-muted-foreground">身份证正面 OCR</div>
                              <div className="mt-1">{ocrStatusTag(detail.id_card_front_ocr?.status)}</div>
                            </div>
                            <div className="rounded-md border bg-background px-3 py-2">
                              <div className="text-xs text-muted-foreground">身份证反面 OCR</div>
                              <div className="mt-1">{ocrStatusTag(detail.id_card_back_ocr?.status)}</div>
                            </div>
                          </div>
                        </div>
                      </div>
                    </CardContent>
                  </Card>

                  <div className="grid gap-4 lg:grid-cols-2">
                    <Card>
                      <CardHeader className="pb-3">
                        <CardTitle className="text-base">营业执照识别结果</CardTitle>
                        <CardDescription>核验主体名称、证照号和法定代表人是否一致</CardDescription>
                      </CardHeader>
                      <CardContent className="space-y-3">
                        <div className="flex items-center gap-2">
                          <span className="text-sm text-muted-foreground">识别状态</span>
                          {ocrStatusTag(detail.business_license_ocr?.status)}
                        </div>
                        <div className="grid gap-3 sm:grid-cols-2">
                          <div className="rounded-md border bg-muted/10 px-3 py-2">
                            <div className="text-xs text-muted-foreground">企业名称</div>
                            <div className="mt-1 text-sm font-medium">
                              {firstNonEmpty(detail.business_license_ocr?.enterprise_name, detail.group_name)}
                            </div>
                          </div>
                          <div className="rounded-md border bg-muted/10 px-3 py-2">
                            <div className="text-xs text-muted-foreground">统一社会信用代码/注册号</div>
                            <div className="mt-1 text-sm font-medium">{licenseNumber}</div>
                          </div>
                          <div className="rounded-md border bg-muted/10 px-3 py-2">
                            <div className="text-xs text-muted-foreground">法定代表人</div>
                            <div className="mt-1 text-sm font-medium">
                              {firstNonEmpty(detail.business_license_ocr?.legal_representative, legalPersonName)}
                            </div>
                          </div>
                          <div className="rounded-md border bg-muted/10 px-3 py-2">
                            <div className="text-xs text-muted-foreground">有效期</div>
                            <div className="mt-1 text-sm font-medium">{detail.business_license_ocr?.valid_period || "-"}</div>
                          </div>
                          <div className="sm:col-span-2 rounded-md border bg-muted/10 px-3 py-2">
                            <div className="text-xs text-muted-foreground">注册地址</div>
                            <div className="mt-1 text-sm font-medium">
                              {firstNonEmpty(detail.business_license_ocr?.address, detail.address)}
                            </div>
                          </div>
                          <div className="sm:col-span-2 rounded-md border bg-muted/10 px-3 py-2">
                            <div className="text-xs text-muted-foreground">经营范围</div>
                            <div className="mt-1 text-sm font-medium">{detail.business_license_ocr?.business_scope || "-"}</div>
                          </div>
                        </div>
                        {detail.business_license_ocr?.error && (
                          <div className="rounded-md border border-destructive/50 bg-destructive/5 px-3 py-2 text-sm text-destructive">
                            {detail.business_license_ocr.error}
                          </div>
                        )}
                      </CardContent>
                    </Card>

                    <Card>
                      <CardHeader className="pb-3">
                        <CardTitle className="text-base">负责人身份证识别结果</CardTitle>
                        <CardDescription>重点核验姓名、身份证号和证件有效期</CardDescription>
                      </CardHeader>
                      <CardContent className="space-y-3">
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="text-sm text-muted-foreground">正面</span>
                          {ocrStatusTag(detail.id_card_front_ocr?.status)}
                          <span className="ml-2 text-sm text-muted-foreground">反面</span>
                          {ocrStatusTag(detail.id_card_back_ocr?.status)}
                        </div>
                        <div className="grid gap-3 sm:grid-cols-2">
                          <div className="rounded-md border bg-muted/10 px-3 py-2">
                            <div className="text-xs text-muted-foreground">姓名</div>
                            <div className="mt-1 text-sm font-medium">{legalPersonName}</div>
                          </div>
                          <div className="rounded-md border bg-muted/10 px-3 py-2">
                            <div className="text-xs text-muted-foreground">身份证号</div>
                            <div className="mt-1 text-sm font-medium">{legalPersonIDNumber}</div>
                          </div>
                          <div className="rounded-md border bg-muted/10 px-3 py-2">
                            <div className="text-xs text-muted-foreground">性别</div>
                            <div className="mt-1 text-sm font-medium">{detail.id_card_front_ocr?.gender || "-"}</div>
                          </div>
                          <div className="rounded-md border bg-muted/10 px-3 py-2">
                            <div className="text-xs text-muted-foreground">民族</div>
                            <div className="mt-1 text-sm font-medium">{detail.id_card_front_ocr?.nation || "-"}</div>
                          </div>
                          <div className="sm:col-span-2 rounded-md border bg-muted/10 px-3 py-2">
                            <div className="text-xs text-muted-foreground">住址</div>
                            <div className="mt-1 text-sm font-medium">{detail.id_card_front_ocr?.address || "-"}</div>
                          </div>
                          <div className="sm:col-span-2 rounded-md border bg-muted/10 px-3 py-2">
                            <div className="text-xs text-muted-foreground">证件有效期</div>
                            <div className="mt-1 text-sm font-medium">{detail.id_card_back_ocr?.valid_date || "-"}</div>
                          </div>
                        </div>
                        {(detail.id_card_front_ocr?.error || detail.id_card_back_ocr?.error) && (
                          <div className="space-y-2">
                            {detail.id_card_front_ocr?.error && (
                              <div className="rounded-md border border-destructive/50 bg-destructive/5 px-3 py-2 text-sm text-destructive">
                                身份证正面识别失败：{detail.id_card_front_ocr.error}
                              </div>
                            )}
                            {detail.id_card_back_ocr?.error && (
                              <div className="rounded-md border border-destructive/50 bg-destructive/5 px-3 py-2 text-sm text-destructive">
                                身份证反面识别失败：{detail.id_card_back_ocr.error}
                              </div>
                            )}
                          </div>
                        )}
                      </CardContent>
                    </Card>
                  </div>

                  <Card>
                    <CardHeader className="pb-3">
                      <CardTitle className="text-base">资质原图核验</CardTitle>
                      <CardDescription>
                        {missingQualificationCount === 0
                          ? "材料齐全，可逐项比对 OCR 与原图是否一致"
                          : `仍缺失 ${missingQualificationCount} 项材料，建议先驳回补充`}
                      </CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-3">
                      {qualificationItems.map((item) => {
                        const assetKey = item.assetId ? String(item.assetId) : "";
                        const asset = assetKey ? signedAssetUrls[assetKey] : undefined;
                        const hasMaterial = !!item.assetId;

                        return (
                          <div
                            key={item.label}
                            className={`space-y-3 rounded-md border p-3 ${
                              hasMaterial ? "bg-background" : "border-destructive/50 bg-destructive/5"
                            }`}
                          >
                            <div className="flex items-center justify-between gap-3">
                              <div className="flex items-center gap-2 text-sm">
                                <FileText className="h-4 w-4 text-muted-foreground" />
                                <span>{item.label}</span>
                                {hasMaterial ? (
                                  <Badge variant="secondary">已上传</Badge>
                                ) : (
                                  <Badge variant="destructive">缺失</Badge>
                                )}
                              </div>
                              {hasMaterial && asset?.state === "ready" && asset.url ? (
                                <Button variant="outline" size="sm" asChild>
                                  <a href={asset.url} target="_blank" rel="noreferrer">
                                    <ExternalLink className="mr-1 h-4 w-4" /> 查看原图
                                  </a>
                                </Button>
                              ) : hasMaterial && asset?.state === "failed" ? (
                                <span className="text-xs text-destructive">私有访问地址生成失败</span>
                              ) : hasMaterial ? (
                                <span className="text-xs text-muted-foreground">链接生成中</span>
                              ) : (
                                <span className="text-xs text-muted-foreground">无文件</span>
                              )}
                            </div>

                            {hasMaterial ? (
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
                                      if (!assetKey) return;
                                      setSignedAssetUrls((prev) => ({
                                        ...prev,
                                        [assetKey]: { ...(prev[assetKey] ?? { url: "" }), state: "failed" },
                                      }));
                                    }}
                                  />
                                </div>
                              ) : asset?.state === "failed" ? (
                                <div className="flex h-52 items-center justify-center rounded-md border border-dashed border-destructive/50 px-4 text-xs text-destructive">
                                  图片加载失败，请检查私有访问权限或文件是否存在
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
                        {statusTag(detail.status)}
                      </div>
                      <Separator />
                      {detail.status === "approved" && (
                        <div className="rounded-md border bg-muted/30 px-3 py-2 text-sm">
                          该申请已通过审核，集团主体已经创建。
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
                      {detail.status === "submitted" && (
                        <div className="space-y-2">
                          <Label htmlFor="group-reject-reason" className="text-xs text-muted-foreground">
                            驳回原因
                          </Label>
                          <Textarea
                            id="group-reject-reason"
                            value={detailRejectReason}
                            onChange={(event) => setDetailRejectReason(event.target.value)}
                            placeholder="仅在驳回时填写，需明确说明补充或修改项"
                            maxLength={200}
                          />
                        </div>
                      )}
                    </CardContent>
                    {detail.status === "submitted" && (
                      <DialogFooter className="gap-2 sm:justify-end">
                        <Button
                          variant="destructive"
                          onClick={() => review(detail.id, "rejected")}
                          disabled={approvingId === detail.id || rejectingId === detail.id}
                        >
                          {rejectingId === detail.id ? (
                            <Loader2 className="mr-1 h-4 w-4 animate-spin" />
                          ) : (
                            <XCircle className="mr-1 h-4 w-4" />
                          )}
                          驳回申请
                        </Button>
                        <Button
                          onClick={() => review(detail.id, "approved")}
                          disabled={approvingId === detail.id || rejectingId === detail.id}
                        >
                          {approvingId === detail.id ? (
                            <Loader2 className="mr-1 h-4 w-4 animate-spin" />
                          ) : (
                            <CheckCircle2 className="mr-1 h-4 w-4" />
                          )}
                          审核通过
                        </Button>
                      </DialogFooter>
                    )}
                  </Card>
                </div>
              </ScrollArea>
            ) : null}
          </DialogContent>
        </Dialog>
      </PageContent>
    </PageShell>
  );
}
