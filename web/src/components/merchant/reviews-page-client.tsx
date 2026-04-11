"use client";

import { useState, useEffect, useMemo, useCallback } from "react";
import Image from "next/image";
import { 
  MessageSquare, 
  Search, 
  RefreshCw, 
  Reply, 
  Clock, 
  User, 
  ArrowRight
} from "lucide-react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { 
  Dialog, 
  DialogContent, 
  DialogTitle,
  DialogDescription
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { 
  Card,
  CardContent,
} from "@/components/ui/card";
import { 
  Tabs, 
  TabsList, 
  TabsTrigger 
} from "@/components/ui/tabs";
import { PageShell, PageHeader, PageContent } from "@/components/merchant/layout/page-shell";
import { apiGet, apiPost } from "@/lib/api";
import { getMediaDisplayUrl } from "@/lib/media";
import { cn } from "@/lib/utils";
import { useMerchantSession } from "@/components/providers/merchant-session-provider";
import type { ReviewResponse, ReviewListResponse } from "@/types/review";

export function ReviewsPageClient() {
  const session = useMerchantSession();
  const activeMerchant = session?.merchant;
  
  const [loading, setLoading] = useState(false);
  const [reviews, setReviews] = useState<ReviewResponse[]>([]);
  const [totalCount, setTotalCount] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize] = useState(10);
  const [searchQuery, setSearchQuery] = useState("");
  const [currentTab, setCurrentTab] = useState("all");

  // Reply State
  const [replyingReview, setReplyingReview] = useState<ReviewResponse | null>(null);
  const [replyContent, setReplyContent] = useState("");
  const [submittingReply, setSubmittingReply] = useState(false);

    const loadReviews = useCallback(async () => {
    if (!activeMerchant?.id) return;
    setLoading(true);
    try {
      const data = await apiGet<ReviewListResponse>(`/reviews/merchants/${activeMerchant.id}/all`, {
        page_id: page,
        page_size: pageSize
      });
      setReviews(data.reviews || []);
      setTotalCount(data.total || 0);
      } catch (error: unknown) {
        const message = error instanceof Error ? error.message : "加载评价列表失败";
        toast.error(message);
    } finally {
      setLoading(false);
    }
    }, [activeMerchant?.id, page, pageSize]);

    useEffect(() => {
      if (activeMerchant?.id) {
        loadReviews();
      }
    }, [activeMerchant?.id, loadReviews]);

  const handleReply = async () => {
    if (!replyingReview || !replyContent) return;
    setSubmittingReply(true);
    try {
      await apiPost(`/reviews/${replyingReview.id}/reply`, { reply: replyContent });
      toast.success("回复成功");
      setReplyingReview(null);
      setReplyContent("");
      loadReviews();
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "回复失败";
      toast.error(message);
    } finally {
      setSubmittingReply(false);
    }
  };

  const filteredReviews = useMemo(() => {
    let result = reviews;
    
    // 1. Tab Filtering (Frontend filtering for current page)
    if (currentTab === "unreplied") {
      result = result.filter(r => !r.merchant_reply);
    } else if (currentTab === "replied") {
      result = result.filter(r => !!r.merchant_reply);
    }

    // 2. Search Filtering
    if (searchQuery) {
      const q = searchQuery.toLowerCase();
      result = result.filter(r => 
        r.content.toLowerCase().includes(q) || 
        r.id.toString().includes(q) ||
        r.order_id.toString().includes(q)
      );
    }

    return result;
  }, [reviews, currentTab, searchQuery]);

  return (
    <PageShell>
      <PageHeader 
        title="评价管理" 
        description="查看并回复顾客对菜品和服务的评价，提升商户口碑与信用"
        actions={
          <Button variant="outline" size="sm" onClick={loadReviews}>
            <RefreshCw className={cn("h-4 w-4 mr-2", loading && "animate-spin")} />
            刷新列表
          </Button>
        }
      />
      <PageContent>
        <div className="space-y-6">
          <div className="flex flex-col md:flex-row gap-4 items-center justify-between">
            <Tabs defaultValue="all" className="w-full md:w-auto" onValueChange={setCurrentTab}>
              <TabsList className="bg-slate-100 p-1 rounded-xl h-11">
                <TabsTrigger value="all" className="rounded-lg px-6 font-bold transition-all data-[state=active]:bg-white data-[state=active]:text-foreground data-[state=active]:shadow-sm">
                  全部评价
                </TabsTrigger>
                <TabsTrigger value="unreplied" className="rounded-lg px-6 font-bold transition-all data-[state=active]:bg-white data-[state=active]:text-foreground data-[state=active]:shadow-sm">
                  待回复
                </TabsTrigger>
                <TabsTrigger value="replied" className="rounded-lg px-6 font-bold transition-all data-[state=active]:bg-white data-[state=active]:text-foreground data-[state=active]:shadow-sm">
                  已回复
                </TabsTrigger>
              </TabsList>
            </Tabs>

            <div className="relative w-full md:w-80">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
              <Input
                placeholder="搜索评价内容或订单号..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="pl-9 bg-white rounded-xl"
              />
            </div>
          </div>

          <div className="grid grid-cols-1 gap-4">
            {loading && reviews.length === 0 ? (
              Array.from({ length: 3 }).map((_, i) => (
                <Card key={i} className="rounded-3xl border-slate-100 animate-pulse h-48" />
              ))
            ) : filteredReviews.length === 0 ? (
              <div className="py-20 text-center bg-slate-50 rounded-[2rem] border-2 border-dashed border-slate-200">
                <div className="bg-white w-20 h-20 rounded-3xl flex items-center justify-center mx-auto mb-6 shadow-sm ring-1 ring-slate-100">
                  <MessageSquare className="h-10 w-10 text-slate-300" />
                </div>
                <h3 className="text-xl font-black text-slate-900">暂无相关评价</h3>
                <p className="text-muted-foreground mt-2 max-w-xs mx-auto text-sm font-medium">
                  {searchQuery ? "换个关键词搜搜看，或者调整过滤条件" : "您的店铺还没有顾客发表评价，好的服务会带来更多好评哦"}
                </p>
                {searchQuery && (
                   <Button variant="link" onClick={() => setSearchQuery("")} className="mt-4 font-bold">清空搜索条件</Button>
                )}
              </div>
            ) : (
              filteredReviews.map((review) => (
                <Card key={review.id} className="rounded-[2rem] border-slate-100 overflow-hidden transition-all hover:shadow-xl hover:border-primary/20 group">
                  <CardContent className="p-0">
                    <div className="flex flex-col md:flex-row">
                      {/* Left Side: Review Info */}
                      <div className="flex-1 p-6 md:p-8 space-y-4">
                        <div className="flex items-center justify-between">
                          <div className="flex items-center gap-3">
                            <div className="w-12 h-12 rounded-2xl bg-slate-50 flex items-center justify-center border border-slate-100">
                              <User className="h-6 w-6 text-slate-300" />
                            </div>
                            <div>
                              <p className="font-black text-slate-900 flex items-center gap-2">
                                神秘用户
                                <span className="text-[10px] font-mono font-medium text-slate-400 bg-slate-50 px-1.5 py-0.5 rounded border border-slate-100">ID: {review.user_id}</span>
                              </p>
                              <div className="flex items-center gap-2 mt-0.5">
                                <Clock className="h-3 w-3 text-muted-foreground" />
                                <span className="text-xs text-muted-foreground font-medium">
                                  {new Date(review.created_at).toLocaleString('zh-CN', { 
                                    year: 'numeric', 
                                    month: '2-digit', 
                                    day: '2-digit', 
                                    hour: '2-digit', 
                                    minute: '2-digit' 
                                  })}
                                </span>
                              </div>
                            </div>
                          </div>
                          
                          <div className="flex flex-col items-end gap-2">
                            <Badge variant={review.merchant_reply ? "default" : "secondary"} className={cn(
                              "text-[10px] h-6 px-2 rounded-lg font-black uppercase tracking-wider",
                              review.merchant_reply ? "bg-emerald-50 text-emerald-600 hover:bg-emerald-50 border-emerald-100" : "bg-orange-50 text-orange-600 hover:bg-orange-50 border-orange-100"
                            )}>
                              {review.merchant_reply ? "已回复" : "待回复"}
                            </Badge>
                            <span className="text-[10px] font-bold text-muted-foreground bg-slate-50 px-2 py-1 rounded-md border border-slate-100">
                              订单号: {review.order_id}
                            </span>
                          </div>
                        </div>

                        <div className="relative">
                          <p className="text-slate-700 leading-relaxed font-medium">
                            {review.content}
                          </p>
                        </div>

                        {review.images && review.images.length > 0 && (
                          <div className="flex flex-wrap gap-2 pt-2">
                            {review.images.map((url, index) => (
                              <div 
                                key={index} 
                                className="relative w-24 h-24 rounded-2xl overflow-hidden border-2 border-white shadow-sm ring-1 ring-slate-100 cursor-zoom-in group/img"
                              >
                                <Image 
                                  src={getMediaDisplayUrl(url)} 
                                  alt={`评价图片 ${index + 1}`} 
                                  width={96}
                                  height={96}
                                  className="w-full h-full object-cover transition-transform duration-500 group-hover/img:scale-110" 
                                />
                                <div className="absolute inset-0 bg-black/5 opacity-0 group-hover/img:opacity-100 transition-opacity" />
                              </div>
                            ))}
                          </div>
                        )}
                      </div>

                      {/* Right Side: Merchant Reply Area */}
                      <div className={cn(
                        "w-full md:w-80 lg:w-100 border-t md:border-t-0 md:border-l p-6 md:p-8 flex flex-col justify-center",
                        review.merchant_reply ? "bg-slate-50/50" : "bg-white"
                      )}>
                        {review.merchant_reply ? (
                          <div className="space-y-4">
                            <div className="flex items-center gap-2">
                              <div className="w-8 h-8 rounded-xl bg-primary/10 flex items-center justify-center">
                                <Reply className="h-4 w-4 text-primary" />
                              </div>
                              <span className="font-black text-sm text-slate-900">商家回复</span>
                              <span className="text-[10px] text-muted-foreground ml-auto">
                                {review.replied_at && new Date(review.replied_at).toLocaleDateString()}
                              </span>
                            </div>
                            <div className="p-4 bg-white border border-slate-100 rounded-2xl shadow-sm italic text-sm text-slate-600 font-medium relative">
                              <div className="absolute -left-2 top-4 w-4 h-4 bg-white border-l border-t border-slate-100 rotate-45" />
                              “{review.merchant_reply}”
                            </div>
                            <Button variant="ghost" size="sm" className="w-full text-xs font-bold rounded-xl h-10 hover:bg-slate-100" onClick={() => {
                              setReplyingReview(review);
                              setReplyContent(review.merchant_reply || "");
                            }}>
                              修改回复内容
                            </Button>
                          </div>
                        ) : (
                          <div className="text-center space-y-4">
                            <div className="w-16 h-16 rounded-full bg-primary/5 flex items-center justify-center mx-auto">
                              <div className="w-10 h-10 rounded-full bg-primary/10 flex items-center justify-center animate-pulse">
                                <Reply className="h-5 w-5 text-primary" />
                              </div>
                            </div>
                            <div>
                              <h4 className="font-black text-slate-900">尚未回复评价</h4>
                              <p className="text-xs text-muted-foreground mt-1 font-medium italic">回复顾客评价能显著提升回头客转化率</p>
                            </div>
                            <Button 
                              className="w-full rounded-2xl h-12 font-black shadow-lg shadow-primary/20 group/btn" 
                              onClick={() => {
                                setReplyingReview(review);
                                setReplyContent("");
                              }}
                            >
                              立即写回复
                              <ArrowRight className="h-4 w-4 ml-2 transition-transform group-hover/btn:translate-x-1" />
                            </Button>
                          </div>
                        )}
                      </div>
                    </div>
                  </CardContent>
                </Card>
              ))
            )}
          </div>
          
          {totalCount > pageSize && (
            <div className="flex justify-center pt-8">
              <div className="flex items-center gap-2 bg-white p-1 rounded-2xl border shadow-sm">
                 <Button 
                   variant="ghost" 
                   size="sm" 
                   disabled={page === 1} 
                   onClick={() => setPage(page - 1)}
                   className="rounded-xl px-4"
                 >
                   上一页
                 </Button>
                 <div className="px-4 text-sm font-black text-slate-900 border-x">
                   {page} / {Math.ceil(totalCount / pageSize)}
                 </div>
                 <Button 
                   variant="ghost" 
                   size="sm" 
                   disabled={page >= Math.ceil(totalCount / pageSize)} 
                   onClick={() => setPage(page + 1)}
                   className="rounded-xl px-4"
                 >
                   下一页
                 </Button>
              </div>
            </div>
          )}
        </div>
      </PageContent>

      {/* Reply Dialog */}
      <Dialog open={!!replyingReview} onOpenChange={(open) => !open && setReplyingReview(null)}>
        <DialogContent className="sm:max-w-125 rounded-[2.5rem] p-0 overflow-hidden border-none shadow-2xl">
          <div className="bg-slate-900 p-10 text-white relative overflow-hidden">
             <div className="absolute top-0 right-0 w-64 h-64 bg-primary/10 rounded-full -translate-y-1/2 translate-x-1/2 blur-3xl" />
             <div className="absolute bottom-0 left-0 w-48 h-48 bg-primary/5 rounded-full translate-y-1/2 -translate-x-1/2 blur-2xl" />
             
             <div className="relative z-10 space-y-4">
               <div className="inline-flex items-center gap-2 px-3 py-1 bg-white/10 rounded-full border border-white/10 backdrop-blur-sm">
                  <Reply className="h-4 w-4 text-primary" />
                  <span className="text-[10px] font-black uppercase tracking-wider text-white">回复顾客评价</span>
               </div>
               <DialogTitle className="text-3xl font-black text-white leading-tight">
                 写下您的诚挚回复
               </DialogTitle>
               <DialogDescription className="text-slate-400 text-sm font-medium pr-10">
                 耐心和礼貌的回复是最好的营销。如果客户反馈了问题，请表现出改进的决心。
               </DialogDescription>
             </div>
          </div>
          
          <div className="p-10 space-y-8 bg-white">
            <div className="bg-slate-50 p-6 rounded-3xl border border-slate-100 flex gap-4 items-start italic relative">
               <div className="absolute -top-3 left-6 bg-white border border-slate-100 px-3 py-1 rounded-full text-[10px] font-black uppercase text-slate-400">顾客评价</div>
               <MessageSquare className="h-5 w-5 text-slate-300 mt-1 shrink-0" />
               <p className="text-sm text-slate-500 font-medium leading-relaxed">
                 {replyingReview?.content}
               </p>
            </div>

            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <Label className="text-xs font-black text-slate-400 uppercase tracking-widest px-1">回复内容</Label>
                <span className={cn(
                  "text-[10px] font-bold px-2 py-0.5 rounded-full border",
                  replyContent.length > 400 ? "text-orange-600 bg-orange-50 border-orange-100" : "text-slate-400 bg-slate-50"
                )}>
                  {replyContent.length} / 500
                </span>
              </div>
              <Textarea
                placeholder="在此输入您的回复，例如：'非常感谢您的好评！您的支持是我们前进的动力...' 或 '很抱歉没能给您满意的体验，我们会继续改进...'"
                value={replyContent}
                onChange={(e) => setReplyContent(e.target.value)}
                className="min-h-40 rounded-3xl border-slate-200 focus:ring-primary/20 focus:border-primary px-6 py-6 font-medium text-slate-700 resize-none transition-all placeholder:text-slate-300"
                maxLength={500}
              />
            </div>
            
            <div className="flex gap-4 pt-2">
               {[
                 "感谢好评，期待您再次光临！",
                 "抱歉给您带来不便，我们会努力改进。",
                 "感谢反馈，您的建议非常宝贵。"
               ].map((temp, i) => (
                 <button 
                   key={i}
                   onClick={() => setReplyContent(temp)}
                   className="text-[10px] font-black bg-slate-50 hover:bg-slate-100 text-slate-500 py-2 px-3 rounded-xl border border-slate-100 transition-colors shrink-0"
                 >
                   使用模板 {i+1}
                 </button>
               ))}
            </div>
          </div>
          
          <div className="px-10 pb-10 flex gap-3">
             <Button variant="ghost" onClick={() => setReplyingReview(null)} className="flex-1 rounded-2xl h-14 font-black">取消</Button>
             <Button 
               className="flex-2 rounded-2xl h-14 font-black shadow-xl shadow-primary/20" 
               onClick={handleReply}
               disabled={!replyContent || submittingReply}
             >
               {submittingReply && <RefreshCw className="mr-2 h-4 w-4 animate-spin" />}
               发布回复
             </Button>
          </div>
        </DialogContent>
      </Dialog>
    </PageShell>
  );
}
