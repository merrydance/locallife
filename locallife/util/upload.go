package util

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	// 上传文件根目录
	UploadBaseDir = "uploads"
	// 允许的图片格式
	AllowedImageExts = ".jpg,.jpeg,.png"
	// 最大文件大小 (10MB)
	MaxFileSize = 10 * 1024 * 1024
	// OCR专用最大文件大小 (2MB) - 与微信OCR接口限制对齐
	MaxOCRFileSize = 2 * 1024 * 1024
)

// ErrImageTooLargeForOCR 图片超过OCR大小限制
var ErrImageTooLargeForOCR = errors.New("图片大小超过限制：最大2MB")

// ErrInvalidImageFormat 无效的图片格式（magic number不匹配）
var ErrInvalidImageFormat = errors.New("无效的图片格式，请上传JPG或PNG格式的图片")

// ValidateImageMagic 验证图片文件的 magic number（文件头）
// 确保上传的文件确实是图片，而不是伪造扩展名的其他文件
func ValidateImageMagic(file multipart.File) (string, error) {
	// 读取文件头（512字节足够检测大多数格式）
	header := make([]byte, 512)
	n, err := file.Read(header)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("读取文件头失败: %w", err)
	}

	// 重置文件读取位置
	if seeker, ok := file.(io.Seeker); ok {
		if _, err := seeker.Seek(0, io.SeekStart); err != nil {
			return "", fmt.Errorf("重置文件位置失败: %w", err)
		}
	}

	// 使用标准库检测 Content-Type
	contentType := http.DetectContentType(header[:n])

	// 只允许 JPEG 和 PNG
	switch contentType {
	case "image/jpeg":
		return ".jpg", nil
	case "image/png":
		return ".png", nil
	default:
		return "", ErrInvalidImageFormat
	}
}

// UploadConfig 上传配置
type UploadConfig struct {
	BaseDir    string
	MaxSize    int64
	AllowedExt string
}

// FileUploader 文件上传器
type FileUploader struct {
	config UploadConfig
}

// NewFileUploader 创建文件上传器
func NewFileUploader(baseDir string) *FileUploader {
	if baseDir == "" {
		baseDir = UploadBaseDir
	}
	return &FileUploader{
		config: UploadConfig{
			BaseDir:    baseDir,
			MaxSize:    MaxFileSize,
			AllowedExt: AllowedImageExts,
		},
	}
}

// UploadMerchantImage 上传商户相关图片
// category: business_license, id_front, id_back, logo
func (u *FileUploader) UploadMerchantImage(userID int64, category string, file multipart.File, header *multipart.FileHeader) (string, error) {
	// 验证文件大小
	if header.Size > u.config.MaxSize {
		return "", fmt.Errorf("file size exceeds limit: %d bytes (max: %d)", header.Size, u.config.MaxSize)
	}

	// 验证文件格式
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !strings.Contains(u.config.AllowedExt, ext) {
		return "", fmt.Errorf("invalid file extension: %s (allowed: %s)", ext, u.config.AllowedExt)
	}

	// 生成文件路径
	// 特殊处理 Logo: 存入公共目录 uploads/public/merchants/{user_id}/logo
	var dir string
	if category == "logo" {
		dir = filepath.Join(u.config.BaseDir, "public", "merchants", fmt.Sprintf("%d", userID), category)
	} else {
		// 其他私有证照: uploads/merchants/{user_id}/{category}
		dir = filepath.Join(u.config.BaseDir, "merchants", fmt.Sprintf("%d", userID), category)
	}

	// 创建目录（如果不存在）
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// 生成唯一文件名
	filename := fmt.Sprintf("%s_%d%s", uuid.New().String(), time.Now().Unix(), ext)
	filePath := filepath.Join(dir, filename)

	// 创建目标文件
	dst, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	// 复制文件内容
	if _, err := io.Copy(dst, file); err != nil {
		// 如果复制失败，删除已创建的文件
		os.Remove(filePath)
		return "", fmt.Errorf("failed to copy file: %w", err)
	}

	// 返回相对路径 (强制使用斜杠 /, 解决 Windows 反斜杠问题)
	relativePath := filepath.Join(dir, filename)
	return filepath.ToSlash(relativePath), nil
}

// UploadMerchantImageForOCR 上传用于OCR识别的商户证件图片
// 与 UploadMerchantImage 不同，此方法：
// 1. 限制文件大小为 2MB（与微信OCR接口限制对齐）
// 2. 验证文件真实格式（magic number），防止伪造扩展名
// category: business_license, food_permit, id_front, id_back
func (u *FileUploader) UploadMerchantImageForOCR(userID int64, category string, file multipart.File, header *multipart.FileHeader) (string, error) {
	// 验证文件大小（OCR专用限制：2MB）
	if header.Size > MaxOCRFileSize {
		return "", ErrImageTooLargeForOCR
	}

	// 验证文件真实格式（magic number）
	detectedExt, err := ValidateImageMagic(file)
	if err != nil {
		return "", err
	}

	// 生成文件路径: uploads/merchants/{user_id}/{category}/{uuid}{ext}
	dir := filepath.Join(u.config.BaseDir, "merchants", fmt.Sprintf("%d", userID), category)

	// 创建目录（如果不存在）
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// 使用检测到的扩展名（而非用户提供的），确保一致性
	filename := fmt.Sprintf("%s_%d%s", uuid.New().String(), time.Now().Unix(), detectedExt)
	filePath := filepath.Join(dir, filename)

	// 创建目标文件
	dst, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	// 复制文件内容
	if _, err := io.Copy(dst, file); err != nil {
		os.Remove(filePath)
		return "", fmt.Errorf("failed to copy file: %w", err)
	}

	// 返回相对路径
	relativePath := filepath.Join(dir, filename)
	return filepath.ToSlash(relativePath), nil
}

// UploadRiderImage 上传骑手相关图片
// category: idcard, healthcert
func (u *FileUploader) UploadRiderImage(userID int64, category string, file multipart.File, header *multipart.FileHeader) (string, error) {
	// 验证文件大小
	if header.Size > u.config.MaxSize {
		return "", fmt.Errorf("file size exceeds limit: %d bytes (max: %d)", header.Size, u.config.MaxSize)
	}

	// 验证文件格式
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !strings.Contains(u.config.AllowedExt, ext) {
		return "", fmt.Errorf("invalid file extension: %s (allowed: %s)", ext, u.config.AllowedExt)
	}

	// 生成文件路径: uploads/riders/{user_id}/{category}/{uuid}{ext}
	dir := filepath.Join(u.config.BaseDir, "riders", fmt.Sprintf("%d", userID), category)

	// 创建目录（如果不存在）
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// 生成唯一文件名
	filename := fmt.Sprintf("%s_%d%s", uuid.New().String(), time.Now().Unix(), ext)
	filePath := filepath.Join(dir, filename)

	// 创建目标文件
	dst, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	// 复制文件内容
	if _, err := io.Copy(dst, file); err != nil {
		os.Remove(filePath)
		return "", fmt.Errorf("failed to copy file: %w", err)
	}

	// 返回相对路径
	relativePath := filepath.Join(u.config.BaseDir, "riders", fmt.Sprintf("%d", userID), category, filename)
	return filepath.ToSlash(relativePath), nil
}

// UploadOperatorImage 上传运营商相关图片
// category: license, idcard_Front, idcard_Back
func (u *FileUploader) UploadOperatorImage(userID int64, category string, file multipart.File, header *multipart.FileHeader) (string, error) {
	// 验证文件大小
	if header.Size > u.config.MaxSize {
		return "", fmt.Errorf("file size exceeds limit: %d bytes (max: %d)", header.Size, u.config.MaxSize)
	}

	// 验证文件格式
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !strings.Contains(u.config.AllowedExt, ext) {
		return "", fmt.Errorf("invalid file extension: %s (allowed: %s)", ext, u.config.AllowedExt)
	}

	// 生成文件路径: uploads/operators/{user_id}/{category}/{uuid}{ext}
	dir := filepath.Join(u.config.BaseDir, "operators", fmt.Sprintf("%d", userID), category)

	// 创建目录（如果不存在）
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// 生成唯一文件名
	filename := fmt.Sprintf("%s_%d%s", uuid.New().String(), time.Now().Unix(), ext)
	filePath := filepath.Join(dir, filename)

	// 创建目标文件
	dst, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	// 复制文件内容
	if _, err := io.Copy(dst, file); err != nil {
		os.Remove(filePath)
		return "", fmt.Errorf("failed to copy file: %w", err)
	}

	// 返回相对路径
	relativePath := filepath.Join(u.config.BaseDir, "operators", fmt.Sprintf("%d", userID), category, filename)
	return filepath.ToSlash(relativePath), nil
}

// UploadReviewImage 上传用户评价图片
// dir: uploads/reviews/{user_id}/
func (u *FileUploader) UploadReviewImage(userID int64, file multipart.File, header *multipart.FileHeader) (string, error) {
	// 验证文件大小
	if header.Size > u.config.MaxSize {
		return "", fmt.Errorf("file size exceeds limit: %d bytes (max: %d)", header.Size, u.config.MaxSize)
	}

	// 验证文件格式
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !strings.Contains(u.config.AllowedExt, ext) {
		return "", fmt.Errorf("invalid file extension: %s (allowed: %s)", ext, u.config.AllowedExt)
	}

	// 生成文件路径: uploads/reviews/{user_id}/{uuid}{ext}
	dir := filepath.Join(u.config.BaseDir, "reviews", fmt.Sprintf("%d", userID))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	filename := fmt.Sprintf("%s_%d%s", uuid.New().String(), time.Now().Unix(), ext)
	filePath := filepath.Join(dir, filename)

	dst, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		os.Remove(filePath)
		return "", fmt.Errorf("failed to copy file: %w", err)
	}

	relativePath := filepath.Join(u.config.BaseDir, "reviews", fmt.Sprintf("%d", userID), filename)
	return filepath.ToSlash(relativePath), nil
}

// UploadPublicMerchantAssetImage 上传用于对外展示的商户素材图片（菜品/桌台/包间等）。
// dir: uploads/public/merchants/{merchant_id}/{category}/
func (u *FileUploader) UploadPublicMerchantAssetImage(merchantID int64, category string, file multipart.File, header *multipart.FileHeader) (string, error) {
	if header.Size > u.config.MaxSize {
		return "", fmt.Errorf("file size exceeds limit: %d bytes (max: %d)", header.Size, u.config.MaxSize)
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !strings.Contains(u.config.AllowedExt, ext) {
		return "", fmt.Errorf("invalid file extension: %s (allowed: %s)", ext, u.config.AllowedExt)
	}

	dir := filepath.Join(u.config.BaseDir, "public", "merchants", fmt.Sprintf("%d", merchantID), category)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	filename := fmt.Sprintf("%s_%d%s", uuid.New().String(), time.Now().Unix(), ext)
	filePath := filepath.Join(dir, filename)

	dst, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		os.Remove(filePath)
		return "", fmt.Errorf("failed to copy file: %w", err)
	}

	relativePath := filepath.Join(u.config.BaseDir, "public", "merchants", fmt.Sprintf("%d", merchantID), category, filename)
	return filepath.ToSlash(relativePath), nil
}

// DeleteFile 删除文件
func (u *FileUploader) DeleteFile(relativePath string) error {
	filePath := filepath.Join(u.config.BaseDir, relativePath)

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", relativePath)
	}

	// 删除文件
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// GetFilePath 获取文件的完整路径
func (u *FileUploader) GetFilePath(relativePath string) string {
	return filepath.Join(u.config.BaseDir, relativePath)
}

// FileExists 检查文件是否存在
func (u *FileUploader) FileExists(relativePath string) bool {
	filePath := filepath.Join(u.config.BaseDir, relativePath)
	_, err := os.Stat(filePath)
	return err == nil
}

// SaveQRCodeImage 保存二维码图片（PNG格式）
// dir: uploads/public/merchants/{merchant_id}/qrcodes/
func (u *FileUploader) SaveQRCodeImage(merchantID int64, filename string, pngData []byte) (string, error) {
	// 生成目录路径
	dir := filepath.Join(u.config.BaseDir, "public", "merchants", fmt.Sprintf("%d", merchantID), "qrcodes")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// 完整文件路径
	filePath := filepath.Join(dir, filename)

	// 写入文件
	if err := os.WriteFile(filePath, pngData, 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	// 返回相对路径
	relativePath := filepath.Join(u.config.BaseDir, "public", "merchants", fmt.Sprintf("%d", merchantID), "qrcodes", filename)
	return filepath.ToSlash(relativePath), nil
}
