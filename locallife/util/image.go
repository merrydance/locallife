package util

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
)

// WebP 转换配置
const (
	DefaultWebPQuality    = 85   // 质量 0-100，85 是高质量且压缩率好的平衡点
	DefaultMaxImageWidth  = 1200 // 最大宽度（像素）
	DefaultMaxImageHeight = 1200 // 最大高度（像素）
)

// WebPConverter WebP 图片转换器
type WebPConverter struct {
	Quality   int  // 质量 0-100
	MaxWidth  int  // 最大宽度，0 表示不限制
	MaxHeight int  // 最大高度，0 表示不限制
	Lossless  bool // 是否使用无损压缩
}

// NewWebPConverter 创建默认配置的转换器
func NewWebPConverter() *WebPConverter {
	return &WebPConverter{
		Quality:   DefaultWebPQuality,
		MaxWidth:  DefaultMaxImageWidth,
		MaxHeight: DefaultMaxImageHeight,
		Lossless:  false,
	}
}

// ConvertToWebP 将图片转换为 WebP 格式
// inputPath: 输入图片路径
// outputPath: 输出 WebP 路径（如果为空，则自动生成 .webp 后缀的路径）
// 返回输出文件路径和错误
func (c *WebPConverter) ConvertToWebP(inputPath string, outputPath string) (string, error) {
	// 检查 cwebp 是否可用
	if _, err := exec.LookPath("cwebp"); err != nil {
		return "", fmt.Errorf("cwebp not found in PATH: %w", err)
	}

	// 检查输入文件是否存在
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		return "", fmt.Errorf("input file not found: %s", inputPath)
	}

	// 自动生成输出路径
	if outputPath == "" {
		ext := filepath.Ext(inputPath)
		outputPath = strings.TrimSuffix(inputPath, ext) + ".webp"
	}

	// 构建 cwebp 命令参数
	args := []string{}

	// 质量设置
	if c.Lossless {
		args = append(args, "-lossless")
	} else {
		args = append(args, "-q", fmt.Sprintf("%d", c.Quality))
	}

	// 尺寸限制（等比例缩放，保持宽高比）
	if c.MaxWidth > 0 && c.MaxHeight > 0 {
		args = append(args, "-resize", fmt.Sprintf("%d", c.MaxWidth), fmt.Sprintf("%d", c.MaxHeight))
	} else if c.MaxWidth > 0 {
		args = append(args, "-resize", fmt.Sprintf("%d", c.MaxWidth), "0")
	} else if c.MaxHeight > 0 {
		args = append(args, "-resize", "0", fmt.Sprintf("%d", c.MaxHeight))
	}

	// 输入输出
	args = append(args, inputPath, "-o", outputPath)

	// 执行转换
	cmd := exec.Command("cwebp", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("cwebp failed: %w, stderr: %s", err, stderr.String())
	}

	// 记录压缩效果
	inputInfo, _ := os.Stat(inputPath)
	outputInfo, _ := os.Stat(outputPath)
	if inputInfo != nil && outputInfo != nil {
		reduction := float64(inputInfo.Size()-outputInfo.Size()) / float64(inputInfo.Size()) * 100
		log.Info().
			Str("input", filepath.Base(inputPath)).
			Int64("input_size", inputInfo.Size()).
			Int64("output_size", outputInfo.Size()).
			Float64("reduction_percent", reduction).
			Msg("WebP conversion completed")
	}

	return outputPath, nil
}

// ConvertReaderToWebP 从 Reader 读取图片并转换为 WebP
// 返回 WebP 数据的 Reader
func (c *WebPConverter) ConvertReaderToWebP(src io.Reader, originalExt string) ([]byte, error) {
	// 创建临时输入文件
	tmpInput, err := os.CreateTemp("", "webp_input_*"+originalExt)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp input file: %w", err)
	}
	defer os.Remove(tmpInput.Name())
	defer tmpInput.Close()

	// 写入输入数据
	if _, err := io.Copy(tmpInput, src); err != nil {
		return nil, fmt.Errorf("failed to write temp input: %w", err)
	}
	tmpInput.Close()

	// 创建临时输出文件
	tmpOutput, err := os.CreateTemp("", "webp_output_*.webp")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp output file: %w", err)
	}
	defer os.Remove(tmpOutput.Name())
	tmpOutput.Close()

	// 执行转换
	if _, err := c.ConvertToWebP(tmpInput.Name(), tmpOutput.Name()); err != nil {
		return nil, err
	}

	// 读取输出文件
	result, err := os.ReadFile(tmpOutput.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to read output file: %w", err)
	}

	return result, nil
}

// IsWebPSupported 检查系统是否支持 WebP 转换
func IsWebPSupported() bool {
	_, err := exec.LookPath("cwebp")
	return err == nil
}

// CanConvertToWebP 检查文件扩展名是否可以转换为 WebP
func CanConvertToWebP(ext string) bool {
	ext = strings.ToLower(ext)
	supportedExts := []string{".jpg", ".jpeg", ".png", ".gif", ".tiff", ".tif"}
	for _, supported := range supportedExts {
		if ext == supported {
			return true
		}
	}
	return false
}
