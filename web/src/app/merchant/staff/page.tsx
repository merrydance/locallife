import { StaffPageClient } from "@/components/merchant/staff-page-client";

export const metadata = {
  title: "员工管理 - 商户管理后台",
  description: "管理商户员工角色与权限",
};

export default function MerchantStaffPage() {
  return <StaffPageClient />;
}
