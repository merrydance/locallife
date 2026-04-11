// Package media 提供媒体资产管理能力：OSS 直传凭证签发、媒体元数据注册、
// URL 解析（规格图 CDN 地址、私有签名地址）。
//
// 四层结构：
//   - ObjectStorage：与具体存储后端交互（阿里云 OSS / 本地开发模式）
//   - MediaPolicy：校验上传策略，决定 object_key 前缀与可见性
//   - MediaURLResolver：将 object_key 转换为客户端可用的 URL
//   - MediaRegistry：媒体元数据 CRUD、上传会话状态机
package media
