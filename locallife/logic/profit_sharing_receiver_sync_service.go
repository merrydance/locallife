package logic

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
)

var ErrProfitSharingReceiverOpenIDRequired = errors.New("profit sharing receiver wechat openid is required")

type ProfitSharingReceiverSyncService struct {
	store           db.Store
	ecommerceClient wechat.EcommerceClientInterface
}

func NewProfitSharingReceiverService(store db.Store, ecommerceClient wechat.EcommerceClientInterface) *ProfitSharingReceiverSyncService {
	return &ProfitSharingReceiverSyncService{
		store:           store,
		ecommerceClient: ecommerceClient,
	}
}

func (s *ProfitSharingReceiverSyncService) EnsureOperatorReceiver(ctx context.Context, operator db.Operator) error {
	user, err := s.store.GetUser(ctx, operator.UserID)
	if err != nil {
		return fmt.Errorf("get operator user: %w", err)
	}

	return s.EnsurePersonalOpenIDReceiver(ctx, strings.TrimSpace(user.WechatOpenid), operatorReceiverDisplayName(operator))
}

func (s *ProfitSharingReceiverSyncService) DeleteOperatorReceiver(ctx context.Context, operator db.Operator) error {
	user, err := s.store.GetUser(ctx, operator.UserID)
	if err != nil {
		return fmt.Errorf("get operator user: %w", err)
	}

	return s.DeletePersonalOpenIDReceiver(ctx, strings.TrimSpace(user.WechatOpenid))
}

func (s *ProfitSharingReceiverSyncService) EnsureRiderReceiver(ctx context.Context, rider db.Rider) error {
	user, err := s.store.GetUser(ctx, rider.UserID)
	if err != nil {
		return fmt.Errorf("get rider user: %w", err)
	}

	return s.EnsurePersonalOpenIDReceiver(ctx, strings.TrimSpace(user.WechatOpenid), strings.TrimSpace(rider.RealName))
}

func (s *ProfitSharingReceiverSyncService) DeleteRiderReceiver(ctx context.Context, rider db.Rider) error {
	user, err := s.store.GetUser(ctx, rider.UserID)
	if err != nil {
		return fmt.Errorf("get rider user: %w", err)
	}

	return s.DeletePersonalOpenIDReceiver(ctx, strings.TrimSpace(user.WechatOpenid))
}

func (s *ProfitSharingReceiverSyncService) EnsurePersonalOpenIDReceiver(ctx context.Context, openID string, receiverName string) error {
	if s.ecommerceClient == nil {
		return fmt.Errorf("ecommerce client not configured")
	}

	trimmedOpenID := strings.TrimSpace(openID)
	if trimmedOpenID == "" {
		return NewRequestError(http.StatusBadRequest, ErrProfitSharingReceiverOpenIDRequired)
	}

	appID := strings.TrimSpace(s.ecommerceClient.GetSpAppID())
	if appID == "" {
		return fmt.Errorf("ecommerce client sp appid not configured")
	}

	req := &wechatcontracts.AddReceiverRequest{
		AppID:        appID,
		Type:         wechatcontracts.ReceiverTypePersonal,
		Account:      trimmedOpenID,
		RelationType: wechatcontracts.RelationOthers,
	}
	if trimmedName := strings.TrimSpace(receiverName); trimmedName != "" {
		encryptedName, err := s.ecommerceClient.EncryptSensitiveData(trimmedName)
		if err != nil {
			return fmt.Errorf("encrypt receiver name: %w", err)
		}
		req.EncryptedName = encryptedName
	}

	if _, err := s.ecommerceClient.AddProfitSharingReceiver(ctx, req); err != nil {
		if isIgnorableAddReceiverError(err) {
			return nil
		}
		return fmt.Errorf("add profit sharing receiver: %w", err)
	}

	return nil
}

func (s *ProfitSharingReceiverSyncService) DeletePersonalOpenIDReceiver(ctx context.Context, openID string) error {
	if s.ecommerceClient == nil {
		return fmt.Errorf("ecommerce client not configured")
	}

	trimmedOpenID := strings.TrimSpace(openID)
	if trimmedOpenID == "" {
		return NewRequestError(http.StatusBadRequest, ErrProfitSharingReceiverOpenIDRequired)
	}

	appID := strings.TrimSpace(s.ecommerceClient.GetSpAppID())
	if appID == "" {
		return fmt.Errorf("ecommerce client sp appid not configured")
	}

	if _, err := s.ecommerceClient.DeleteProfitSharingReceiver(ctx, &wechatcontracts.DeleteReceiverRequest{
		AppID:   appID,
		Type:    wechatcontracts.ReceiverTypePersonal,
		Account: trimmedOpenID,
	}); err != nil {
		if isIgnorableDeleteReceiverError(err) {
			return nil
		}
		return fmt.Errorf("delete profit sharing receiver: %w", err)
	}

	return nil
}

func operatorReceiverDisplayName(operator db.Operator) string {
	if name := strings.TrimSpace(operator.ContactName); name != "" {
		return name
	}
	return strings.TrimSpace(operator.Name)
}

func isIgnorableAddReceiverError(err error) bool {
	var wxErr *wechat.WechatPayError
	if !errors.As(err, &wxErr) {
		return false
	}

	code := strings.ToUpper(strings.TrimSpace(wxErr.Code))
	message := strings.ToLower(strings.TrimSpace(wxErr.Message + " " + wxErr.Detail))
	if code == "RESOURCE_ALREADY_EXISTS" || code == "RELATION_EXISTS" {
		return true
	}

	return code == "INVALID_REQUEST" && strings.Contains(message, "exist")
}

func isIgnorableDeleteReceiverError(err error) bool {
	var wxErr *wechat.WechatPayError
	if !errors.As(err, &wxErr) {
		return false
	}

	code := strings.ToUpper(strings.TrimSpace(wxErr.Code))
	message := strings.ToLower(strings.TrimSpace(wxErr.Message + " " + wxErr.Detail))
	if code == "RESOURCE_NOT_EXISTS" || code == "NO_RELATION" {
		return true
	}

	if code != "INVALID_REQUEST" {
		return false
	}

	return strings.Contains(message, "not exist") || strings.Contains(message, "no relation")
}
