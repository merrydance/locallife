package util

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"

	"github.com/rs/zerolog/log"
)

const (
	// DefaultMaxOCRBytes OCR图片最大字节数（2MB）
	DefaultMaxOCRBytes int64 = 2 * 1024 * 1024
	// MinJPEGQuality JPEG压缩最低质量
	MinJPEGQuality = 30
	// DefaultJPEGQuality 默认JPEG质量
	DefaultJPEGQuality = 85
)

// ImageCompressor 图片压缩器
type ImageCompressor struct {
	maxBytes int64
}

// NewImageCompressor 创建图片压缩器
func NewImageCompressor(maxBytes int64) *ImageCompressor {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxOCRBytes
	}
	return &ImageCompressor{maxBytes: maxBytes}
}

// CompressFileForOCR 压缩图片文件以满足OCR大小要求
// 如果文件已经小于限制，直接返回原文件内容
// 否则逐步降低质量压缩直到满足要求
// 返回：压缩后的图片数据、是否进行了压缩、错误
func (c *ImageCompressor) CompressFileForOCR(filePath string) ([]byte, bool, error) {
	// 读取原始文件
	originalData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, false, fmt.Errorf("读取图片文件失败: %w", err)
	}

	// 如果文件大小已经符合要求，直接返回
	if int64(len(originalData)) <= c.maxBytes {
		return originalData, false, nil
	}

	log.Info().
		Str("file", filePath).
		Int("original_size", len(originalData)).
		Int64("max_size", c.maxBytes).
		Msg("图片超过OCR大小限制，开始压缩")

	// 检测图片格式
	contentType := http.DetectContentType(originalData)

	// 解码图片
	var img image.Image
	reader := bytes.NewReader(originalData)

	switch contentType {
	case "image/jpeg":
		img, err = jpeg.Decode(reader)
	case "image/png":
		img, err = png.Decode(reader)
	default:
		return nil, false, fmt.Errorf("不支持的图片格式: %s", contentType)
	}
	if err != nil {
		return nil, false, fmt.Errorf("解码图片失败: %w", err)
	}

	// 逐步降低质量压缩
	for quality := DefaultJPEGQuality; quality >= MinJPEGQuality; quality -= 10 {
		compressed, err := c.compressToJPEG(img, quality)
		if err != nil {
			return nil, false, fmt.Errorf("压缩图片失败: %w", err)
		}

		if int64(len(compressed)) <= c.maxBytes {
			log.Info().
				Str("file", filePath).
				Int("original_size", len(originalData)).
				Int("compressed_size", len(compressed)).
				Int("quality", quality).
				Msg("图片压缩完成")
			return compressed, true, nil
		}
	}

	// 如果降低质量仍然无法满足，尝试缩小尺寸
	resized := c.resizeImage(img, 0.7) // 缩小到70%
	for quality := DefaultJPEGQuality; quality >= MinJPEGQuality; quality -= 10 {
		compressed, err := c.compressToJPEG(resized, quality)
		if err != nil {
			return nil, false, fmt.Errorf("压缩图片失败: %w", err)
		}

		if int64(len(compressed)) <= c.maxBytes {
			log.Info().
				Str("file", filePath).
				Int("original_size", len(originalData)).
				Int("compressed_size", len(compressed)).
				Int("quality", quality).
				Float64("scale", 0.7).
				Msg("图片压缩完成（含缩放）")
			return compressed, true, nil
		}
	}

	// 最后尝试更大幅度缩小
	resized = c.resizeImage(img, 0.5) // 缩小到50%
	compressed, err := c.compressToJPEG(resized, MinJPEGQuality)
	if err != nil {
		return nil, false, fmt.Errorf("压缩图片失败: %w", err)
	}

	if int64(len(compressed)) <= c.maxBytes {
		log.Info().
			Str("file", filePath).
			Int("original_size", len(originalData)).
			Int("compressed_size", len(compressed)).
			Msg("图片压缩完成（大幅缩放）")
		return compressed, true, nil
	}

	return nil, false, fmt.Errorf("无法将图片压缩到 %d 字节以内", c.maxBytes)
}

// compressToJPEG 将图片压缩为JPEG格式
func (c *ImageCompressor) compressToJPEG(img image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// resizeImage 缩放图片
func (c *ImageCompressor) resizeImage(img image.Image, scale float64) image.Image {
	bounds := img.Bounds()
	newWidth := int(float64(bounds.Dx()) * scale)
	newHeight := int(float64(bounds.Dy()) * scale)

	// 使用简单的最近邻采样缩放（对于OCR足够）
	dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))

	for y := 0; y < newHeight; y++ {
		for x := 0; x < newWidth; x++ {
			srcX := int(float64(x) / scale)
			srcY := int(float64(y) / scale)
			dst.Set(x, y, img.At(srcX+bounds.Min.X, srcY+bounds.Min.Y))
		}
	}

	return dst
}

// CompressReaderForOCR 压缩 io.Reader 中的图片数据
// 返回压缩后的 io.Reader
func (c *ImageCompressor) CompressReaderForOCR(r io.Reader) (io.Reader, bool, error) {
	// 先读取全部数据
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, false, fmt.Errorf("读取图片数据失败: %w", err)
	}

	// 如果大小符合要求，直接返回
	if int64(len(data)) <= c.maxBytes {
		return bytes.NewReader(data), false, nil
	}

	// 检测图片格式
	contentType := http.DetectContentType(data)

	// 解码图片
	var img image.Image
	reader := bytes.NewReader(data)

	switch contentType {
	case "image/jpeg":
		img, err = jpeg.Decode(reader)
	case "image/png":
		img, err = png.Decode(reader)
	default:
		return nil, false, fmt.Errorf("不支持的图片格式: %s", contentType)
	}
	if err != nil {
		return nil, false, fmt.Errorf("解码图片失败: %w", err)
	}

	// 逐步降低质量压缩
	for quality := DefaultJPEGQuality; quality >= MinJPEGQuality; quality -= 10 {
		compressed, err := c.compressToJPEG(img, quality)
		if err != nil {
			return nil, false, fmt.Errorf("压缩图片失败: %w", err)
		}

		if int64(len(compressed)) <= c.maxBytes {
			return bytes.NewReader(compressed), true, nil
		}
	}

	// 尝试缩小尺寸
	resized := c.resizeImage(img, 0.7)
	compressed, err := c.compressToJPEG(resized, MinJPEGQuality)
	if err != nil {
		return nil, false, fmt.Errorf("压缩图片失败: %w", err)
	}

	if int64(len(compressed)) <= c.maxBytes {
		return bytes.NewReader(compressed), true, nil
	}

	return nil, false, fmt.Errorf("无法将图片压缩到 %d 字节以内", c.maxBytes)
}

// BytesFile 实现 multipart.File 接口的字节文件包装器
// 用于将压缩后的字节数据传递给需要 multipart.File 的接口
type BytesFile struct {
	*bytes.Reader
}

// NewBytesFile 创建 BytesFile
func NewBytesFile(data []byte) *BytesFile {
	return &BytesFile{Reader: bytes.NewReader(data)}
}

// Close 实现 io.Closer 接口
func (f *BytesFile) Close() error {
	return nil
}

// ReadAt 实现 io.ReaderAt 接口
func (f *BytesFile) ReadAt(p []byte, off int64) (n int, err error) {
	return f.Reader.ReadAt(p, off)
}
