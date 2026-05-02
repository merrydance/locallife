package ordinaryserviceprovider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
)

const ordinaryMerchantMediaUploadPath = "/v3/merchant/media/upload"

func (c *Client) UploadImage(ctx context.Context, filename string, fileData []byte) (*contracts.MediaUploadResponse, error) {
	if c == nil || c.sdk == nil {
		return nil, &ProviderError{
			Operation: "upload ordinary service provider media image",
			Category:  ErrorCategoryAuthConfig,
			Frontend:  frontendGuidanceForCategory(ErrorCategoryAuthConfig),
			cause:     fmt.Errorf("ordinary service provider sdk client is not configured"),
		}
	}
	normalizedFilename, contentType, err := validateOrdinaryMediaUploadImage(filename, fileData)
	if err != nil {
		return nil, validationProviderError("upload ordinary service provider media image", err)
	}
	meta := map[string]string{
		"filename": normalizedFilename,
		"sha256":   fmt.Sprintf("%x", sha256.Sum256(fileData)),
	}
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return nil, localProviderError("upload ordinary service provider media image", "LOCAL_REQUEST_ENCODE_ERROR", err)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writeOrdinaryUploadField(writer, "meta", "", "application/json", metaBytes); err != nil {
		return nil, localProviderError("upload ordinary service provider media image", "LOCAL_MULTIPART_ERROR", err)
	}
	if err := writeOrdinaryUploadField(writer, "file", normalizedFilename, contentType, fileData); err != nil {
		return nil, localProviderError("upload ordinary service provider media image", "LOCAL_MULTIPART_ERROR", err)
	}
	if err := writer.Close(); err != nil {
		return nil, localProviderError("upload ordinary service provider media image", "LOCAL_MULTIPART_ERROR", err)
	}

	result, err := c.sdk.Upload(ctx, strings.TrimRight(c.config.BaseURL, "/")+ordinaryMerchantMediaUploadPath, string(metaBytes), body.String(), writer.FormDataContentType())
	if err != nil {
		return nil, mapSDKAPIError("upload ordinary service provider media image", err)
	}
	if result == nil || result.Response == nil || result.Response.Body == nil {
		return nil, localProviderError("upload ordinary service provider media image", "LOCAL_EMPTY_RESPONSE", fmt.Errorf("empty upload response"))
	}
	defer result.Response.Body.Close()
	respBody, err := io.ReadAll(result.Response.Body)
	if err != nil {
		return nil, localProviderError("upload ordinary service provider media image", "LOCAL_RESPONSE_READ_ERROR", err)
	}
	var response contracts.MediaUploadResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, localProviderError("upload ordinary service provider media image", "LOCAL_RESPONSE_DECODE_ERROR", err)
	}
	if strings.TrimSpace(response.MediaID) == "" {
		return nil, localProviderError("upload ordinary service provider media image", "LOCAL_EMPTY_MEDIA_ID", fmt.Errorf("wechat upload returned empty media_id"))
	}
	return &response, nil
}

func writeOrdinaryUploadField(writer *multipart.Writer, fieldName, filename, contentType string, data []byte) error {
	var part io.Writer
	var err error
	if filename == "" {
		part, err = writer.CreateFormField(fieldName)
	} else {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fieldName, filename))
		if strings.TrimSpace(contentType) != "" {
			header.Set("Content-Type", contentType)
		}
		part, err = writer.CreatePart(header)
	}
	if err != nil {
		return err
	}
	_, err = part.Write(data)
	return err
}

func validateOrdinaryMediaUploadImage(filename string, fileData []byte) (string, string, error) {
	normalizedFilename := strings.TrimSpace(filepath.Base(filename))
	if normalizedFilename == "" {
		return "", "", fmt.Errorf("filename is required and must end with .jpg, .jpeg, .png, or .bmp")
	}
	if len(fileData) == 0 {
		return "", "", fmt.Errorf("file is empty; provide a non-empty JPG, JPEG, PNG, or BMP image")
	}
	if len(fileData) > 2<<20 {
		return "", "", fmt.Errorf("file size %d exceeds the 2MB WeChat merchant media upload limit; compress the image and retry", len(fileData))
	}
	contentType := http.DetectContentType(fileData)
	switch contentType {
	case "image/jpeg":
		if !hasAnySuffix(normalizedFilename, ".jpg", ".jpeg") {
			normalizedFilename = strings.TrimSuffix(normalizedFilename, filepath.Ext(normalizedFilename)) + ".jpg"
		}
	case "image/png":
		if !strings.HasSuffix(strings.ToLower(normalizedFilename), ".png") {
			return "", "", fmt.Errorf("file content does not match filename; provide a PNG file")
		}
	case "image/bmp", "image/x-ms-bmp":
		if !strings.HasSuffix(strings.ToLower(normalizedFilename), ".bmp") {
			return "", "", fmt.Errorf("file content does not match filename; provide a BMP file")
		}
	default:
		return "", "", fmt.Errorf("file content is not a supported JPG, JPEG, PNG, or BMP image")
	}
	return normalizedFilename, contentType, nil
}

func hasAnySuffix(value string, suffixes ...string) bool {
	lower := strings.ToLower(value)
	for _, suffix := range suffixes {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

func (c *Client) ListPersonalBankingBanks(ctx context.Context, offset, limit int) (*contracts.CapitalBankListResponse, error) {
	return c.listBankingBanks(ctx, "list ordinary service provider personal banking banks", "/v3/capital/capitallhh/banks/personal-banking", offset, limit)
}

func (c *Client) ListCorporateBankingBanks(ctx context.Context, offset, limit int) (*contracts.CapitalBankListResponse, error) {
	return c.listBankingBanks(ctx, "list ordinary service provider corporate banking banks", "/v3/capital/capitallhh/banks/corporate-banking", offset, limit)
}

func (c *Client) listBankingBanks(ctx context.Context, operation, path string, offset, limit int) (*contracts.CapitalBankListResponse, error) {
	query := url.Values{}
	query.Set("offset", fmt.Sprintf("%d", offset))
	query.Set("limit", fmt.Sprintf("%d", limit))
	response := &contracts.CapitalBankListResponse{}
	if err := c.requestJSON(ctx, operation, http.MethodGet, path, query, nil, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) SearchBanksByBankAccount(ctx context.Context, accountNumber string) (*contracts.CapitalBankAccountSearchResponse, error) {
	encryptedAccountNumber, err := c.EncryptSensitiveData(accountNumber)
	if err != nil {
		return nil, localProviderError("search ordinary service provider bank by account", "LOCAL_ENCRYPT_ERROR", err)
	}
	query := url.Values{}
	query.Set("account_number", encryptedAccountNumber)
	response := &contracts.CapitalBankAccountSearchResponse{}
	if err := c.requestJSON(ctx, "search ordinary service provider bank by account", http.MethodGet, "/v3/capital/capitallhh/banks/search-banks-by-bank-account", query, nil, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) ListProvinceAreas(ctx context.Context) (*contracts.CapitalProvinceListResponse, error) {
	response := &contracts.CapitalProvinceListResponse{}
	if err := c.requestJSON(ctx, "list ordinary service provider province areas", http.MethodGet, "/v3/capital/capitallhh/areas/provinces", nil, nil, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) ListCityAreas(ctx context.Context, provinceCode int) (*contracts.CapitalCityListResponse, error) {
	if provinceCode <= 0 {
		return nil, requiredProviderFieldError("list ordinary service provider city areas", "province_code")
	}
	response := &contracts.CapitalCityListResponse{}
	path := fmt.Sprintf("/v3/capital/capitallhh/areas/provinces/%d/cities", provinceCode)
	if err := c.requestJSON(ctx, "list ordinary service provider city areas", http.MethodGet, path, nil, nil, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) ListBankBranches(ctx context.Context, bankAliasCode string, cityCode, offset, limit int) (*contracts.CapitalBranchListResponse, error) {
	if strings.TrimSpace(bankAliasCode) == "" {
		return nil, requiredProviderFieldError("list ordinary service provider bank branches", "bank_alias_code")
	}
	query := url.Values{}
	query.Set("city_code", fmt.Sprintf("%d", cityCode))
	query.Set("offset", fmt.Sprintf("%d", offset))
	query.Set("limit", fmt.Sprintf("%d", limit))
	response := &contracts.CapitalBranchListResponse{}
	path := "/v3/capital/capitallhh/banks/" + url.PathEscape(bankAliasCode) + "/branches"
	if err := c.requestJSON(ctx, "list ordinary service provider bank branches", http.MethodGet, path, query, nil, response); err != nil {
		return nil, err
	}
	return response, nil
}
