"use client";

import React, { useState } from "react";
import NextImage, { type ImageProps } from "next/image";
import { getMediaDisplayUrl } from "@/lib/media";

const PLACEHOLDER = "/assets/placeholder.png";

export type MediaVariant = "thumb" | "card" | "detail" | "original";

export interface MediaImageProps extends Omit<ImageProps, "src"> {
  /** 后端返回的公共图片 URL */
  src?: string;
  /** 图片变体说明，仅用于文档注释，实际 URL 由后端决定 */
  variant?: MediaVariant;
}

/**
 * `<MediaImage>` — 通用媒体图片组件。
 *
 * 封装 Next.js `<Image>` 的标准用法：
 * - 自动调用 `getMediaDisplayUrl` 解析公共展示 URL
 * - `src` 为空时显示占位图
 * - 加载出错时回退到占位图
 */
export function MediaImage({ src, alt = "", variant: _variant, ...rest }: MediaImageProps) {
  const resolved = getMediaDisplayUrl(src) || PLACEHOLDER;
  const [imgSrc, setImgSrc] = useState(resolved);

  return (
    <NextImage
      {...rest}
      src={imgSrc}
      alt={alt}
      onError={() => setImgSrc(PLACEHOLDER)}
    />
  );
}
