"use client";

import { useState, useMemo, useEffect, useCallback } from "react";
import Image from "next/image";
import { 
  Users, 
  Search, 
  RefreshCw, 
  Plus, 
  Minus, 
  History,
  Info,
  Wallet,
  ArrowUpCircle,
  ArrowDownCircle,
  Edit,
  Trash2,
  Calendar,
  Sparkles
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Separator } from "@/components/ui/separator";
import { 
  Dialog, 
  DialogContent, 
  DialogHeader, 
  DialogTitle,
  DialogFooter,
  DialogDescription
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { PageShell, PageHeader, PageContent } from "@/components/merchant/layout/page-shell";
import { apiGet, apiPost, apiPatch, apiPut, apiDelete, formatAmount } from "@/lib/api";
import { cn } from "@/lib/utils";
import { 
  Tabs, 
  TabsContent, 
  TabsList, 
  TabsTrigger 
} from "@/components/ui/tabs";
import { Checkbox } from "@/components/ui/checkbox";
import { Switch } from "@/components/ui/switch";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Slider } from "@/components/ui/slider";
import { useMerchantSession } from "@/components/providers/merchant-session-provider";
import type { 
  MemberResponse, 
  ListMerchantMembersResponse, 
  MemberDetailResponse,
  RechargeRuleResponse,
  MembershipSettings
} from "@/types/member";

export function MembersPageClient() {
  const session = useMerchantSession();
  const activeMerchant = session?.merchant;
  const [activeTab, setActiveTab] = useState("members");
  
  // Members List State
  const [loading, setLoading] = useState(false);
  const [members, setMembers] = useState<MemberResponse[]>([]);
  const pageSize = 20;

  const [searchQuery, setSearchQuery] = useState("");
  const [selectedMemberId, setSelectedMemberId] = useState<number | null>(null);
  const [detail, setDetail] = useState<MemberDetailResponse | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);

  // Recharge Rules State
  const [rules, setRules] = useState<RechargeRuleResponse[]>([]);
  const [rulesLoading, setRulesLoading] = useState(false);
  const [isRuleDialogOpen, setIsRuleDialogOpen] = useState(false);
  const [editingRule, setEditingRule] = useState<Partial<RechargeRuleResponse> | null>(null);
  const [savingRule, setSavingRule] = useState(false);
  const [deleteRuleId, setDeleteRuleId] = useState<number | null>(null);

  // Settings State
  const [settings, setSettings] = useState<MembershipSettings | null>(null);
  const [settingsLoading, setSettingsLoading] = useState(false);
  const [savingSettings, setSavingSettings] = useState(false);

  // Adjust Balance Dialog
  const [isAdjustOpen, setIsAdjustOpen] = useState(false);
  const [adjustType, setAdjustType] = useState<"add" | "deduct">("add");
  const [adjustAmount, setAdjustAmount] = useState("");
  const [adjustNotes, setAdjustNotes] = useState("");
  const [adjusting, setAdjusting] = useState(false);

  const loadMembers = useCallback(async (pageToLoad = 1, append = false) => {
    if (!activeMerchant?.id) return;
    setLoading(true);
    try {
      const data = await apiGet<ListMerchantMembersResponse>(
        `/merchants/${activeMerchant.id}/members`,
        { page_id: pageToLoad, page_size: pageSize }
      );
      if (append) {
        setMembers(prev => [...prev, ...data.members]);
      } else {
        setMembers(data.members);
      }
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "加载会员列表失败";
      toast.error(message);
    } finally {
      setLoading(false);
    }
  }, [activeMerchant?.id]);

  const loadDetail = async (userId: number) => {
    if (!activeMerchant?.id) return;
    setDetailLoading(true);
    try {
      const data = await apiGet<MemberDetailResponse>(
        `/merchants/${activeMerchant.id}/members/${userId}`
      );
      setDetail(data);
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "加载会员详情失败";
      toast.error(message);
    } finally {
      setDetailLoading(false);
    }
  };

  const loadRules = useCallback(async () => {
    if (!activeMerchant?.id) return;
    setRulesLoading(true);
    try {
      const data = await apiGet<RechargeRuleResponse[]>(`/merchants/${activeMerchant.id}/recharge-rules`);
      setRules(data);
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "加载充值规则失败";
      toast.error(message);
    } finally {
      setRulesLoading(false);
    }
  }, [activeMerchant?.id]);

  const loadSettings = useCallback(async () => {
    if (!activeMerchant?.id) return;
    setSettingsLoading(true);
    try {
      const data = await apiGet<MembershipSettings>(`/merchants/me/membership-settings`);
      setSettings(data);
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "加载会员设置失败";
      toast.error(message);
    } finally {
      setSettingsLoading(false);
    }
  }, [activeMerchant?.id]);

  useEffect(() => {
    if (activeMerchant?.id) {
      if (activeTab === "members") loadMembers();
      if (activeTab === "rules") loadRules();
      if (activeTab === "settings") loadSettings();
    }
  }, [activeMerchant?.id, activeTab, loadMembers, loadRules, loadSettings]);

  const handleMemberSelect = (userId: number) => {
    setSelectedMemberId(userId);
    loadDetail(userId);
  };

  const handleAdjustBalance = async () => {
    if (!activeMerchant?.id || !detail) return;
    const amountFloat = parseFloat(adjustAmount);
    if (isNaN(amountFloat) || amountFloat <= 0) {
      toast.error("请输入有效金额");
      return;
    }
    if (!adjustNotes.trim()) {
      toast.error("请输入调整备注");
      return;
    }

    setAdjusting(true);
    try {
      const amountFen = Math.round(amountFloat * 100);
      const finalAmount = adjustType === "add" ? amountFen : -amountFen;

      await apiPost(`/merchants/${activeMerchant.id}/members/${detail.user_id}/balance`, {
        amount: finalAmount,
        notes: adjustNotes
      });

      toast.success("余额调整成功");
      setIsAdjustOpen(false);
      setAdjustAmount("");
      setAdjustNotes("");
      
      loadMembers(1, false);
      if (selectedMemberId) loadDetail(selectedMemberId);
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "操作失败";
      toast.error(message);
    } finally {
      setAdjusting(false);
    }
  };

  const handleSaveRule = async () => {
    if (!activeMerchant?.id || !editingRule) return;
    
    setSavingRule(true);
    try {
      const payload = {
        recharge_amount: Number(editingRule.recharge_amount),
        bonus_amount: Number(editingRule.bonus_amount || 0),
        is_active: editingRule.is_active,
        valid_from: editingRule.valid_from ? new Date(editingRule.valid_from).toISOString() : undefined,
        valid_until: editingRule.valid_until ? new Date(editingRule.valid_until).toISOString() : undefined,
      };

      if (editingRule.id) {
        await apiPatch(`/merchants/${activeMerchant.id}/recharge-rules/${editingRule.id}`, payload);
      } else {
        await apiPost(`/merchants/${activeMerchant.id}/recharge-rules`, payload);
      }
      toast.success("规则保存成功");
      setIsRuleDialogOpen(false);
      loadRules();
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "保存失败";
      toast.error(message);
    } finally {
      setSavingRule(false);
    }
  };

  const handleDeleteRule = async () => {
    if (!activeMerchant?.id || !deleteRuleId) return;
    try {
      await apiDelete(`/merchants/${activeMerchant.id}/recharge-rules/${deleteRuleId}`);
      toast.success("规则已删除");
      setDeleteRuleId(null);
      loadRules();
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "删除失败";
      toast.error(message);
    }
  };

  const handleUpdateSettings = async () => {
    if (!settings) return;
    setSavingSettings(true);
    try {
      await apiPut("/merchants/me/membership-settings", settings);
      toast.success("设置更新成功");
      loadSettings();
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "保存失败";
      toast.error(message);
    } finally {
      setSavingSettings(false);
    }
  };

  const filteredMembers = useMemo(() => {
    if (!searchQuery) return members;
    return members.filter(m => 
      m.full_name?.toLowerCase().includes(searchQuery.toLowerCase()) ||
      m.phone?.includes(searchQuery)
    );
  }, [members, searchQuery]);

  const formatTxType = (type: string) => {
    const map: Record<string, { label: string; color: string; icon: LucideIcon }> = {
      'recharge': { label: '充值', color: 'text-emerald-600', icon: ArrowUpCircle },
      'consume': { label: '消费', color: 'text-rose-600', icon: ArrowDownCircle },
      'refund': { label: '退款', color: 'text-emerald-600', icon: ArrowUpCircle },
      'adjustment_credit': { label: '系统加值', color: 'text-blue-600', icon: Plus },
      'adjustment_debit': { label: '系统扣减', color: 'text-amber-600', icon: Minus },
    };
    return map[type] || { label: type, color: 'text-slate-600', icon: History };
  };

  return (
    <PageShell>
      <PageHeader 
        title="会员管理" 
        description="查看品牌会员信息、设置充值规则以及储值使用规范"
        actions={
          <div className="flex gap-2">
            {activeTab === "rules" && (
                <Button size="sm" onClick={() => {
                    setEditingRule({
                        recharge_amount: 10000,
                        bonus_amount: 0,
                        is_active: true,
                        valid_from: new Date().toISOString().split('T')[0],
                        valid_until: new Date(Date.now() + 365*24*60*60*1000).toISOString().split('T')[0]
                    });
                    setIsRuleDialogOpen(true);
                }}>
                    <Plus className="h-4 w-4 mr-2" /> 新增规则
                </Button>
            )}
            <Button variant="outline" size="sm" onClick={() => {
                if (activeTab === "members") loadMembers(1, false);
                if (activeTab === "rules") loadRules();
                if (activeTab === "settings") loadSettings();
            }}>
                <RefreshCw className={cn("h-4 w-4 mr-2", (loading || rulesLoading || settingsLoading) && "animate-spin")} />
                刷新
            </Button>
          </div>
        }
      />
      <PageContent>
        <Tabs value={activeTab} onValueChange={setActiveTab} className="space-y-6">
          <TabsList className="bg-slate-100 p-1 rounded-xl h-12">
            <TabsTrigger value="members" className="rounded-lg px-8 font-bold data-[state=active]:bg-white data-[state=active]:text-foreground data-[state=active]:shadow-sm">
                会员查询
            </TabsTrigger>
            <TabsTrigger value="rules" className="rounded-lg px-8 font-bold data-[state=active]:bg-white data-[state=active]:text-foreground data-[state=active]:shadow-sm">
                充值规则
            </TabsTrigger>
            <TabsTrigger value="settings" className="rounded-lg px-8 font-bold data-[state=active]:bg-white data-[state=active]:text-foreground data-[state=active]:shadow-sm">
                储值使用设置
            </TabsTrigger>
          </TabsList>

          <TabsContent value="members">
            <div className="flex h-[calc(100vh-16rem)] gap-6">
              <div className="w-1/3 min-w-90 flex flex-col bg-white rounded-xl border shadow-sm overflow-hidden">
                <div className="p-4 border-b space-y-4 bg-slate-50/50">
                  <div className="relative">
                    <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                    <Input
                      placeholder="搜索姓名或手机号..."
                      className="pl-9 bg-white border-slate-200"
                      value={searchQuery}
                      onChange={(e) => setSearchQuery(e.target.value)}
                    />
                  </div>
                </div>

                <ScrollArea className="flex-1">
                  <div className="p-2 space-y-1">
                    {loading && members.length === 0 ? (
                      <div className="p-8 text-center text-muted-foreground animate-pulse text-sm">加载中...</div>
                    ) : filteredMembers.length === 0 ? (
                      <div className="p-8 text-center text-muted-foreground text-sm">未找到相关会员</div>
                    ) : (
                      filteredMembers.map((member) => (
                        <div
                          key={member.user_id}
                          onClick={() => handleMemberSelect(member.user_id)}
                          className={cn(
                            "group p-3 rounded-lg border transition-all cursor-pointer flex items-center justify-between hover:border-primary/50 hover:bg-slate-50",
                            selectedMemberId === member.user_id ? "border-primary bg-primary/5 ring-1 ring-primary" : "border-transparent"
                          )}
                        >
                          <div className="flex items-center gap-3">
                            <div className="w-10 h-10 rounded-full bg-slate-100 flex items-center justify-center overflow-hidden border border-slate-200">
                              {member.avatar_url ? (
                                <Image src={member.avatar_url} alt={member.full_name || "会员头像"} width={40} height={40} className="w-full h-full object-cover" />
                              ) : (
                                <Users className="w-5 h-5 text-slate-400" />
                              )}
                            </div>
                            <div className="flex flex-col">
                              <span className="font-medium text-slate-900">{member.full_name}</span>
                              <span className="text-xs text-muted-foreground">{member.phone}</span>
                            </div>
                          </div>
                          <div className="text-right flex flex-col items-end">
                            <span className="text-sm font-bold text-primary">¥{formatAmount(member.balance)}</span>
                            <span className="text-[10px] text-muted-foreground font-mono">ID:{member.membership_id}</span>
                          </div>
                        </div>
                      ))
                    )}
                  </div>
                </ScrollArea>
              </div>

              <div className="flex-1 bg-white rounded-xl border shadow-sm flex flex-col overflow-hidden">
                {detailLoading ? (
                  <div className="flex-1 flex items-center justify-center"><RefreshCw className="h-8 w-8 animate-spin opacity-20" /></div>
                ) : detail ? (
                  <>
                    <div className="p-6 border-b bg-slate-50/50 flex items-center justify-between">
                      <div className="flex items-center gap-4">
                        <div className="w-16 h-16 rounded-2xl bg-white shadow-sm flex items-center justify-center overflow-hidden border">
                          {detail.avatar_url ? (
                            <Image src={detail.avatar_url} alt={detail.full_name || "会员头像"} width={64} height={64} className="w-full h-full object-cover" />
                          ) : (
                            <Users className="w-8 h-8 text-slate-300" />
                          )}
                        </div>
                        <div>
                          <h2 className="text-xl font-bold text-slate-900">{detail.full_name}</h2>
                          <p className="text-sm text-muted-foreground">{detail.phone} · 加入于 {new Date(detail.created_at).toLocaleDateString()}</p>
                        </div>
                      </div>
                      <Button variant="outline" onClick={() => { setAdjustType("add"); setIsAdjustOpen(true); }}><Plus className="h-4 w-4 mr-2" /> 增减余额</Button>
                    </div>
                    <ScrollArea className="flex-1 p-6">
                        <div className="grid grid-cols-3 gap-6 mb-8">
                            <div className="p-4 rounded-xl border bg-slate-50">
                                <p className="text-xs text-muted-foreground font-medium uppercase mb-1">当前余额</p>
                                <p className="text-2xl font-black text-primary">¥{formatAmount(detail.balance)}</p>
                            </div>
                            <div className="p-4 rounded-xl border bg-slate-50">
                                <p className="text-xs text-muted-foreground font-medium uppercase mb-1">累计充值</p>
                                <p className="text-2xl font-black text-emerald-600">¥{formatAmount(detail.total_recharged)}</p>
                            </div>
                            <div className="p-4 rounded-xl border bg-slate-50">
                                <p className="text-xs text-muted-foreground font-medium uppercase mb-1">累计消费</p>
                                <p className="text-2xl font-black text-rose-600">¥{formatAmount(detail.total_consumed)}</p>
                            </div>
                        </div>
                        <div className="space-y-4">
                            <h3 className="text-sm font-semibold border-l-4 border-primary pl-3">最近交易流水</h3>
                            <div className="rounded-xl border divide-y overflow-hidden">
                                {detail.transactions?.map(tx => {
                                    const typeInfo = formatTxType(tx.type);
                                    return (
                                        <div key={tx.id} className="p-4 flex items-center justify-between hover:bg-slate-50 transition-colors">
                                            <div className="flex items-center gap-3">
                                                <div className={cn("w-8 h-8 rounded-lg flex items-center justify-center", typeInfo.color.replace('text', 'bg').replace('600', '100'))}>
                                                    <typeInfo.icon className={cn("w-4 h-4", typeInfo.color)} />
                                                </div>
                                                <div className="flex flex-col">
                                                    <span className="text-sm font-bold">{typeInfo.label}</span>
                                                    <span className="text-xs text-muted-foreground">{new Date(tx.created_at).toLocaleString()}</span>
                                                </div>
                                            </div>
                                            <div className="text-right">
                                                <p className={cn("font-bold", tx.amount > 0 ? "text-emerald-600" : "text-rose-600")}>
                                                    {tx.amount > 0 ? "+" : ""}{formatAmount(tx.amount)}
                                                </p>
                                                <p className="text-[10px] text-muted-foreground">余额 ¥{formatAmount(tx.balance_after)}</p>
                                            </div>
                                        </div>
                                    )
                                })}
                            </div>
                        </div>
                    </ScrollArea>
                  </>
                ) : (
                  <div className="flex-1 flex flex-col items-center justify-center opacity-40"><Info className="h-12 w-12 mb-4" /><p>选择会员以查看详情</p></div>
                )}
              </div>
            </div>
          </TabsContent>

          <TabsContent value="rules">
            <div className="bg-white rounded-xl border shadow-sm overflow-hidden">
                <div className="p-6 border-b bg-slate-50/50">
                    <h3 className="font-bold text-slate-900 border-l-4 border-primary pl-3">储值营销规则</h3>
                    <p className="text-sm text-muted-foreground mt-1">设置“充 $100 送 $20”等规则，刺激用户转化。用户在支付时会自动匹配最优规则。</p>
                </div>
                <div className="p-6">
                    {rulesLoading ? (
                        <div className="py-20 text-center"><RefreshCw className="h-8 w-8 animate-spin mx-auto opacity-20" /></div>
                    ) : rules.length === 0 ? (
                        <div className="py-20 text-center border-2 border-dashed rounded-xl">
                            <Plus className="h-10 w-10 mx-auto text-slate-200 mb-4" />
                            <p className="text-slate-400">尚未创建任何充值规则</p>
                            <Button variant="outline" className="mt-4" onClick={() => {
                                setEditingRule({
                                    recharge_amount: 10000,
                                    bonus_amount: 0,
                                    is_active: true,
                                    valid_from: new Date().toISOString().split('T')[0],
                                    valid_until: new Date(Date.now() + 365*24*60*60*1000).toISOString().split('T')[0]
                                });
                                setIsRuleDialogOpen(true);
                            }}>立即创建第一条规则</Button>
                        </div>
                    ) : (
                        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
                            {rules.map(rule => (
                                <div key={rule.id} className="relative group p-6 rounded-2xl border bg-white hover:border-primary/50 transition-all shadow-sm">
                                    <div className="flex justify-between items-start mb-6">
                                        <Badge className={cn("rounded-md", rule.is_active ? "bg-emerald-500" : "bg-slate-400")}>
                                            {rule.is_active ? "生效中" : "已停用"}
                                        </Badge>
                                        <div className="flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                                            <Button variant="ghost" size="icon" className="h-8 w-8 rounded-full" onClick={() => {
                                                setEditingRule({ ...rule, 
                                                    valid_from: rule.valid_from.split('T')[0], 
                                                    valid_until: rule.valid_until.split('T')[0] 
                                                });
                                                setIsRuleDialogOpen(true);
                                            }}><Edit className="h-4 w-4" /></Button>
                                            <Button variant="ghost" size="icon" className="h-8 w-8 rounded-full text-rose-500 hover:text-rose-600" onClick={() => setDeleteRuleId(rule.id)}><Trash2 className="h-4 w-4" /></Button>
                                        </div>
                                    </div>
                                    <div className="space-y-4">
                                        <div className="flex items-end gap-2">
                                            <span className="text-xs text-muted-foreground font-bold mb-1">充</span>
                                            <span className="text-3xl font-black">¥{formatAmount(rule.recharge_amount)}</span>
                                        </div>
                                        <div className="flex items-end gap-2 text-emerald-600">
                                            <span className="text-xs font-bold mb-1">送</span>
                                            <span className="text-3xl font-black">¥{formatAmount(rule.bonus_amount)}</span>
                                        </div>
                                        <Separator className="bg-slate-100" />
                                        <div className="text-[11px] text-muted-foreground flex items-center gap-2">
                                            <Calendar className="h-3 w-3" />
                                            {new Date(rule.valid_from).toLocaleDateString()} ~ {new Date(rule.valid_until).toLocaleDateString()}
                                        </div>
                                    </div>
                                </div>
                            ))}
                        </div>
                    )}
                </div>
            </div>
          </TabsContent>

          <TabsContent value="settings">
            <div className="max-w-4xl mx-auto space-y-6">
              <div className="bg-white rounded-xl border shadow-sm overflow-hidden">
                <div className="p-6 border-b bg-slate-50/50 flex justify-between items-center">
                    <div>
                        <h3 className="font-bold text-slate-900 border-l-4 border-primary pl-3">会员储值使用权限</h3>
                        <p className="text-sm text-muted-foreground mt-1">全局配置会员余额和赠送金的使用场景及其叠加规则。</p>
                    </div>
                    <Button onClick={handleUpdateSettings} disabled={savingSettings || !settings}>
                        {savingSettings && <RefreshCw className="mr-2 h-4 w-4 animate-spin" />}
                        保存全局设置
                    </Button>
                </div>
                {settingsLoading ? (
                    <div className="p-20 text-center"><RefreshCw className="h-8 w-8 animate-spin mx-auto opacity-20" /></div>
                ) : settings && (
                    <div className="p-8 space-y-10">
                        <div className="grid grid-cols-2 gap-12">
                            <section className="space-y-4">
                                <h4 className="text-sm font-black text-slate-700 uppercase tracking-widest flex items-center gap-2">
                                    <Wallet className="h-4 w-4 text-primary" /> 本金余额可用场景
                                </h4>
                                <div className="space-y-3 p-4 rounded-xl bg-slate-50 border border-slate-100">
                                    {[
                                        { id: "dine_in", label: "堂食买单" },
                                        { id: "takeout", label: "外卖下单" },
                                        { id: "reservation", label: "预约订座" }
                                    ].map(scene => (
                                        <div key={scene.id} className="flex items-center space-x-3">
                                            <Checkbox 
                                                id={`balance-${scene.id}`} 
                                                checked={settings.balance_usable_scenes?.includes(scene.id)}
                                                onCheckedChange={(checked) => {
                                                    const next = checked 
                                                        ? [...(settings.balance_usable_scenes || []), scene.id]
                                                        : settings.balance_usable_scenes?.filter(s => s !== scene.id);
                                                    setSettings({ ...settings, balance_usable_scenes: next });
                                                }}
                                            />
                                            <Label htmlFor={`balance-${scene.id}`} className="text-sm font-medium cursor-pointer">{scene.label}</Label>
                                        </div>
                                    ))}
                                </div>
                            </section>

                            <section className="space-y-4">
                                <h4 className="text-sm font-black text-slate-700 uppercase tracking-widest flex items-center gap-2">
                                    <Sparkles className="h-4 w-4 text-emerald-500" /> 赠送金可用场景
                                </h4>
                                <div className="space-y-3 p-4 rounded-xl bg-slate-50 border border-slate-100">
                                    {[
                                        { id: "dine_in", label: "堂食买单" },
                                        { id: "takeout", label: "外卖下单" },
                                        { id: "reservation", label: "预约订座" }
                                    ].map(scene => (
                                        <div key={scene.id} className="flex items-center space-x-3">
                                            <Checkbox 
                                                id={`bonus-${scene.id}`} 
                                                checked={settings.bonus_usable_scenes?.includes(scene.id)}
                                                onCheckedChange={(checked) => {
                                                    const next = checked 
                                                        ? [...(settings.bonus_usable_scenes || []), scene.id]
                                                        : settings.bonus_usable_scenes?.filter(s => s !== scene.id);
                                                    setSettings({ ...settings, bonus_usable_scenes: next });
                                                }}
                                            />
                                            <Label htmlFor={`bonus-${scene.id}`} className="text-sm font-medium cursor-pointer">{scene.label}</Label>
                                        </div>
                                    ))}
                                </div>
                            </section>
                        </div>

                        <Separator className="bg-slate-100" />

                        <section className="space-y-6">
                            <h4 className="text-sm font-black text-slate-700 uppercase tracking-widest flex items-center gap-2">
                                <Info className="h-4 w-4 text-blue-500" /> 优惠叠加限制
                            </h4>
                            <div className="grid gap-6">
                                <div className="flex items-center justify-between p-4 rounded-xl border bg-white shadow-sm hover:border-primary/20 transition-colors">
                                    <div className="space-y-1">
                                        <Label className="text-base font-bold">允许与代金券叠加</Label>
                                        <p className="text-xs text-muted-foreground">如果关闭，使用代金券的订单将不能使用余额支付</p>
                                    </div>
                                    <Switch 
                                        checked={settings.allow_with_voucher}
                                        onCheckedChange={c => setSettings({ ...settings, allow_with_voucher: c })}
                                    />
                                </div>
                                <div className="flex items-center justify-between p-4 rounded-xl border bg-white shadow-sm hover:border-primary/20 transition-colors">
                                    <div className="space-y-1">
                                        <Label className="text-base font-bold">允许与满减折扣叠加</Label>
                                        <p className="text-xs text-muted-foreground">如果关闭，命中满减优惠的订单将不能使用余额支付</p>
                                    </div>
                                    <Switch 
                                        checked={settings.allow_with_discount}
                                        onCheckedChange={c => setSettings({ ...settings, allow_with_discount: c })}
                                    />
                                </div>
                                
                                <div className="space-y-6 p-6 rounded-2xl bg-slate-50 border-2 border-dashed border-slate-200">
                                    <div className="flex items-center justify-between">
                                        <div className="space-y-1">
                                            <Label className="text-lg font-black text-slate-900">单笔储值抵扣上限</Label>
                                            <p className="text-sm text-uted-foreground font-medium">限制当前订单中余额支付所占的最大比例</p>
                                        </div>
                                        <div className="px-4 py-2 bg-primary rounded-xl shadow-lg shadow-primary/20">
                                            <span className="text-2xl font-black text-white font-mono">{settings.max_deduction_percent}%</span>
                                        </div>
                                    </div>
                                    
                                    <div className="px-2 pt-4">
                                        <Slider 
                                            value={[settings.max_deduction_percent]} 
                                            min={1} 
                                            max={100} 
                                            step={1}
                                            onValueChange={([val]) => setSettings({ ...settings, max_deduction_percent: val })}
                                            className="py-4"
                                        />
                                        <div className="flex justify-between mt-4 text-[10px] font-black text-slate-400 uppercase tracking-widest">
                                            <div className="flex flex-col items-start gap-1">
                                                <div className="h-1.5 w-0.5 bg-slate-300 rounded-full" />
                                                <span>严格限制 (1%)</span>
                                            </div>
                                            <div className="flex flex-col items-center gap-1">
                                                <div className="h-1.5 w-0.5 bg-slate-300 rounded-full" />
                                                <span>均衡 (50%)</span>
                                            </div>
                                            <div className="flex flex-col items-end gap-1">
                                                <div className="h-1.5 w-0.5 bg-slate-300 rounded-full" />
                                                <span>无限制 (100%)</span>
                                            </div>
                                        </div>
                                    </div>

                                    <div className="p-4 bg-white rounded-xl border border-slate-100 flex items-start gap-4">
                                        <div className="w-10 h-10 rounded-full bg-amber-50 flex items-center justify-center shrink-0">
                                            <Info className="h-5 w-5 text-amber-500" />
                                        </div>
                                        <div className="text-xs text-slate-500 leading-relaxed">
                                            <span className="font-bold text-slate-700 block mb-1">提示：</span>
                                            设置较低的比例可以有效缓解商户现金流压力。例如设为 50%，则 100 元的订单用户最多只能用余额付 50 元，剩下 50 元需实付。
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </section>
                    </div>
                )}
              </div>
            </div>
          </TabsContent>
        </Tabs>
      </PageContent>

      <Dialog open={isAdjustOpen} onOpenChange={setIsAdjustOpen}>
        <DialogContent className="sm:max-w-105 rounded-2xl">
          <DialogHeader className="space-y-2">
            <DialogTitle className="text-xl font-bold">手动调整会员余额</DialogTitle>
            <DialogDescription>此操作会直接修改会员余额，并记入人工流水。</DialogDescription>
          </DialogHeader>
          <div className="grid gap-6 py-4">
            <div className="space-y-2">
              <Label>调整类型</Label>
              <div className="grid grid-cols-2 gap-2">
                <Button 
                  type="button" variant={adjustType === "add" ? "default" : "outline"}
                  className={cn("h-12 border-2", adjustType === "add" ? "border-primary" : "border-slate-100 bg-slate-50")}
                  onClick={() => setAdjustType("add")}
                ><Plus className="h-4 w-4 mr-2" /> 增加余额</Button>
                <Button 
                  type="button" variant={adjustType === "deduct" ? "destructive" : "outline"}
                  className={cn("h-12 border-2", adjustType === "deduct" ? "border-rose-600" : "border-slate-100 bg-slate-50")}
                  onClick={() => setAdjustType("deduct")}
                ><Minus className="h-4 w-4 mr-2" /> 扣减余额</Button>
              </div>
            </div>
            <div className="space-y-2">
              <Label htmlFor="amount">调整金额 (元) <span className="text-destructive">*</span></Label>
              <div className="relative">
                <span className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground font-bold">¥</span>
                <Input
                  id="amount" type="number" step="0.01" placeholder="0.00"
                  className="pl-7 h-12 text-lg font-black border-2 focus-visible:ring-primary"
                  value={adjustAmount} onChange={(e) => setAdjustAmount(e.target.value)}
                />
              </div>
            </div>
            <div className="space-y-2">
              <Label htmlFor="notes">调整原因/备注 <span className="text-destructive">*</span></Label>
              <Textarea
                id="notes" placeholder="例如：系统核销错误、线下充值等..."
                className="resize-none h-24 border-2 rounded-xl"
                value={adjustNotes} onChange={(e) => setAdjustNotes(e.target.value)}
              />
            </div>
          </div>
          <DialogFooter className="bg-slate-50 -mx-6 -mb-6 p-4 border-t rounded-b-2xl">
            <Button variant="ghost" onClick={() => setIsAdjustOpen(false)}>取消</Button>
            <Button 
              className={cn("rounded-xl font-bold px-8 shadow-md", adjustType === "add" ? "bg-primary" : "bg-rose-600")}
              onClick={handleAdjustBalance} disabled={adjusting}
            >确认提交</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={isRuleDialogOpen} onOpenChange={setIsRuleDialogOpen}>
        <DialogContent className="sm:max-w-115 rounded-2xl">
          <DialogHeader>
            <DialogTitle className="text-xl font-black">{editingRule?.id ? '编辑充值规则' : '创建充值规则'}</DialogTitle>
            <DialogDescription>设置会员单次充值的金额以及对应的获赠金额。</DialogDescription>
          </DialogHeader>
          <div className="grid gap-6 py-6">
            <div className="grid grid-cols-2 gap-4">
                <div className="space-y-2">
                    <Label className="font-bold">充值金额 (元)</Label>
                    <Input 
                        type="number"
                        step="0.01"
                        className="h-11 border-2 font-black"
                        value={editingRule?.recharge_amount ? (editingRule.recharge_amount / 100) : ""}
                        onChange={e => {
                            const val = Math.round(parseFloat(e.target.value) * 100) || 0;
                            setEditingRule(p => p ? ({ ...p, recharge_amount: val }) : null);
                        }}
                    />
                </div>
                <div className="space-y-2">
                    <Label className="font-bold">获赠金额 (元)</Label>
                    <Input 
                        type="number"
                        step="0.01"
                        className="h-11 border-2 font-black text-emerald-600"
                        value={editingRule?.bonus_amount ? (editingRule.bonus_amount / 100) : (editingRule?.bonus_amount === 0 ? "0" : "")}
                        onChange={e => {
                            const val = Math.round(parseFloat(e.target.value) * 100) || 0;
                            setEditingRule(p => p ? ({ ...p, bonus_amount: val }) : null);
                        }}
                    />
                </div>
            </div>
            <div className="grid grid-cols-2 gap-4">
                <div className="space-y-2">
                    <Label className="font-bold">生效日期</Label>
                    <Input type="date" value={editingRule?.valid_from} onChange={e => setEditingRule(p => p ? ({ ...p, valid_from: e.target.value }) : null)} className="h-11 border-2 font-bold" />
                </div>
                <div className="space-y-2">
                    <Label className="font-bold">失效日期</Label>
                    <Input type="date" value={editingRule?.valid_until} onChange={e => setEditingRule(p => p ? ({ ...p, valid_until: e.target.value }) : null)} className="h-11 border-2 font-bold" />
                </div>
            </div>
            <div className="flex items-center justify-between p-4 bg-slate-50 rounded-xl border-2">
                <div className="space-y-0.5">
                    <Label className="font-bold">是否激活</Label>
                    <p className="text-[10px] text-muted-foreground font-medium">非激活状态的规则在用户支付时将不可见</p>
                </div>
                <Switch checked={editingRule?.is_active} onCheckedChange={c => setEditingRule(p => p ? ({ ...p, is_active: c }) : null)} />
            </div>
          </div>
          <DialogFooter className="bg-slate-50 -mx-6 -mb-6 p-4 border-t rounded-b-2xl">
            <Button variant="ghost" onClick={() => setIsRuleDialogOpen(false)}>取消</Button>
            <Button className="rounded-xl font-bold px-8 shadow-md" onClick={handleSaveRule} disabled={savingRule}>保存规则</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog 
        open={!!deleteRuleId}
        onOpenChange={(open) => !open && setDeleteRuleId(null)}
        title="确认删除规则"
        description="该操作无法撤销。删除后，新的充值将不再匹配该规则，但不影响已充值的本金和赠送金。"
        confirmText="确认删除"
        variant="destructive"
        onConfirm={handleDeleteRule}
      />
    </PageShell>
  );
}
