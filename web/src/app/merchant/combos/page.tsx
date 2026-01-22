import { CombosPageClient } from "@/components/merchant/combos-page-client";

export default function CombosPage() {
  // 由于 Token 存储在 LocalStorage 中，服务端无法获取用户认证信息。
  // 因此，数据获取必须在客户端进行。
  // CombosPageClient组件已经包含 useEffect(() => loadCombos(), []) 逻辑，
  // 这里只需传递空数组作为初始数据即可。
  return (
    <CombosPageClient initialData={[]} />
  );
}
