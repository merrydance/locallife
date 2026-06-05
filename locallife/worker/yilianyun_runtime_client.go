package worker

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/cloudprint"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/util"
)

type yilianyunAuthorizationStore interface {
	GetActiveCloudPrinterProviderAuthorizationByPrinter(context.Context, db.GetActiveCloudPrinterProviderAuthorizationByPrinterParams) (db.CloudPrinterProviderAuthorization, error)
}

type yilianyunRuntimeClient struct {
	store     yilianyunAuthorizationStore
	encryptor util.DataEncryptor
	client    *cloudprint.YilianyunClient
}

func newYilianyunRuntimeClient(config util.Config, store yilianyunAuthorizationStore, encryptor util.DataEncryptor) cloudprint.Client {
	if store == nil {
		return nil
	}
	client := cloudprint.NewYilianyunClientFromConfig(config)
	if client == nil {
		return nil
	}
	return &yilianyunRuntimeClient{
		store:     store,
		encryptor: encryptor,
		client:    client,
	}
}

func (c *yilianyunRuntimeClient) AddPrinter(ctx context.Context, input cloudprint.AddPrinterInput) error {
	return fmt.Errorf("%w: yilianyun uses open-app authorization for printer binding", cloudprint.ErrUnsupportedCapability)
}

func (c *yilianyunRuntimeClient) RemovePrinter(ctx context.Context, input cloudprint.RemovePrinterInput) error {
	return fmt.Errorf("%w: yilianyun authorization revocation requires provider token lifecycle support", cloudprint.ErrUnsupportedCapability)
}

func (c *yilianyunRuntimeClient) Print(ctx context.Context, input cloudprint.PrintInput) (string, error) {
	authorization, err := c.loadAuthorization(ctx, input.PrinterID, input.SN)
	if err != nil {
		return "", err
	}
	accessToken, err := c.decryptAccessToken(authorization)
	if err != nil {
		return "", err
	}
	result, err := c.client.Print(ctx, cloudprint.YilianyunPrintInput{
		MachineCode:      strings.TrimSpace(input.SN),
		AccessToken:      accessToken,
		Content:          input.Content,
		ProviderOriginID: input.ProviderOriginID,
	})
	if err != nil {
		return "", err
	}
	return result.ProviderOrderID, nil
}

func (c *yilianyunRuntimeClient) PrintResultCallbackEnabled() bool {
	return c != nil && c.client != nil && c.client.PrintResultCallbackEnabled()
}

func (c *yilianyunRuntimeClient) QueryOrderState(ctx context.Context, orderID string) (bool, error) {
	return false, fmt.Errorf("%w: yilianyun order state query requires printer-scoped authorization", cloudprint.ErrUnsupportedCapability)
}

func (c *yilianyunRuntimeClient) QueryPrinterStatus(ctx context.Context, sn string) (string, error) {
	return "", fmt.Errorf("%w: yilianyun printer status query requires printer id", cloudprint.ErrUnsupportedCapability)
}

func (c *yilianyunRuntimeClient) GetPrinterInfo(ctx context.Context, sn string) (cloudprint.PrinterInfo, error) {
	return cloudprint.PrinterInfo{}, fmt.Errorf("%w: yilianyun printer info query requires printer id", cloudprint.ErrUnsupportedCapability)
}

func (c *yilianyunRuntimeClient) loadAuthorization(ctx context.Context, printerID int64, machineCode string) (db.CloudPrinterProviderAuthorization, error) {
	if c == nil || c.store == nil || c.client == nil {
		return db.CloudPrinterProviderAuthorization{}, errors.New("yilianyun runtime client is not configured")
	}
	if printerID <= 0 {
		return db.CloudPrinterProviderAuthorization{}, errors.New("yilianyun authorization requires printer id")
	}
	machineCode = strings.TrimSpace(machineCode)
	if machineCode == "" {
		return db.CloudPrinterProviderAuthorization{}, errors.New("yilianyun authorization requires machine code")
	}
	authorization, err := c.store.GetActiveCloudPrinterProviderAuthorizationByPrinter(ctx, db.GetActiveCloudPrinterProviderAuthorizationByPrinterParams{
		AuthorizedCloudPrinterID: pgtype.Int8{Int64: printerID, Valid: true},
		ProviderType:             db.CloudPrinterProviderYilianyun,
		MachineCode:              machineCode,
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return db.CloudPrinterProviderAuthorization{}, errors.New("yilianyun active authorization is missing")
		}
		return db.CloudPrinterProviderAuthorization{}, fmt.Errorf("load yilianyun printer authorization: %w", err)
	}
	return authorization, nil
}

func (c *yilianyunRuntimeClient) decryptAccessToken(authorization db.CloudPrinterProviderAuthorization) (string, error) {
	accessToken, err := util.DecryptSensitiveField(c.encryptor, authorization.AccessTokenCiphertext)
	if err != nil {
		return "", errors.New("decrypt yilianyun access token failed")
	}
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return "", errors.New("yilianyun access token is missing")
	}
	return accessToken, nil
}
