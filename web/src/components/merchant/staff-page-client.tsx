"use client";

import { useState, useEffect, useMemo } from "react";
import { 
  Users, 
  Search, 
  RefreshCw, 
  UserPlus, 
  ShieldCheck, 
  Trash2, 
  Edit, 
  Copy, 
  Check, 
  QrCode,
  Info,
  MoreVertical,
  User,
  Crown,
  Briefcase,
  ChefHat,
  Banknote,
  Clock,
  ExternalLink
} from "lucide-react";
import { toast } from "sonner";
import QRCode from "qrcode";
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
  DialogDescription,
  DialogTrigger
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { 
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  DropdownMenuSeparator,
  DropdownMenuLabel
} from "@/components/ui/dropdown-menu";
import { PageShell, PageHeader, PageContent } from "@/components/merchant/layout/page-shell";
import { apiGet, apiPost, apiPatch, apiDelete, getMediaUrl } from "@/lib/api";
import { cn } from "@/lib/utils";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { useMerchantSession } from "@/components/providers/merchant-session-provider";
import type { 
  StaffResponse, 
  ListMerchantStaffResponse, 
  InviteCodeResponse 
} from "@/types/staff";

const ROLE_MAP: Record<string, { label: string; icon: any; color: string; bg: string }> = {
  owner: { label: "老板", icon: Crown, color: "text-amber-600", bg: "bg-amber-50" },
  manager: { label: "店长", icon: Briefcase, color: "text-blue-600", bg: "bg-blue-50" },
  chef: { label: "厨师长", icon: ChefHat, color: "text-orange-600", bg: "bg-orange-50" },
  cashier: { label: "收银员", icon: Banknote, color: "text-emerald-600", bg: "bg-emerald-50" },
  pending: { label: "待分配", icon: Clock, color: "text-slate-400", bg: "bg-slate-50" },
};

const STATUS_MAP: Record<string, { label: string; variant: "default" | "secondary" | "destructive" | "outline" }> = {
  active: { label: "在职", variant: "default" },
  disabled: { label: "离职", variant: "destructive" },
  pending: { label: "待入号", variant: "secondary" },
};

export function StaffPageClient() {
  const session = useMerchantSession();
  const activeMerchant = session?.merchant;
  
  const [loading, setLoading] = useState(false);
  const [staff, setStaff] = useState<StaffResponse[]>([]);
  const [searchQuery, setSearchQuery] = useState("");
  
  // Invite Code State
  const [isInviteOpen, setIsInviteOpen] = useState(false);
  const [inviteData, setInviteData] = useState<InviteCodeResponse | null>(null);
  const [qrCodeUrl, setQrCodeUrl] = useState<string>("");
  const [generatingInvite, setGeneratingInvite] = useState(false);
  const [copied, setCopied] = useState(false);

  // Role Edit State
  const [editingStaff, setEditingStaff] = useState<StaffResponse | null>(null);
  const [newRole, setNewRole] = useState<string>("");
  const [updatingRole, setUpdatingRole] = useState(false);

  // Delete State
  const [deletingStaff, setDeletingStaff] = useState<StaffResponse | null>(null);

  useEffect(() => {
    if (activeMerchant?.id) {
      loadStaff();
    }
  }, [activeMerchant?.id]);

  const loadStaff = async () => {
    if (!activeMerchant?.id) return;
    setLoading(true);
    try {
      const data = await apiGet<ListMerchantStaffResponse>("/merchant/staff");
      setStaff(data.staff || []);
    } catch (error: any) {
      toast.error(error.message || "加载员工列表失败");
    } finally {
      setLoading(false);
    }
  };

  const generateInvite = async () => {
    if (!activeMerchant?.id) return;
    setGeneratingInvite(true);
    try {
      const data = await apiPost<InviteCodeResponse>("/merchant/staff/invite-code");
      setInviteData(data);
      
      // Generate QR Code
      const qrColor = { dark: '#000000', light: '#ffffff' };
      const url = await QRCode.toDataURL(`invite-merchant:${data.invite_code}`, {
        width: 300,
        margin: 2,
        color: qrColor
      });
      setQrCodeUrl(url);
      setIsInviteOpen(true);
    } catch (error: any) {
      toast.error(error.message || "生成邀请码失败");
    } finally {
      setGeneratingInvite(false);
    }
  };

  const handleUpdateRole = async () => {
    if (!editingStaff || !newRole) return;
    setUpdatingRole(true);
    try {
      await apiPatch(`/merchant/staff/${editingStaff.id}/role`, { role: newRole });
      toast.success("员工角色更新成功");
      setEditingStaff(null);
      loadStaff();
    } catch (error: any) {
      toast.error(error.message || "操作失败");
    } finally {
      setUpdatingRole(false);
    }
  };

  const handleRemoveStaff = async () => {
    if (!deletingStaff) return;
    try {
      await apiDelete(`/merchant/staff/${deletingStaff.id}`);
      toast.success("员工已移除（已设置为离职状态）");
      setDeletingStaff(null);
      loadStaff();
    } catch (error: any) {
      toast.error(error.message || "操作失败");
    }
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    toast.success("邀请码已复制");
    setTimeout(() => setCopied(false), 2000);
  };

  const filteredStaff = useMemo(() => {
    if (!searchQuery) return staff;
    const q = searchQuery.toLowerCase();
    return staff.filter(s => 
      s.full_name?.toLowerCase().includes(q) || 
      (ROLE_MAP[s.role]?.label.toLowerCase().includes(q))
    );
  }, [staff, searchQuery]);

  // 获取当前用户的权限等级 (是否为老板)
  const isOwner = session?.roles?.includes("owner") || staff.find(s => s.user_id === session?.user?.id)?.role === "owner";

  return (
    <PageShell>
      <PageHeader 
        title="员工管理" 
        description="管理商户内部人员及其权限，支持店助、收银、厨师长等角色协作"
        actions={
          <div className="flex gap-2">
            <Button size="sm" onClick={generateInvite} disabled={generatingInvite}>
              <UserPlus className={cn("h-4 w-4 mr-2", generatingInvite && "animate-spin")} />
              邀请新员工
            </Button>
            <Button variant="outline" size="sm" onClick={loadStaff}>
              <RefreshCw className={cn("h-4 w-4 mr-2", loading && "animate-spin")} />
              刷新
            </Button>
          </div>
        }
      />
      <PageContent>
        <div className="space-y-6">
          <div className="flex flex-col md:flex-row gap-4 items-center justify-between">
            <div className="relative w-full md:w-96">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
              <Input
                placeholder="搜索员工姓名或职位..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="pl-9 bg-white"
              />
            </div>
            <div className="flex items-center gap-6">
              <div className="flex items-center gap-2 text-xs font-medium text-muted-foreground">
                <span className="flex items-center gap-1"><div className="w-2 h-2 rounded-full bg-primary" /> 在职 {staff.filter(s=>s.status==='active').length}</span>
                <span className="flex items-center gap-1"><div className="w-2 h-2 rounded-full bg-slate-300" /> 离职 {staff.filter(s=>s.status==='disabled').length}</span>
              </div>
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6">
            {loading && staff.length === 0 ? (
              Array.from({ length: 6 }).map((_, i) => (
                <div key={i} className="h-48 rounded-2xl border bg-white animate-pulse" />
              ))
            ) : filteredStaff.length === 0 ? (
              <div className="col-span-full py-20 text-center bg-slate-50 rounded-3xl border-2 border-dashed">
                <Users className="h-12 w-12 mx-auto text-slate-300 mb-4" />
                <h3 className="text-lg font-bold text-slate-900">暂无员工信息</h3>
                <p className="text-sm text-muted-foreground mt-1">您可以点击右上角邀请新员工加入</p>
                <Button variant="outline" className="mt-6" onClick={generateInvite}>立即邀请</Button>
              </div>
            ) : (
              filteredStaff.map((member) => {
                const RoleIcon = ROLE_MAP[member.role]?.icon || User;
                const roleInfo = ROLE_MAP[member.role];
                const statusInfo = STATUS_MAP[member.status];
                
                return (
                  <div 
                    key={member.id} 
                    className={cn(
                      "group relative bg-white border rounded-3xl p-6 transition-all hover:shadow-xl hover:border-primary/20",
                      member.status === 'disabled' && "opacity-60 grayscale"
                    )}
                  >
                    <div className="flex justify-between items-start mb-4">
                      <div className="relative">
                        <div className="w-16 h-16 rounded-2xl bg-slate-100 flex items-center justify-center overflow-hidden border-2 border-white shadow-sm ring-1 ring-slate-100">
                           {member.avatar_url ? (
                             <img src={getMediaUrl(member.avatar_url)} alt="" className="w-full h-full object-cover" />
                           ) : (
                             <User className="h-8 w-8 text-slate-300" />
                           )}
                        </div>
                        {member.role === 'owner' && (
                          <div className="absolute -top-1 -right-1 bg-amber-500 text-white p-1 rounded-lg border-2 border-white shadow-sm">
                            <Crown className="h-3 w-3" />
                          </div>
                        )}
                      </div>

                      {member.role !== 'owner' && isOwner && (
                        <DropdownMenu>
                          <DropdownMenuTrigger asChild>
                            <Button variant="ghost" size="icon" className="h-8 w-8 rounded-full">
                              <MoreVertical className="h-4 w-4" />
                            </Button>
                          </DropdownMenuTrigger>
                          <DropdownMenuContent align="end" className="w-48 rounded-xl p-2">
                            <DropdownMenuLabel className="text-[10px] font-black uppercase text-muted-foreground px-3 py-2">管理员工</DropdownMenuLabel>
                            <DropdownMenuItem 
                              className="rounded-lg gap-2 cursor-pointer"
                              onClick={() => {
                                setEditingStaff(member);
                                setNewRole(member.role);
                              }}
                            >
                              <Edit className="h-4 w-4 text-primary" /> 修改权限角色
                            </DropdownMenuItem>
                            <DropdownMenuSeparator />
                            <DropdownMenuItem 
                              className="rounded-lg gap-2 text-rose-600 focus:text-rose-600 cursor-pointer"
                              onClick={() => setDeletingStaff(member)}
                            >
                              <Trash2 className="h-4 w-4" /> 移除员工身份
                            </DropdownMenuItem>
                          </DropdownMenuContent>
                        </DropdownMenu>
                      )}
                    </div>

                    <div className="space-y-1">
                      <div className="flex items-center gap-2">
                        <h3 className="font-black text-slate-900 truncate max-w-[120px]">{member.full_name}</h3>
                        <Badge variant={statusInfo.variant} className="text-[10px] h-5 px-1.5 rounded-md leading-none">
                          {statusInfo.label}
                        </Badge>
                      </div>
                      <p className="text-[10px] text-muted-foreground font-mono">UID: {member.user_id}</p>
                    </div>

                    <div className="mt-6 pt-6 border-t flex items-center justify-between">
                      <div className={cn("flex items-center gap-2 px-3 py-1.5 rounded-xl border", roleInfo.bg, roleInfo.color.replace('text', 'border').replace('600', '100'))}>
                        <RoleIcon className={cn("h-4 w-4", roleInfo.color)} />
                        <span className={cn("text-xs font-black", roleInfo.color)}>{roleInfo.label}</span>
                      </div>
                      <div className="text-right">
                        <p className="text-[10px] text-muted-foreground font-medium uppercase mb-0.5">入职时间</p>
                        <p className="text-[11px] font-bold text-slate-700">{new Date(member.created_at).toLocaleDateString()}</p>
                      </div>
                    </div>
                  </div>
                );
              })
            )}
          </div>
        </div>
      </PageContent>

      {/* Invite Code Dialog */}
      <Dialog open={isInviteOpen} onOpenChange={setIsInviteOpen}>
        <DialogContent className="sm:max-w-[420px] rounded-3xl p-0 overflow-hidden">
          <div className="bg-primary p-8 text-center text-white relative overflow-hidden">
             <div className="absolute top-0 right-0 w-32 h-32 bg-white/10 rounded-full -translate-y-1/2 translate-x-1/2" />
             <div className="absolute bottom-0 left-0 w-24 h-24 bg-white/10 rounded-full translate-y-1/2 -translate-x-1/2" />
             <ShieldCheck className="h-12 w-12 mx-auto mb-4 opacity-90" />
             <DialogTitle className="text-2xl font-black mb-2 text-white">员工注册邀请码</DialogTitle>
             <DialogDescription className="text-white/80 text-sm">让员工使用微信扫码，或在小程序员工绑定页面手动输入此代码</DialogDescription>
          </div>
          
          <div className="p-8 space-y-8">
            <div className="flex flex-col items-center">
              <div className="relative group p-4 bg-white rounded-3xl border-2 border-dashed border-slate-200">
                {qrCodeUrl ? (
                  <img src={qrCodeUrl} alt="Invite QR" className="w-48 h-48 rounded-xl" />
                ) : (
                  <div className="w-48 h-48 flex items-center justify-center"><RefreshCw className="h-8 w-8 animate-spin opacity-20" /></div>
                )}
                <div className="absolute inset-0 bg-white/80 flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity rounded-3xl">
                   <p className="text-xs font-black text-slate-900 border-b-2 border-primary pb-1">点击保存二维码</p>
                </div>
              </div>
              <p className="mt-4 text-[10px] font-bold text-muted-foreground uppercase tracking-widest flex items-center gap-1">
                <Clock className="h-3 w-3" /> 有效期至：{inviteData ? new Date(inviteData.expires_at).toLocaleString() : '-'}
              </p>
            </div>

            <div className="space-y-3">
              <Label className="text-xs font-black text-slate-500 uppercase">文本验证码</Label>
              <div className="flex gap-2">
                <div className="flex-1 bg-slate-50 border h-12 rounded-xl flex items-center px-4 font-black font-mono tracking-tighter text-slate-600">
                  {inviteData?.invite_code || "••••••••••••••••••••••••••••••••"}
                </div>
                <Button 
                  size="icon" 
                  variant="outline" 
                  className="h-12 w-12 rounded-xl"
                  onClick={() => inviteData && copyToClipboard(inviteData.invite_code)}
                >
                  {copied ? <Check className="h-4 w-4 text-emerald-500" /> : <Copy className="h-4 w-4" />}
                </Button>
              </div>
            </div>
          </div>
          
          <div className="p-4 bg-slate-50 border-t flex justify-center">
             <Button variant="ghost" onClick={() => setIsInviteOpen(false)} className="rounded-xl px-12 font-bold">已完成复制</Button>
          </div>
        </DialogContent>
      </Dialog>

      {/* Edit Role Dialog */}
      <Dialog open={!!editingStaff} onOpenChange={(open) => !open && setEditingStaff(null)}>
        <DialogContent className="sm:max-w-[420px] rounded-3xl">
          <DialogHeader>
            <DialogTitle className="text-xl font-black">分配员工权限</DialogTitle>
            <DialogDescription>
              配置“{editingStaff?.full_name}”在商户内的操作权限级别。不同的角色将对应不同的后端功能访问权限。
            </DialogDescription>
          </DialogHeader>
          <div className="py-6 space-y-6">
            <div className="flex flex-col gap-4">
               {[
                 { id: 'manager', label: '店长', desc: '拥有除老板外的最高权限，可管理员工和全店设置' },
                 { id: 'chef', label: '厨师长', desc: '主要负责 KDS 系统、菜品管理及库存统筹' },
                 { id: 'cashier', label: '收银员', desc: '负责台位状态管理、账单核销及前台订单处理' }
               ].map(role => (
                 <div 
                   key={role.id}
                   onClick={() => setNewRole(role.id)}
                   className={cn(
                     "relative p-4 rounded-2xl border-2 transition-all cursor-pointer flex items-start gap-4 hover:border-primary/30",
                     newRole === role.id ? "border-primary bg-primary/5 ring-1 ring-primary/20" : "border-slate-100 bg-white"
                   )}
                 >
                    <div className={cn(
                      "w-10 h-10 rounded-xl flex items-center justify-center shrink-0",
                      ROLE_MAP[role.id].bg
                    )}>
                       {(() => { const Icon = ROLE_MAP[role.id].icon; return <Icon className={cn("h-5 w-5", ROLE_MAP[role.id].color)} /> })()}
                    </div>
                    <div className="space-y-0.5">
                      <p className="font-black text-slate-900">{role.label}</p>
                      <p className="text-xs text-muted-foreground">{role.desc}</p>
                    </div>
                    {newRole === role.id && (
                      <div className="absolute top-4 right-4 bg-primary text-white rounded-full p-0.5">
                        <Check className="h-3 w-3" />
                      </div>
                    )}
                 </div>
               ))}
            </div>
          </div>
          <DialogFooter className="bg-slate-50 -mx-6 -mb-6 p-4 border-t rounded-b-3xl">
            <Button variant="ghost" onClick={() => setEditingStaff(null)}>取消</Button>
            <Button 
              className="rounded-xl px-12 font-bold shadow-lg shadow-primary/20" 
              onClick={handleUpdateRole}
              disabled={updatingRole}
            >
              {updatingRole && <RefreshCw className="mr-2 h-4 w-4 animate-spin" />}
              确认变更角色
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog 
        open={!!deletingStaff}
        onOpenChange={(open) => !open && setDeletingStaff(null)}
        title="确认移除员工身份？"
        description={`正在移除员工“${deletingStaff?.full_name}”，移除后该用户将失去访问商户后台的权限。该操作会将员工状态标记为“已离职”，但保留历史数据记录。`}
        confirmText="确认移除"
        variant="destructive"
        onConfirm={handleRemoveStaff}
      />
    </PageShell>
  );
}
