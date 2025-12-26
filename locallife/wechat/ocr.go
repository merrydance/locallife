package wechat

import (
	"bytes"
	"context"
	"errors"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"path/filepath"
	"strings"
	"time"
)

const (
	// 微信OCR接口
	businessLicenseOCRURL = "https://api.weixin.qq.com/cv/ocr/bizlicense?access_token=%s"
	idCardOCRURL          = "https://api.weixin.qq.com/cv/ocr/idcard?access_token=%s"
	printedTextOCRURL     = "https://api.weixin.qq.com/cv/ocr/comm?access_token=%s" // 通用印刷体识别
)

// BusinessLicenseOCRResponse 营业执照OCR识别结果
type BusinessLicenseOCRResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
	// 营业执照注册号
	RegNum string `json:"reg_num"`
	// 企业名称
	EnterpriseEName string `json:"enterprise_name"`
	// 法定代表人
	LegalRepresentative string `json:"legal_representative"`
	// 类型
	TypeOfEnterprise string `json:"type_of_enterprise"`
	// 地址
	Address string `json:"address"`
	// 经营范围
	BusinessScope string `json:"business_scope"`
	// 注册资本
	RegisteredCapital string `json:"registered_capital"`
	// 成立日期
	PaidInCapital string `json:"paid_in_capital"`
	// 营业期限
	ValidPeriod string `json:"valid_period"`
	// 登记机关
	RegisteredAuthority string `json:"registered_authority"`
	// 核准日期
	ApprovalDate string `json:"approval_date"`
	// 统一社会信用代码
	CreditCode string `json:"credit_code"`
}

// IDCardOCRResponse 身份证OCR识别结果
type IDCardOCRResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
	Type    string `json:"type"` // Front or Back
	// 正面信息
	Name   string `json:"name"`
	ID     string `json:"id"`
	Addr   string `json:"addr"`
	Gender string `json:"gender"`
	Nation string `json:"nation"`
	// 背面信息
	ValidDate string `json:"valid_date"`
}

// OCRBusinessLicense 识别营业执照
// imgFile: 图片文件（multipart/form-data 方式上传到微信OCR）
func (c *Client) OCRBusinessLicense(ctx context.Context, imgFile multipart.File) (*BusinessLicenseOCRResponse, error) {
	// 获取access_token（使用mp类型）
	token, err := c.GetAccessToken(ctx, "mp")
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	url := fmt.Sprintf(businessLicenseOCRURL, token)

	imgData, err := readAllLimited(imgFile, MaxOCRImageBytes)
	if err != nil {
		if errors.Is(err, ErrImageTooLarge) {
			return nil, fmt.Errorf("%w: ocr_business_license requires <= %d bytes", ErrImageTooLarge, MaxOCRImageBytes)
		}
		return nil, fmt.Errorf("failed to read image file: %w", err)
	}

	contentType := http.DetectContentType(imgData)
	filename := "license"
	switch contentType {
	case "image/jpeg":
		filename += ".jpg"
	case "image/png":
		filename += ".png"
	default:
		contentType = "application/octet-stream"
	}
	filename = filepath.Base(filename)

	// 构造 multipart/form-data 请求体（微信支持 img 或 img_url 二选一）
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="img"; filename="%s"`, filename))
	partHeader.Set("Content-Type", contentType)
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := part.Write(imgData); err != nil {
		return nil, fmt.Errorf("failed to write image data: %w", err)
	}
	_ = writer.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// 设置超时
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// 解析响应
	var result BusinessLicenseOCRResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// 检查错误
	if result.ErrCode != 0 {
		return nil, fmt.Errorf("wechat ocr error: %s (code: %d)", result.ErrMsg, result.ErrCode)
	}

	return &result, nil
}

// OCRIDCard 识别身份证
// imgFile: 图片文件
// cardSide: Front(正面) 或 Back(背面)
func (c *Client) OCRIDCard(ctx context.Context, imgFile multipart.File, cardSide string) (*IDCardOCRResponse, error) {
	// 获取access_token（使用mp类型）
	token, err := c.GetAccessToken(ctx, "mp")
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	// 构造请求URL
	url := fmt.Sprintf(idCardOCRURL, token) + "&type=" + cardSide

	// 读取图片数据
	imgData, err := readAllLimited(imgFile, MaxOCRImageBytes)
	if err != nil {
		if errors.Is(err, ErrImageTooLarge) {
			return nil, fmt.Errorf("%w: ocr_idcard requires <= %d bytes", ErrImageTooLarge, MaxOCRImageBytes)
		}
		return nil, fmt.Errorf("failed to read image file: %w", err)
	}

	contentType := http.DetectContentType(imgData)
	filename := "idcard"
	switch contentType {
	case "image/jpeg":
		filename += ".jpg"
	case "image/png":
		filename += ".png"
	default:
		contentType = "application/octet-stream"
	}
	filename = filepath.Base(filename)

	// 构造 multipart/form-data 请求
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="img"; filename="%s"`, filename))
	partHeader.Set("Content-Type", contentType)
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := part.Write(imgData); err != nil {
		return nil, fmt.Errorf("failed to write image data: %w", err)
	}
	_ = writer.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// 设置超时
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// 解析响应
	var result IDCardOCRResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// 检查错误
	if result.ErrCode != 0 {
		return nil, fmt.Errorf("wechat ocr error: %s (code: %d)", result.ErrMsg, result.ErrCode)
	}

	return &result, nil
}

// PrintedTextPosition 文字位置信息
type PrintedTextPosition struct {
	LeftTop struct {
		X int `json:"x"`
		Y int `json:"y"`
	} `json:"left_top"`
	RightTop struct {
		X int `json:"x"`
		Y int `json:"y"`
	} `json:"right_top"`
	RightBottom struct {
		X int `json:"x"`
		Y int `json:"y"`
	} `json:"right_bottom"`
	LeftBottom struct {
		X int `json:"x"`
		Y int `json:"y"`
	} `json:"left_bottom"`
}

// PrintedTextItem 识别出的文字项
type PrintedTextItem struct {
	Text string              `json:"text"` // 识别出的文字
	Pos  PrintedTextPosition `json:"pos"`  // 文字位置
}

// PrintedTextOCRResponse 通用印刷体OCR识别结果
type PrintedTextOCRResponse struct {
	ErrCode int               `json:"errcode"`
	ErrMsg  string            `json:"errmsg"`
	Items   []PrintedTextItem `json:"items"` // 识别结果列表
	ImgSize struct {
		W int `json:"w"` // 图片宽度
		H int `json:"h"` // 图片高度
	} `json:"img_size"`
}

// GetAllText 获取所有识别出的文字（按位置排序，从上到下、从左到右）
func (r *PrintedTextOCRResponse) GetAllText() string {
	if r == nil || len(r.Items) == 0 {
		return ""
	}
	var texts []string
	for _, item := range r.Items {
		texts = append(texts, item.Text)
	}
	return strings.Join(texts, " ")
}

// OCRPrintedText 通用印刷体识别（用于食品经营许可证等）
// imgFile: 图片文件
func (c *Client) OCRPrintedText(ctx context.Context, imgFile multipart.File) (*PrintedTextOCRResponse, error) {
	// 获取access_token（使用mp类型）
	token, err := c.GetAccessToken(ctx, "mp")
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	// 构造请求URL
	url := fmt.Sprintf(printedTextOCRURL, token)

	// 读取图片数据
	imgData, err := readAllLimited(imgFile, MaxOCRImageBytes)
	if err != nil {
		if errors.Is(err, ErrImageTooLarge) {
			return nil, fmt.Errorf("%w: ocr_printed_text requires <= %d bytes", ErrImageTooLarge, MaxOCRImageBytes)
		}
		return nil, fmt.Errorf("failed to read image file: %w", err)
	}

	contentType := http.DetectContentType(imgData)
	filename := "image"
	switch contentType {
	case "image/jpeg":
		filename += ".jpg"
	case "image/png":
		filename += ".png"
	default:
		contentType = "application/octet-stream"
	}
	filename = filepath.Base(filename)

	// 构造multipart form请求
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="img"; filename="%s"`, filename))
	partHeader.Set("Content-Type", contentType)
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := part.Write(imgData); err != nil {
		return nil, fmt.Errorf("failed to write image data: %w", err)
	}
	_ = writer.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// 设置超时
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// 解析响应
	var result PrintedTextOCRResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// 检查错误
	if result.ErrCode != 0 {
		return nil, fmt.Errorf("wechat ocr error: %s (code: %d)", result.ErrMsg, result.ErrCode)
	}

	return &result, nil
}
