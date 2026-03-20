package db

import (
	"context"
)

type ListGroupApplicationsAdminParams struct {
	Status string
	Limit  int32
	Offset int32
}

func (store *SQLStore) ListGroupApplicationsAdmin(ctx context.Context, arg ListGroupApplicationsAdminParams) ([]MerchantGroupApplication, error) {
	const listGroupApplicationsAdmin = `
SELECT id, applicant_user_id, group_name, contact_phone, license_number, license_media_asset_id, address, region_id, status, reject_reason, reviewed_by, reviewed_at, application_data, created_at, updated_at
FROM merchant_group_applications
WHERE (NULLIF($1::text, '') IS NULL OR status = $1)
ORDER BY created_at DESC
LIMIT $2 OFFSET $3`

	rows, err := store.connPool.Query(ctx, listGroupApplicationsAdmin, arg.Status, arg.Limit, arg.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]MerchantGroupApplication, 0)
	for rows.Next() {
		var item MerchantGroupApplication
		if err := rows.Scan(
			&item.ID,
			&item.ApplicantUserID,
			&item.GroupName,
			&item.ContactPhone,
			&item.LicenseNumber,
			&item.LicenseMediaAssetID,
			&item.Address,
			&item.RegionID,
			&item.Status,
			&item.RejectReason,
			&item.ReviewedBy,
			&item.ReviewedAt,
			&item.ApplicationData,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (store *SQLStore) CountGroupApplicationsAdmin(ctx context.Context, status string) (int64, error) {
	const countGroupApplicationsAdmin = `
SELECT COUNT(*)
FROM merchant_group_applications
WHERE (NULLIF($1::text, '') IS NULL OR status = $1)`

	var count int64
	err := store.connPool.QueryRow(ctx, countGroupApplicationsAdmin, status).Scan(&count)
	return count, err
}
