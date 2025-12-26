package wechat

import (
	"bytes"
	"context"
	"errors"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"path/filepath"
	"strings"
	"time"
)

const (
	imgSecCheckURL = "https://api.weixin.qq.com/wxa/img_sec_check?access_token=%s"
	// legacyImgSecCheckMaxBytes is the practical limit enforced by the legacy
	// /wxa/img_sec_check endpoint. Empirically and via errors like 40006
	legacyImgSecCheckMaxBytes int = 1 * 1024 * 1024
	// legacyImgSecCheckTargetBytes leaves headroom to avoid edge cases while
	// still giving typical ~1.2MB images a chance to be compressed under the
	// legacy ~1MB upstream limit.
	legacyImgSecCheckTargetBytes int = (1 * 1024 * 1024) - (16 * 1024) // ~1008KiB
)

type imgSecCheckResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

var ErrRiskyContent = errors.New("risky content")

// ImgSecCheck 图片内容安全检测（小程序内容安全）
// 注意：该接口官方提示旧版（1.0）停止维护，但仍可作为兜底能力；后续可升级为 mediaCheckAsync。
// 返回 nil 表示通过；返回 error 表示不通过或调用失败。
func (c *Client) ImgSecCheck(ctx context.Context, imgFile multipart.File) error {
	// 获取 access_token（使用 mp 类型）
	token, err := c.GetAccessToken(ctx, "mp")
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	url := fmt.Sprintf(imgSecCheckURL, token)

	imgData, err := readAllLimited(imgFile, MaxImgSecCheckBytes)
	if err != nil {
		if errors.Is(err, ErrImageTooLarge) {
			return fmt.Errorf("%w: img_sec_check requires <= %d bytes", ErrImageTooLarge, MaxImgSecCheckBytes)
		}
		return fmt.Errorf("failed to read image file: %w", err)
	}

	// The legacy img_sec_check endpoint is strict about media size. If the
	// original upload is larger than ~1MB, compress it for the safety check
	// only; we still keep the original file for storage/OCR.
	compressed := false
	if len(imgData) > legacyImgSecCheckMaxBytes {
		if b, err := compressForLegacyImgSecCheck(imgData, legacyImgSecCheckTargetBytes); err == nil && len(b) > 0 {
			imgData = b
			compressed = true
		} else if err != nil {
			return fmt.Errorf("%w: %v", ErrImageTooLarge, err)
		}
	}

	contentType := http.DetectContentType(imgData)
	filename := "image"
	switch contentType {
	case "image/jpeg":
		filename += ".jpg"
	case "image/png":
		filename += ".png"
	default:
		// keep no ext for unknown types
		contentType = "application/octet-stream"
	}
	if compressed {
		// compressed output is always JPEG
		contentType = "image/jpeg"
		filename = "image.jpg"
	}
	filename = filepath.Base(filename)

	client := &http.Client{Timeout: 30 * time.Second}

	var respBody []byte
	for attempt := 1; attempt <= 3; attempt++ {
		// 构造 multipart/form-data（字段名固定为 media）
		// 使用 bytes.Buffer 代替 io.Pipe 以确保 http.Client 能够设置 Content-Length，
		// 这有助于解决微信 API 返回 412 (Precondition Failed) 的问题。
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)

		partHeader := make(textproto.MIMEHeader)
		partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="media"; filename="%s"`, filename))
		partHeader.Set("Content-Type", contentType)

		part, err := writer.CreatePart(partHeader)
		if err != nil {
			return fmt.Errorf("failed to create multipart part: %w", err)
		}

		if _, err := io.Copy(part, bytes.NewReader(imgData)); err != nil {
			return fmt.Errorf("failed to copy image data to multipart part: %w", err)
		}

		if err := writer.Close(); err != nil {
			return fmt.Errorf("failed to close multipart writer: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())

		resp, err := client.Do(req)
		if err != nil {
			if attempt < 3 && isRetryableImgSecCheckErr(ctx, err) {
				time.Sleep(time.Duration(attempt) * 200 * time.Millisecond)
				continue
			}
			return fmt.Errorf("failed to send request: %w", err)
		}

		defer resp.Body.Close()
		respBody, err = io.ReadAll(resp.Body)
		if err != nil {
			if attempt < 3 && isRetryableImgSecCheckErr(ctx, err) {
				time.Sleep(time.Duration(attempt) * 200 * time.Millisecond)
				continue
			}
			return fmt.Errorf("failed to read response body: %w", err)
		}

		if resp.StatusCode/100 != 2 {
			// 提供更详细的诊断信息，包括状态码和响应体
			err = fmt.Errorf("wechat img sec check failed: http_status=%d body=%s request_url=%s", resp.StatusCode, string(respBody), url)
			if attempt < 3 && isRetryableImgSecCheckErr(ctx, err) {
				time.Sleep(time.Duration(attempt) * 200 * time.Millisecond)
				continue
			}
			return err
		}
		break
	}

	var result imgSecCheckResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}
	if result.ErrCode != 0 {
		// 40006: invalid media size (common for slightly-over-1MB images on legacy endpoint)
		if result.ErrCode == 40006 {
			return fmt.Errorf("%w: %s (code: %d)", ErrImageTooLarge, result.ErrMsg, result.ErrCode)
		}
		if result.ErrCode == 87014 {
			return fmt.Errorf("%w: %s (code: %d)", ErrRiskyContent, result.ErrMsg, result.ErrCode)
		}
		return fmt.Errorf("wechat img sec check error: %s (code: %d)", result.ErrMsg, result.ErrCode)
	}

	return nil
}

func isRetryableImgSecCheckErr(ctx context.Context, err error) bool {
	if err == nil {
		return false
	}
	if ctx != nil {
		if ctx.Err() != nil {
			return false
		}
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return true
		}
		// net.Error.Temporary is deprecated; keep a conservative string fallback.
	}

	msg := err.Error()
	if strings.Contains(msg, "connection reset by peer") {
		return true
	}
	if strings.Contains(msg, "TLS handshake timeout") {
		return true
	}
	if strings.Contains(msg, "i/o timeout") {
		return true
	}
	return false
}

func compressForLegacyImgSecCheck(src []byte, targetBytes int) ([]byte, error) {
	if targetBytes <= 0 {
		return nil, fmt.Errorf("invalid targetBytes")
	}
	img, _, err := image.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	// Try a few JPEG quality levels until we fit under targetBytes.
	qualities := []int{85, 75, 65, 55, 45, 35}
	for _, q := range qualities {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: q}); err != nil {
			return nil, fmt.Errorf("encode jpeg (q=%d): %w", q, err)
		}
		if buf.Len() <= targetBytes {
			return buf.Bytes(), nil
		}
	}

	return nil, fmt.Errorf("cannot compress image to <=%d bytes", targetBytes)
}
