package db

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

// 创建用于菜品测试的商户
func createRandomMerchantForDish(t *testing.T) Merchant {
	user := createRandomUser(t)
	region := createRandomRegion(t)

	arg := CreateMerchantParams{
		OwnerUserID: user.ID,
		Name:        util.RandomString(10),
		Description: pgtype.Text{String: util.RandomString(20), Valid: true},
		Phone:       util.RandomString(11),
		Address:     util.RandomString(30),
		Status:      "active",
		RegionID:    region.ID,
		Latitude:    numericFromFloat(39.9282),
		Longitude:   numericFromFloat(116.4507),
	}

	merchant, err := testStore.CreateMerchant(context.Background(), arg)
	require.NoError(t, err)

	merchant, err = testStore.UpdateMerchantStatus(context.Background(), UpdateMerchantStatusParams{
		ID:     merchant.ID,
		Status: "active",
	})
	require.NoError(t, err)
	require.NotEmpty(t, merchant)

	return merchant
}

// 创建随机菜品分类
func createRandomDishCategory(t *testing.T) DishCategory {
	name := util.RandomString(8)
	category, err := testStore.CreateDishCategory(context.Background(), name)
	require.NoError(t, err)
	require.NotEmpty(t, category)

	require.Equal(t, name, category.Name)
	require.NotZero(t, category.ID)
	require.NotZero(t, category.CreatedAt)

	return category
}

func createAndLinkRandomDishCategory(t *testing.T, merchantID int64) (DishCategory, MerchantDishCategory) {
	category := createRandomDishCategory(t)
	sortOrder := int16(util.RandomInt(1, 100))

	arg := LinkMerchantDishCategoryParams{
		MerchantID: merchantID,
		CategoryID: category.ID,
		SortOrder:  sortOrder,
	}

	mdc, err := testStore.LinkMerchantDishCategory(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, mdc)
	require.Equal(t, merchantID, mdc.MerchantID)
	require.Equal(t, category.ID, mdc.CategoryID)
	require.Equal(t, sortOrder, mdc.SortOrder)

	return category, mdc
}

// 辅助函数：创建随机菜品
func createRandomDish(t *testing.T, merchantID, categoryID int64) Dish {
	arg := CreateDishParams{
		MerchantID:  merchantID,
		CategoryID:  pgtype.Int8{Int64: categoryID, Valid: true},
		Name:        util.RandomString(10),
		Description: pgtype.Text{String: util.RandomString(50), Valid: true},
		Price:       util.RandomMoney(),
		MemberPrice: pgtype.Int8{Int64: util.RandomMoney(), Valid: true},
		IsAvailable: true,
		IsOnline:    true,
		SortOrder:   int16(util.RandomInt(1, 100)),
	}

	dish, err := testStore.CreateDish(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, dish)

	require.Equal(t, arg.MerchantID, dish.MerchantID)
	require.Equal(t, arg.Name, dish.Name)
	require.Equal(t, arg.Price, dish.Price)
	require.True(t, dish.IsAvailable)
	require.True(t, dish.IsOnline)
	require.NotZero(t, dish.ID)
	require.NotZero(t, dish.CreatedAt)

	return dish
}

// ============================================
// 菜品分类测试
// ============================================

func TestCreateDishCategory(t *testing.T) {
	createRandomDishCategory(t)
}

func TestLinkMerchantDishCategory(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	createAndLinkRandomDishCategory(t, merchant.ID)
}

func TestGetDishCategory(t *testing.T) {
	category1 := createRandomDishCategory(t)

	category2, err := testStore.GetDishCategory(context.Background(), category1.ID)
	require.NoError(t, err)
	require.NotEmpty(t, category2)

	require.Equal(t, category1.ID, category2.ID)
	require.Equal(t, category1.Name, category2.Name)
}

func TestListDishCategories(t *testing.T) {
	merchant := createRandomMerchantForDish(t)

	// 创建多个分类
	for i := 0; i < 5; i++ {
		createAndLinkRandomDishCategory(t, merchant.ID)
	}

	categories, err := testStore.ListDishCategories(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(categories), 5)

	for _, category := range categories {
		require.NotZero(t, category.ID)
		require.NotEmpty(t, category.Name)
	}
}

func TestUpdateMerchantDishCategoryOrder(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category, mdc := createAndLinkRandomDishCategory(t, merchant.ID)

	newSortOrder := int16(util.RandomInt(1, 100))

	arg := UpdateMerchantDishCategoryOrderParams{
		MerchantID: merchant.ID,
		CategoryID: category.ID,
		SortOrder:  newSortOrder,
	}

	updatedMdc, err := testStore.UpdateMerchantDishCategoryOrder(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, updatedMdc)

	require.Equal(t, mdc.MerchantID, updatedMdc.MerchantID)
	require.Equal(t, mdc.CategoryID, updatedMdc.CategoryID)
	require.Equal(t, newSortOrder, updatedMdc.SortOrder)
}

func TestUnlinkMerchantDishCategory(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category, _ := createAndLinkRandomDishCategory(t, merchant.ID)

	arg := UnlinkMerchantDishCategoryParams{
		MerchantID: merchant.ID,
		CategoryID: category.ID,
	}

	err := testStore.UnlinkMerchantDishCategory(context.Background(), arg)
	require.NoError(t, err)

	// 验证已断开关联
	mdc, err := testStore.GetMerchantDishCategory(context.Background(), GetMerchantDishCategoryParams{
		MerchantID: merchant.ID,
		CategoryID: category.ID,
	})
	require.Error(t, err)
	require.Empty(t, mdc)
}

func TestUnlinkUnusedMerchantDishCategoryTxBlocksActiveDishes(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category, _ := createAndLinkRandomDishCategory(t, merchant.ID)
	dish := createRandomDish(t, merchant.ID, category.ID)

	_, err := testStore.UnlinkUnusedMerchantDishCategoryTx(context.Background(), UnlinkUnusedMerchantDishCategoryParams{
		MerchantID: merchant.ID,
		CategoryID: category.ID,
	})
	require.ErrorIs(t, err, ErrMerchantDishCategoryHasActiveDishes)

	mdc, err := testStore.GetMerchantDishCategory(context.Background(), GetMerchantDishCategoryParams{
		MerchantID: merchant.ID,
		CategoryID: category.ID,
	})
	require.NoError(t, err)
	require.Equal(t, category.ID, mdc.CategoryID)

	reloadedDish, err := testStore.GetDish(context.Background(), dish.ID)
	require.NoError(t, err)
	require.True(t, reloadedDish.CategoryID.Valid)
	require.Equal(t, category.ID, reloadedDish.CategoryID.Int64)
}

func TestUnlinkUnusedMerchantDishCategoryTxAllowsEmptyCategory(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category, _ := createAndLinkRandomDishCategory(t, merchant.ID)

	unlinked, err := testStore.UnlinkUnusedMerchantDishCategoryTx(context.Background(), UnlinkUnusedMerchantDishCategoryParams{
		MerchantID: merchant.ID,
		CategoryID: category.ID,
	})
	require.NoError(t, err)
	require.Equal(t, merchant.ID, unlinked.MerchantID)
	require.Equal(t, category.ID, unlinked.CategoryID)

	_, err = testStore.GetMerchantDishCategory(context.Background(), GetMerchantDishCategoryParams{
		MerchantID: merchant.ID,
		CategoryID: category.ID,
	})
	require.ErrorIs(t, err, ErrRecordNotFound)
}

func TestUnlinkUnusedMerchantDishCategoryTxRejectsUnlinkedCategory(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)

	_, err := testStore.UnlinkUnusedMerchantDishCategoryTx(context.Background(), UnlinkUnusedMerchantDishCategoryParams{
		MerchantID: merchant.ID,
		CategoryID: category.ID,
	})
	require.ErrorIs(t, err, ErrMerchantDishCategoryNotLinked)
}

func pauseDishCategoryMigration(t *testing.T, store *SQLStore, oldCategoryID int64) (func(), func()) {
	t.Helper()
	suffix := time.Now().UnixNano()
	functionName := fmt.Sprintf("pause_rename_category_%d", suffix)
	triggerName := fmt.Sprintf("pause_rename_category_trigger_%d", suffix)
	lockClassID := int32(65001)
	lockObjectID := int32(suffix % 1_000_000_000)

	lockConn, err := store.connPool.Acquire(context.Background())
	require.NoError(t, err)
	t.Cleanup(lockConn.Release)

	_, err = lockConn.Exec(context.Background(), "SELECT pg_advisory_lock($1, $2)", lockClassID, lockObjectID)
	require.NoError(t, err)
	lockHeld := true
	t.Cleanup(func() {
		if lockHeld {
			_, _ = lockConn.Exec(context.Background(), "SELECT pg_advisory_unlock($1, $2)", lockClassID, lockObjectID)
		}
	})

	_, err = store.connPool.Exec(context.Background(), fmt.Sprintf(`
CREATE FUNCTION %s() RETURNS trigger AS $$
BEGIN
	IF OLD.category_id = %d AND NEW.category_id IS DISTINCT FROM OLD.category_id THEN
		PERFORM pg_advisory_xact_lock(%d, %d);
	END IF;
	RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER %s
BEFORE UPDATE OF category_id ON dishes
FOR EACH ROW EXECUTE FUNCTION %s();
`, functionName, oldCategoryID, lockClassID, lockObjectID, triggerName, functionName))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = store.connPool.Exec(context.Background(), fmt.Sprintf("DROP TRIGGER IF EXISTS %s ON dishes", triggerName))
		_, _ = store.connPool.Exec(context.Background(), fmt.Sprintf("DROP FUNCTION IF EXISTS %s()", functionName))
	})

	waitForPausedMigration := func() {
		t.Helper()
		require.Eventually(t, func() bool {
			var waitingLocks int64
			err := store.connPool.QueryRow(context.Background(), `
SELECT COUNT(*)
FROM pg_locks
WHERE locktype = 'advisory'
  AND classid::bigint = $1
  AND objid::bigint = $2
  AND granted = false
`, lockClassID, lockObjectID).Scan(&waitingLocks)
			return err == nil && waitingLocks > 0
		}, 2*time.Second, 25*time.Millisecond)
	}

	releaseMigration := func() {
		t.Helper()
		if !lockHeld {
			return
		}

		var unlocked bool
		err = lockConn.QueryRow(context.Background(), "SELECT pg_advisory_unlock($1, $2)", lockClassID, lockObjectID).Scan(&unlocked)
		require.NoError(t, err)
		require.True(t, unlocked)
		lockHeld = false
	}

	return waitForPausedMigration, releaseMigration
}

func TestRenameMerchantDishCategoryTxSerializesOldCategoryCreateWriters(t *testing.T) {
	store := testStore.(*SQLStore)
	merchant := createRandomMerchantForDish(t)
	oldCategory, _ := createAndLinkRandomDishCategory(t, merchant.ID)
	createRandomDish(t, merchant.ID, oldCategory.ID)
	waitForPausedMigration, releaseMigration := pauseDishCategoryMigration(t, store, oldCategory.ID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	renameErrCh := make(chan error, 1)
	go func() {
		_, renameErr := store.RenameMerchantDishCategoryTx(ctx, RenameMerchantDishCategoryTxParams{
			MerchantID:    merchant.ID,
			OldCategoryID: oldCategory.ID,
			NewName:       util.RandomString(12),
			SortOrder:     int16(util.RandomInt(1, 100)),
		})
		renameErrCh <- renameErr
	}()

	waitForPausedMigration()

	createErrCh := make(chan error, 1)
	go func() {
		_, createErr := store.CreateDishTx(ctx, CreateDishTxParams{
			MerchantID:    merchant.ID,
			CategoryID:    pgtype.Int8{Int64: oldCategory.ID, Valid: true},
			Name:          util.RandomString(10),
			Price:         util.RandomMoney(),
			IsAvailable:   true,
			IsOnline:      true,
			SortOrder:     int16(util.RandomInt(1, 100)),
			PrepareTime:   10,
			IsPackaging:   false,
			IngredientIDs: nil,
			TagIDs:        nil,
		})
		createErrCh <- createErr
	}()

	select {
	case createErr := <-createErrCh:
		require.Failf(t, "create dish should wait for rename to finish with the old category link locked", "create error: %v", createErr)
	case <-time.After(500 * time.Millisecond):
	}

	releaseMigration()

	select {
	case renameErr := <-renameErrCh:
		require.NoError(t, renameErr)
	case <-time.After(3 * time.Second):
		require.FailNow(t, "rename did not finish after releasing advisory lock")
	}

	select {
	case createErr := <-createErrCh:
		require.ErrorIs(t, createErr, ErrMerchantDishCategoryNotLinked)
	case <-time.After(3 * time.Second):
		require.FailNow(t, "create did not finish after rename committed")
	}

	var activeOldCategoryDishes int64
	err := store.connPool.QueryRow(context.Background(), `
SELECT COUNT(*)
FROM dishes
WHERE merchant_id = $1 AND category_id = $2 AND deleted_at IS NULL
`, merchant.ID, oldCategory.ID).Scan(&activeOldCategoryDishes)
	require.NoError(t, err)
	require.Zero(t, activeOldCategoryDishes)
}

func TestRenameMerchantDishCategoryTxSerializesOldCategoryUpdateWriters(t *testing.T) {
	store := testStore.(*SQLStore)
	merchant := createRandomMerchantForDish(t)
	oldCategory, _ := createAndLinkRandomDishCategory(t, merchant.ID)
	currentCategory, _ := createAndLinkRandomDishCategory(t, merchant.ID)
	createRandomDish(t, merchant.ID, oldCategory.ID)
	dishToUpdate := createRandomDish(t, merchant.ID, currentCategory.ID)
	waitForPausedMigration, releaseMigration := pauseDishCategoryMigration(t, store, oldCategory.ID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	renameErrCh := make(chan error, 1)
	go func() {
		_, renameErr := store.RenameMerchantDishCategoryTx(ctx, RenameMerchantDishCategoryTxParams{
			MerchantID:    merchant.ID,
			OldCategoryID: oldCategory.ID,
			NewName:       util.RandomString(12),
			SortOrder:     int16(util.RandomInt(1, 100)),
		})
		renameErrCh <- renameErr
	}()

	waitForPausedMigration()

	updateErrCh := make(chan error, 1)
	go func() {
		_, updateErr := store.UpdateDishTx(ctx, UpdateDishTxParams{
			ID:         dishToUpdate.ID,
			CategoryID: pgtype.Int8{Int64: oldCategory.ID, Valid: true},
		})
		updateErrCh <- updateErr
	}()

	select {
	case updateErr := <-updateErrCh:
		require.Failf(t, "update dish should wait for rename to finish with the old category link locked", "update error: %v", updateErr)
	case <-time.After(500 * time.Millisecond):
	}

	releaseMigration()

	select {
	case renameErr := <-renameErrCh:
		require.NoError(t, renameErr)
	case <-time.After(3 * time.Second):
		require.FailNow(t, "rename did not finish after releasing advisory lock")
	}

	select {
	case updateErr := <-updateErrCh:
		require.ErrorIs(t, updateErr, ErrMerchantDishCategoryNotLinked)
	case <-time.After(3 * time.Second):
		require.FailNow(t, "update did not finish after rename committed")
	}

	reloadedDish, err := store.GetDish(context.Background(), dishToUpdate.ID)
	require.NoError(t, err)
	require.True(t, reloadedDish.CategoryID.Valid)
	require.Equal(t, currentCategory.ID, reloadedDish.CategoryID.Int64)
}

func TestRenameMerchantDishCategoryTxReusesLinkedCategory(t *testing.T) {
	store := testStore.(*SQLStore)
	merchant := createRandomMerchantForDish(t)
	oldCategory, _ := createAndLinkRandomDishCategory(t, merchant.ID)
	targetCategory, _ := createAndLinkRandomDishCategory(t, merchant.ID)
	dish := createRandomDish(t, merchant.ID, oldCategory.ID)
	newSortOrder := int16(util.RandomInt(1, 100))

	result, err := store.RenameMerchantDishCategoryTx(context.Background(), RenameMerchantDishCategoryTxParams{
		MerchantID:    merchant.ID,
		OldCategoryID: oldCategory.ID,
		NewName:       targetCategory.Name,
		SortOrder:     newSortOrder,
	})
	require.NoError(t, err)
	require.Equal(t, targetCategory.ID, result.NewCategoryID)
	require.Equal(t, targetCategory.Name, result.NewCategoryName)
	require.Equal(t, newSortOrder, result.SortOrder)

	_, err = store.GetMerchantDishCategory(context.Background(), GetMerchantDishCategoryParams{
		MerchantID: merchant.ID,
		CategoryID: oldCategory.ID,
	})
	require.ErrorIs(t, err, ErrRecordNotFound)

	targetLink, err := store.GetMerchantDishCategory(context.Background(), GetMerchantDishCategoryParams{
		MerchantID: merchant.ID,
		CategoryID: targetCategory.ID,
	})
	require.NoError(t, err)
	require.Equal(t, newSortOrder, targetLink.SortOrder)

	reloadedDish, err := store.GetDish(context.Background(), dish.ID)
	require.NoError(t, err)
	require.True(t, reloadedDish.CategoryID.Valid)
	require.Equal(t, targetCategory.ID, reloadedDish.CategoryID.Int64)
}

// ============================================
// 菜品测试
// ============================================

func TestCreateDish(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	createRandomDish(t, merchant.ID, category.ID)
}

func TestCreateDishTxRollbackOnCustomizationFailure(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)

	beforeCount, err := testStore.CountDishesByMerchant(context.Background(), CountDishesByMerchantParams{
		MerchantID: merchant.ID,
	})
	require.NoError(t, err)

	result, err := testStore.CreateDishTx(context.Background(), CreateDishTxParams{
		MerchantID:  merchant.ID,
		CategoryID:  pgtype.Int8{Int64: category.ID, Valid: true},
		Name:        util.RandomString(10),
		Description: pgtype.Text{String: util.RandomString(20), Valid: true},
		Price:       util.RandomMoney(),
		IsAvailable: true,
		IsOnline:    true,
		SortOrder:   int16(util.RandomInt(1, 100)),
		PrepareTime: 10,
		CustomizationGroups: []CustomizationGroupInput{
			{
				Name:       "辣度",
				IsRequired: true,
				SortOrder:  1,
				Options: []CustomizationOptionInput{
					{
						TagID:      -1,
						ExtraPrice: 100,
						SortOrder:  1,
					},
				},
			},
		},
	})
	require.Error(t, err)

	afterCount, countErr := testStore.CountDishesByMerchant(context.Background(), CountDishesByMerchantParams{
		MerchantID: merchant.ID,
	})
	require.NoError(t, countErr)
	require.Equal(t, beforeCount, afterCount)

	if result.Dish.ID != 0 {
		_, getErr := testStore.GetDish(context.Background(), result.Dish.ID)
		require.Error(t, getErr)
	}
}

func TestSetDishCustomizationsTxResolvesOptionName(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)
	optionName := "规格-" + util.RandomString(8)

	result, err := testStore.SetDishCustomizationsTx(context.Background(), SetDishCustomizationsTxParams{
		DishID: dish.ID,
		Groups: []CustomizationGroupInput{
			{
				Name:       "规格",
				IsRequired: true,
				SortOrder:  0,
				Options: []CustomizationOptionInput{
					{
						Name:       optionName,
						ExtraPrice: 100,
						SortOrder:  0,
					},
				},
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	require.Len(t, result.Groups[0].Options, 1)

	firstOption := result.Groups[0].Options[0]
	require.NotZero(t, firstOption.TagID)
	require.Equal(t, int64(100), firstOption.ExtraPrice)

	tag, err := testStore.GetTag(context.Background(), firstOption.TagID)
	require.NoError(t, err)
	require.Equal(t, optionName, tag.Name)
	require.Equal(t, "customization", tag.Type)
	require.Equal(t, "active", tag.Status)

	secondResult, err := testStore.SetDishCustomizationsTx(context.Background(), SetDishCustomizationsTxParams{
		DishID: dish.ID,
		Groups: []CustomizationGroupInput{
			{
				Name:       "规格",
				IsRequired: true,
				SortOrder:  0,
				Options: []CustomizationOptionInput{
					{
						Name:       optionName,
						ExtraPrice: 200,
						SortOrder:  0,
					},
				},
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, secondResult.Groups, 1)
	require.Len(t, secondResult.Groups[0].Options, 1)
	require.Equal(t, firstOption.TagID, secondResult.Groups[0].Options[0].TagID)
	require.Equal(t, int64(200), secondResult.Groups[0].Options[0].ExtraPrice)
}

func TestSetDishCustomizationsTxRejectsInvalidExplicitTag(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)
	dishTag := createRandomTag(t, "dish")
	previousResult, err := testStore.SetDishCustomizationsTx(context.Background(), SetDishCustomizationsTxParams{
		DishID: dish.ID,
		Groups: []CustomizationGroupInput{
			{
				Name:       "规格",
				IsRequired: true,
				SortOrder:  0,
				Options: []CustomizationOptionInput{
					{
						Name:       "常规",
						ExtraPrice: 0,
						SortOrder:  0,
					},
				},
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, previousResult.Groups, 1)

	_, err = testStore.SetDishCustomizationsTx(context.Background(), SetDishCustomizationsTxParams{
		DishID: dish.ID,
		Groups: []CustomizationGroupInput{
			{
				Name:       "规格",
				IsRequired: true,
				SortOrder:  0,
				Options: []CustomizationOptionInput{
					{
						TagID:      dishTag.ID,
						ExtraPrice: 100,
						SortOrder:  0,
					},
				},
			},
		},
	})
	require.ErrorIs(t, err, ErrCustomizationTagUnavailable)

	groups, listErr := testStore.ListDishCustomizationGroups(context.Background(), dish.ID)
	require.NoError(t, listErr)
	require.Len(t, groups, 1)
	require.Equal(t, previousResult.Groups[0].Group.ID, groups[0].ID)
}

func TestSetDishCustomizationsTxRejectsDuplicateResolvedOption(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)
	optionName := "重复规格-" + util.RandomString(8)

	_, err := testStore.SetDishCustomizationsTx(context.Background(), SetDishCustomizationsTxParams{
		DishID: dish.ID,
		Groups: []CustomizationGroupInput{
			{
				Name:       "规格",
				IsRequired: true,
				SortOrder:  0,
				Options: []CustomizationOptionInput{
					{
						Name:       optionName,
						ExtraPrice: 0,
						SortOrder:  0,
					},
					{
						Name:       " " + optionName + " ",
						ExtraPrice: 100,
						SortOrder:  1,
					},
				},
			},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate customization option")
}

func TestCreateDishTxRejectsUnlinkedCategory(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)

	beforeCount, err := testStore.CountDishesByMerchant(context.Background(), CountDishesByMerchantParams{
		MerchantID: merchant.ID,
	})
	require.NoError(t, err)

	_, err = testStore.CreateDishTx(context.Background(), CreateDishTxParams{
		MerchantID:  merchant.ID,
		CategoryID:  pgtype.Int8{Int64: category.ID, Valid: true},
		Name:        util.RandomString(10),
		Description: pgtype.Text{String: util.RandomString(20), Valid: true},
		Price:       util.RandomMoney(),
		IsAvailable: true,
		IsOnline:    true,
		SortOrder:   int16(util.RandomInt(1, 100)),
		PrepareTime: 10,
	})
	require.ErrorIs(t, err, ErrMerchantDishCategoryNotLinked)

	afterCount, err := testStore.CountDishesByMerchant(context.Background(), CountDishesByMerchantParams{
		MerchantID: merchant.ID,
	})
	require.NoError(t, err)
	require.Equal(t, beforeCount, afterCount)
}

func TestCreateDishTxRejectsUnlinkedSelectableTag(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category, _ := createAndLinkRandomDishCategory(t, merchant.ID)
	tag := createRandomTag(t, TagTypeDish)

	beforeCount, err := testStore.CountDishesByMerchant(context.Background(), CountDishesByMerchantParams{
		MerchantID: merchant.ID,
	})
	require.NoError(t, err)

	_, err = testStore.CreateDishTx(context.Background(), CreateDishTxParams{
		MerchantID:  merchant.ID,
		CategoryID:  pgtype.Int8{Int64: category.ID, Valid: true},
		Name:        util.RandomString(10),
		Description: pgtype.Text{String: util.RandomString(20), Valid: true},
		Price:       util.RandomMoney(),
		IsAvailable: true,
		IsOnline:    true,
		SortOrder:   int16(util.RandomInt(1, 100)),
		PrepareTime: 10,
		TagIDs:      []int64{tag.ID},
	})
	require.ErrorIs(t, err, ErrMerchantSelectableTagUnavailable)

	afterCount, err := testStore.CountDishesByMerchant(context.Background(), CountDishesByMerchantParams{
		MerchantID: merchant.ID,
	})
	require.NoError(t, err)
	require.Equal(t, beforeCount, afterCount)
}

func TestCreateDishTxDeduplicatesSelectableTagIDs(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category, _ := createAndLinkRandomDishCategory(t, merchant.ID)
	tag, err := testStore.CreateMerchantSelectableTagTx(context.Background(), CreateMerchantSelectableTagTxParams{
		MerchantID: merchant.ID,
		Name:       "菜品标签-" + util.RandomString(8),
		Type:       TagTypeDish,
		SortOrder:  1,
	})
	require.NoError(t, err)

	result, err := testStore.CreateDishTx(context.Background(), CreateDishTxParams{
		MerchantID:  merchant.ID,
		CategoryID:  pgtype.Int8{Int64: category.ID, Valid: true},
		Name:        util.RandomString(10),
		Description: pgtype.Text{String: util.RandomString(20), Valid: true},
		Price:       util.RandomMoney(),
		IsAvailable: true,
		IsOnline:    true,
		SortOrder:   int16(util.RandomInt(1, 100)),
		PrepareTime: 10,
		TagIDs:      []int64{tag.ID, tag.ID},
	})
	require.NoError(t, err)
	require.Len(t, result.Tags, 1)
	require.Equal(t, tag.ID, result.Tags[0].TagID)

	tags, err := testStore.ListDishTags(context.Background(), result.Dish.ID)
	require.NoError(t, err)
	require.Len(t, tags, 1)
	require.Equal(t, tag.ID, tags[0].ID)
}

func TestGetDish(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish1 := createRandomDish(t, merchant.ID, category.ID)

	dish2, err := testStore.GetDish(context.Background(), dish1.ID)
	require.NoError(t, err)
	require.NotEmpty(t, dish2)

	require.Equal(t, dish1.ID, dish2.ID)
	require.Equal(t, dish1.Name, dish2.Name)
	require.Equal(t, dish1.Price, dish2.Price)
	require.Equal(t, dish1.MerchantID, dish2.MerchantID)
}

func TestListDishesByMerchant(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)

	// 创建多个菜品
	for i := 0; i < 5; i++ {
		createRandomDish(t, merchant.ID, category.ID)
	}

	arg := ListDishesByMerchantParams{
		MerchantID: merchant.ID,
		Limit:      10,
		Offset:     0,
	}

	dishes, err := testStore.ListDishesByMerchant(context.Background(), arg)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(dishes), 5)

	for _, dish := range dishes {
		require.Equal(t, merchant.ID, dish.MerchantID)
	}
}

func TestListDishesByMerchantUsesIDTieBreaker(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish1 := createRandomDish(t, merchant.ID, category.ID)
	dish2 := createRandomDish(t, merchant.ID, category.ID)
	tiedCreatedAt := time.Now().UTC().Truncate(time.Microsecond)
	sharedSortOrder := int16(7)

	store, ok := testStore.(*SQLStore)
	require.True(t, ok)

	_, err := store.connPool.Exec(context.Background(),
		"UPDATE dishes SET created_at = $1, sort_order = $2 WHERE id = ANY($3)",
		tiedCreatedAt,
		sharedSortOrder,
		[]int64{dish1.ID, dish2.ID},
	)
	require.NoError(t, err)

	rows, err := testStore.ListDishesByMerchant(context.Background(), ListDishesByMerchantParams{
		MerchantID: merchant.ID,
		Limit:      2,
		Offset:     0,
	})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, dish2.ID, rows[0].ID)
	require.Equal(t, dish1.ID, rows[1].ID)
}

func TestListDishesByMerchantWithFilters(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)

	// 创建在线和离线菜品
	createRandomDish(t, merchant.ID, category.ID)

	testCases := []struct {
		name   string
		params ListDishesByMerchantParams
	}{
		{
			name: "FilterByCategory",
			params: ListDishesByMerchantParams{
				MerchantID: merchant.ID,
				CategoryID: pgtype.Int8{Int64: category.ID, Valid: true},
				Limit:      10,
				Offset:     0,
			},
		},
		{
			name: "FilterByOnlineStatus",
			params: ListDishesByMerchantParams{
				MerchantID: merchant.ID,
				IsOnline:   pgtype.Bool{Bool: true, Valid: true},
				Limit:      10,
				Offset:     0,
			},
		},
		{
			name: "FilterByAvailability",
			params: ListDishesByMerchantParams{
				MerchantID:  merchant.ID,
				IsAvailable: pgtype.Bool{Bool: true, Valid: true},
				Limit:       10,
				Offset:      0,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dishes, err := testStore.ListDishesByMerchant(context.Background(), tc.params)
			require.NoError(t, err)
			require.NotEmpty(t, dishes)
		})
	}
}

func TestSearchDishesByName(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	// 搜索菜品名称的前几个字符
	searchTerm := dish.Name[:3]

	arg := SearchDishesByNameParams{
		MerchantID:       merchant.ID,
		NameQuery:        pgtype.Text{String: searchTerm, Valid: true},
		ExcludePackaging: false,
		Limit:            10,
		Offset:           0,
	}

	dishes, err := testStore.SearchDishesByName(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, dishes)

	// 验证找到创建的菜品
	found := false
	for _, d := range dishes {
		if d.ID == dish.ID {
			found = true
			break
		}
	}
	require.True(t, found, "Created dish should be found in search results")
}

func TestSearchDishesByNameExcludesUnavailable(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	uniqueName := "UnavailableByName_" + util.RandomString(10)

	dish, err := testStore.CreateDish(context.Background(), CreateDishParams{
		MerchantID:  merchant.ID,
		CategoryID:  pgtype.Int8{Int64: category.ID, Valid: true},
		Name:        uniqueName,
		Price:       util.RandomMoney(),
		IsAvailable: false,
		IsOnline:    true,
	})
	require.NoError(t, err)

	dishes, err := testStore.SearchDishesByName(context.Background(), SearchDishesByNameParams{
		MerchantID:       merchant.ID,
		NameQuery:        pgtype.Text{String: uniqueName, Valid: true},
		ExcludePackaging: false,
		Limit:            10,
		Offset:           0,
	})
	require.NoError(t, err)
	require.Empty(t, dishes, "不可售菜品不应出现在商户内搜索结果")

	count, err := testStore.CountSearchDishesByName(context.Background(), CountSearchDishesByNameParams{
		MerchantID:       merchant.ID,
		NameQuery:        pgtype.Text{String: uniqueName, Valid: true},
		ExcludePackaging: false,
	})
	require.NoError(t, err)
	require.Zero(t, count)

	allDishes, err := testStore.GetDishesByIDsAll(context.Background(), []int64{dish.ID})
	require.NoError(t, err)
	require.Len(t, allDishes, 1, "测试数据应真实创建成功")
	require.False(t, allDishes[0].IsAvailable)
}

func TestUpdateDish(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	newName := util.RandomString(10)
	newPrice := util.RandomMoney()

	arg := UpdateDishParams{
		ID:    dish.ID,
		Name:  pgtype.Text{String: newName, Valid: true},
		Price: pgtype.Int8{Int64: newPrice, Valid: true},
	}

	updatedDish, err := testStore.UpdateDish(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, updatedDish)

	require.Equal(t, dish.ID, updatedDish.ID)
	require.Equal(t, newName, updatedDish.Name)
	require.Equal(t, newPrice, updatedDish.Price)
}

func TestUpdateDishTxRejectsUnlinkedCategory(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category, _ := createAndLinkRandomDishCategory(t, merchant.ID)
	unlinkedCategory := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	_, err := testStore.UpdateDishTx(context.Background(), UpdateDishTxParams{
		ID:         dish.ID,
		CategoryID: pgtype.Int8{Int64: unlinkedCategory.ID, Valid: true},
	})
	require.ErrorIs(t, err, ErrMerchantDishCategoryNotLinked)

	reloadedDish, err := testStore.GetDish(context.Background(), dish.ID)
	require.NoError(t, err)
	require.True(t, reloadedDish.CategoryID.Valid)
	require.Equal(t, category.ID, reloadedDish.CategoryID.Int64)
}

func TestUpdateDishTxRejectsWrongTypeSelectableTag(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category, _ := createAndLinkRandomDishCategory(t, merchant.ID)
	dish := createRandomDish(t, merchant.ID, category.ID)
	tableTag, err := testStore.CreateMerchantSelectableTagTx(context.Background(), CreateMerchantSelectableTagTxParams{
		MerchantID: merchant.ID,
		Name:       "桌台标签-" + util.RandomString(8),
		Type:       TagTypeTable,
		SortOrder:  1,
	})
	require.NoError(t, err)

	tagIDs := []int64{tableTag.ID}
	_, err = testStore.UpdateDishTx(context.Background(), UpdateDishTxParams{
		ID:     dish.ID,
		TagIDs: &tagIDs,
	})
	require.ErrorIs(t, err, ErrMerchantSelectableTagUnavailable)

	tags, err := testStore.ListDishTags(context.Background(), dish.ID)
	require.NoError(t, err)
	require.Empty(t, tags)
}

func TestUpdateDishTxDeduplicatesSelectableTagIDs(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category, _ := createAndLinkRandomDishCategory(t, merchant.ID)
	dish := createRandomDish(t, merchant.ID, category.ID)
	tag, err := testStore.CreateMerchantSelectableTagTx(context.Background(), CreateMerchantSelectableTagTxParams{
		MerchantID: merchant.ID,
		Name:       "菜品标签-" + util.RandomString(8),
		Type:       TagTypeDish,
		SortOrder:  1,
	})
	require.NoError(t, err)

	tagIDs := []int64{tag.ID, tag.ID}
	result, err := testStore.UpdateDishTx(context.Background(), UpdateDishTxParams{
		ID:     dish.ID,
		TagIDs: &tagIDs,
	})
	require.NoError(t, err)
	require.Len(t, result.Tags, 1)
	require.Equal(t, tag.ID, result.Tags[0].TagID)

	tags, err := testStore.ListDishTags(context.Background(), dish.ID)
	require.NoError(t, err)
	require.Len(t, tags, 1)
	require.Equal(t, tag.ID, tags[0].ID)
}

func TestUpdateDishCanSetAvailability(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	_, err := testStore.UpdateDish(context.Background(), UpdateDishParams{
		ID:          dish.ID,
		IsAvailable: pgtype.Bool{Bool: false, Valid: true},
	})
	require.NoError(t, err)

	// 验证更新
	updatedDish, err := testStore.GetDish(context.Background(), dish.ID)
	require.NoError(t, err)
	require.False(t, updatedDish.IsAvailable)
}

func TestUpdateDishOnlineStatus(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	arg := UpdateDishOnlineStatusParams{
		ID:       dish.ID,
		IsOnline: false,
	}

	err := testStore.UpdateDishOnlineStatus(context.Background(), arg)
	require.NoError(t, err)

	// 验证更新
	updatedDish, err := testStore.GetDish(context.Background(), dish.ID)
	require.NoError(t, err)
	require.False(t, updatedDish.IsOnline)
}

func TestBatchUpdateDishOnlineStatusReturnsActualUpdatedIDs(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	otherMerchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)
	foreignDish := createRandomDish(t, otherMerchant.ID, category.ID)

	updatedIDs, err := testStore.BatchUpdateDishOnlineStatus(context.Background(), BatchUpdateDishOnlineStatusParams{
		IsOnline:   false,
		Column2:    []int64{dish.ID, foreignDish.ID, util.RandomInt(100000, 200000)},
		MerchantID: merchant.ID,
	})
	require.NoError(t, err)
	require.Equal(t, []int64{dish.ID}, updatedIDs)

	updatedDish, err := testStore.GetDish(context.Background(), dish.ID)
	require.NoError(t, err)
	require.False(t, updatedDish.IsOnline)

	unchangedForeignDish, err := testStore.GetDish(context.Background(), foreignDish.ID)
	require.NoError(t, err)
	require.True(t, unchangedForeignDish.IsOnline)
}

func TestDeleteDish(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	err := testStore.DeleteDish(context.Background(), dish.ID)
	require.NoError(t, err)

	// 验证已删除
	dish2, err := testStore.GetDish(context.Background(), dish.ID)
	require.Error(t, err)
	require.Empty(t, dish2)
}

func TestCountDishesByMerchant(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)

	// 创建一些菜品
	for i := 0; i < 3; i++ {
		createRandomDish(t, merchant.ID, category.ID)
	}

	arg := CountDishesByMerchantParams{
		MerchantID: merchant.ID,
	}

	count, err := testStore.CountDishesByMerchant(context.Background(), arg)
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, int64(3))
}

// ============================================
// 菜品-食材关联测试
// ============================================

func TestAddDishIngredient(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)
	ingredient := createRandomIngredient(t, true)

	arg := AddDishIngredientParams{
		DishID:       dish.ID,
		IngredientID: ingredient.ID,
	}

	_, err := testStore.AddDishIngredient(context.Background(), arg)
	require.NoError(t, err)

	// 验证关联
	ingredients, err := testStore.ListDishIngredients(context.Background(), dish.ID)
	require.NoError(t, err)
	require.NotEmpty(t, ingredients)

	found := false
	for _, ing := range ingredients {
		if ing.ID == ingredient.ID {
			found = true
			break
		}
	}
	require.True(t, found)
}

func TestRemoveDishIngredient(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)
	ingredient := createRandomIngredient(t, true)

	// 先添加
	addArg := AddDishIngredientParams{
		DishID:       dish.ID,
		IngredientID: ingredient.ID,
	}
	_, err := testStore.AddDishIngredient(context.Background(), addArg)
	require.NoError(t, err)

	// 再移除
	removeArg := RemoveDishIngredientParams{
		DishID:       dish.ID,
		IngredientID: ingredient.ID,
	}
	err = testStore.RemoveDishIngredient(context.Background(), removeArg)
	require.NoError(t, err)

	// 验证已移除
	ingredients, err := testStore.ListDishIngredients(context.Background(), dish.ID)
	require.NoError(t, err)

	for _, ing := range ingredients {
		require.NotEqual(t, ingredient.ID, ing.ID)
	}
}

// ============================================
// 菜品-标签关联测试
// ============================================

func TestAddDishTag(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	// 创建一个dish类型的tag
	tagArg := CreateTagParams{
		Name: util.RandomString(5),
		Type: "dish",
	}
	tag, err := testStore.CreateTag(context.Background(), tagArg)
	require.NoError(t, err)

	// 添加tag到dish
	arg := AddDishTagParams{
		DishID: dish.ID,
		TagID:  tag.ID,
	}

	_, err = testStore.AddDishTag(context.Background(), arg)
	require.NoError(t, err)

	// 验证关联
	tags, err := testStore.ListDishTags(context.Background(), dish.ID)
	require.NoError(t, err)
	require.NotEmpty(t, tags)

	found := false
	for _, t := range tags {
		if t.ID == tag.ID {
			found = true
			break
		}
	}
	require.True(t, found)
}

func TestRemoveDishTag(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	// 创建并添加tag
	tagArg := CreateTagParams{
		Name: util.RandomString(5),
		Type: "dish",
	}
	tag, err := testStore.CreateTag(context.Background(), tagArg)
	require.NoError(t, err)

	addArg := AddDishTagParams{
		DishID: dish.ID,
		TagID:  tag.ID,
	}
	_, err = testStore.AddDishTag(context.Background(), addArg)
	require.NoError(t, err)

	// 移除tag
	removeArg := RemoveDishTagParams{
		DishID: dish.ID,
		TagID:  tag.ID,
	}
	err = testStore.RemoveDishTag(context.Background(), removeArg)
	require.NoError(t, err)

	// 验证已移除
	tags, err := testStore.ListDishTags(context.Background(), dish.ID)
	require.NoError(t, err)

	for _, tg := range tags {
		require.NotEqual(t, tag.ID, tg.ID)
	}
}

func TestSetDishFeaturedTagsTx(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	recommended, err := testStore.GetSystemTagByName(context.Background(), "推荐")
	require.NoError(t, err)
	hot, err := testStore.GetSystemTagByName(context.Background(), "热卖")
	require.NoError(t, err)
	normal, err := testStore.CreateTag(context.Background(), CreateTagParams{
		Name: "普通-" + util.RandomString(6),
		Type: "dish",
	})
	require.NoError(t, err)

	_, err = testStore.AddDishTag(context.Background(), AddDishTagParams{
		DishID: dish.ID,
		TagID:  recommended.ID,
	})
	require.NoError(t, err)
	_, err = testStore.AddDishTag(context.Background(), AddDishTagParams{
		DishID: dish.ID,
		TagID:  normal.ID,
	})
	require.NoError(t, err)

	result, err := testStore.SetDishFeaturedTagsTx(context.Background(), SetDishFeaturedTagsTxParams{
		DishID: dish.ID,
		Tags:   []string{hot.Name},
	})
	require.NoError(t, err)
	require.Len(t, result.Tags, 2)
	require.ElementsMatch(t, []string{normal.Name, hot.Name}, tagNames(result.Tags))

	tags, err := testStore.ListDishTags(context.Background(), dish.ID)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{normal.Name, hot.Name}, tagNames(tags))
}

func TestSetDishFeaturedTagsTxFiltersAndDedupesInput(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	hot, err := testStore.GetSystemTagByName(context.Background(), "热卖")
	require.NoError(t, err)
	normal, err := testStore.CreateTag(context.Background(), CreateTagParams{
		Name: "普通-" + util.RandomString(6),
		Type: "dish",
	})
	require.NoError(t, err)

	_, err = testStore.AddDishTag(context.Background(), AddDishTagParams{
		DishID: dish.ID,
		TagID:  normal.ID,
	})
	require.NoError(t, err)

	result, err := testStore.SetDishFeaturedTagsTx(context.Background(), SetDishFeaturedTagsTxParams{
		DishID: dish.ID,
		Tags:   []string{"普通标签", hot.Name, hot.Name},
	})
	require.NoError(t, err)
	require.ElementsMatch(t, []string{normal.Name, hot.Name}, tagNames(result.Tags))

	tags, err := testStore.ListDishTags(context.Background(), dish.ID)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{normal.Name, hot.Name}, tagNames(tags))
}

func TestSetDishFeaturedTagsTxIgnoresUnknownNonFeaturedInput(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	recommended, err := testStore.GetSystemTagByName(context.Background(), "推荐")
	require.NoError(t, err)

	_, err = testStore.AddDishTag(context.Background(), AddDishTagParams{
		DishID: dish.ID,
		TagID:  recommended.ID,
	})
	require.NoError(t, err)

	result, err := testStore.SetDishFeaturedTagsTx(context.Background(), SetDishFeaturedTagsTxParams{
		DishID: dish.ID,
		Tags:   []string{"missing-featured-" + util.RandomString(6)},
	})
	require.NoError(t, err)
	require.Empty(t, result.Tags)

	tags, err := testStore.ListDishTags(context.Background(), dish.ID)
	require.NoError(t, err)
	require.Empty(t, tags)
}

func TestSetDishFeaturedTagsTxRollsBackWhenFeaturedSystemTagMissing(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	recommended, err := testStore.GetSystemTagByName(context.Background(), "推荐")
	require.NoError(t, err)

	_, err = testStore.AddDishTag(context.Background(), AddDishTagParams{
		DishID: dish.ID,
		TagID:  recommended.ID,
	})
	require.NoError(t, err)

	store, ok := testStore.(*SQLStore)
	require.True(t, ok)
	_, err = store.connPool.Exec(context.Background(), "UPDATE tags SET type = 'dish' WHERE id = $1", recommended.ID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, restoreErr := store.connPool.Exec(context.Background(), "UPDATE tags SET type = 'system' WHERE id = $1", recommended.ID)
		require.NoError(t, restoreErr)
	})

	_, err = testStore.SetDishFeaturedTagsTx(context.Background(), SetDishFeaturedTagsTxParams{
		DishID: dish.ID,
		Tags:   []string{recommended.Name},
	})
	require.Error(t, err)

	tags, err := testStore.ListDishTags(context.Background(), dish.ID)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{recommended.Name}, tagNames(tags))
}

func tagNames(tags []Tag) []string {
	names := make([]string, 0, len(tags))
	for _, tag := range tags {
		names = append(names, tag.Name)
	}
	return names
}

// ============================================
// 复杂查询测试 - GetDishWithDetails
// ============================================

func TestGetDishWithDetails(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	// 添加食材
	ingredient1 := createRandomIngredient(t, true)
	ingredient2 := createRandomIngredient(t, true)
	_, err := testStore.AddDishIngredient(context.Background(), AddDishIngredientParams{
		DishID:       dish.ID,
		IngredientID: ingredient1.ID,
	})
	require.NoError(t, err)
	_, err = testStore.AddDishIngredient(context.Background(), AddDishIngredientParams{
		DishID:       dish.ID,
		IngredientID: ingredient2.ID,
	})
	require.NoError(t, err)

	// 添加标签
	tag, err := testStore.CreateTag(context.Background(), CreateTagParams{
		Name: util.RandomString(5),
		Type: "dish",
	})
	require.NoError(t, err)
	_, err = testStore.AddDishTag(context.Background(), AddDishTagParams{
		DishID: dish.ID,
		TagID:  tag.ID,
	})
	require.NoError(t, err)

	// 获取完整信息
	dishDetails, err := testStore.GetDishWithDetails(context.Background(), dish.ID)
	require.NoError(t, err)
	require.NotEmpty(t, dishDetails)

	// 验证基本信息
	require.Equal(t, dish.ID, dishDetails.ID)
	require.Equal(t, dish.Name, dishDetails.Name)
	require.Equal(t, category.Name, dishDetails.CategoryName.String)

	// 验证JSON字段不为空（具体解析在API层）
	require.NotEmpty(t, dishDetails.Ingredients)
	require.NotEmpty(t, dishDetails.Tags)
}

// ============================================
// 全局搜索测试 - SearchDishesGlobal
// ============================================

func TestSearchDishesGlobal(t *testing.T) {
	// 创建已激活的商户
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	// 将商户状态改为 active
	_, err := testStore.UpdateMerchantStatus(context.Background(), UpdateMerchantStatusParams{
		ID:     merchant.ID,
		Status: "active",
	})
	require.NoError(t, err)

	category := createRandomDishCategory(t)

	// 使用足够唯一的名称确保搜索结果准确
	uniqueSuffix := util.RandomString(12)
	uniqueName := "XYZGlobalDish_" + uniqueSuffix
	dish, err := testStore.CreateDish(context.Background(), CreateDishParams{
		MerchantID:  merchant.ID,
		CategoryID:  pgtype.Int8{Int64: category.ID, Valid: true},
		Name:        uniqueName,
		Price:       util.RandomMoney(),
		IsAvailable: true,
		IsOnline:    true,
	})
	require.NoError(t, err)

	// 用完整的唯一名称搜索
	dishes, err := testStore.SearchDishesGlobal(context.Background(), SearchDishesGlobalParams{
		UserLat: 39.9282,
		UserLng: 116.4507,
		RegionID: pgtype.Int8{
			Int64: merchant.RegionID,
			Valid: true,
		},
		Keyword:          pgtype.Text{String: uniqueName, Valid: true},
		ExcludePackaging: false,
		TagID:            pgtype.Int8{Valid: false},
		Limit:            10,
		Offset:           0,
	})
	require.NoError(t, err)
	require.NotEmpty(t, dishes)

	// 验证找到创建的菜品
	found := false
	for _, d := range dishes {
		if d.ID == dish.ID {
			found = true
			break
		}
	}
	require.True(t, found, "Created dish should be found in global search results")
}

func TestSearchDishesGlobal_OnlyApprovedMerchants(t *testing.T) {
	// 创建一个用户
	owner := createRandomUser(t)
	region := createRandomRegion(t)

	// 手动创建pending状态的商户
	appData, _ := json.Marshal(map[string]string{"test": "data"})
	merchant, err := testStore.CreateMerchant(context.Background(), CreateMerchantParams{
		OwnerUserID:     owner.ID,
		Name:            "PendingMerchant_" + util.RandomString(6),
		Phone:           fmt.Sprintf("138%08d", util.RandomInt(10000000, 99999999)),
		Address:         "test address " + util.RandomString(10),
		Status:          "pending", // pending 状态
		ApplicationData: appData,
		RegionID:        region.ID,
	})
	require.NoError(t, err)

	category := createRandomDishCategory(t)

	uniqueName := "TestPendingMerchantDish_" + util.RandomString(6)
	_, err = testStore.CreateDish(context.Background(), CreateDishParams{
		MerchantID:  merchant.ID,
		CategoryID:  pgtype.Int8{Int64: category.ID, Valid: true},
		Name:        uniqueName,
		Price:       util.RandomMoney(),
		IsAvailable: true,
		IsOnline:    true,
	})
	require.NoError(t, err)

	// 全局搜索不应找到未批准商户的菜品
	dishes, err := testStore.SearchDishesGlobal(context.Background(), SearchDishesGlobalParams{
		UserLat: 39.9282,
		UserLng: 116.4507,
		RegionID: pgtype.Int8{
			Int64: merchant.RegionID,
			Valid: true,
		},
		Keyword:          pgtype.Text{String: uniqueName, Valid: true},
		ExcludePackaging: false,
		TagID:            pgtype.Int8{Valid: false},
		Limit:            10,
		Offset:           0,
	})
	require.NoError(t, err)
	require.Empty(t, dishes, "Dishes from pending merchant should not appear in global search")
}

func TestSearchDishesGlobal_ExcludesTakeoutSuspendedMerchants(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	uniqueName := "SuspendedDish_" + util.RandomString(8)

	_, err := testStore.CreateDish(context.Background(), CreateDishParams{
		MerchantID:  merchant.ID,
		CategoryID:  pgtype.Int8{Int64: category.ID, Valid: true},
		Name:        uniqueName,
		Price:       util.RandomMoney(),
		IsAvailable: true,
		IsOnline:    true,
	})
	require.NoError(t, err)

	_, err = testStore.CreateMerchantProfile(context.Background(), merchant.ID)
	require.NoError(t, err)

	err = testStore.SuspendMerchantTakeout(context.Background(), SuspendMerchantTakeoutParams{
		MerchantID:           merchant.ID,
		TakeoutSuspendReason: pgtype.Text{String: "claim recovery overdue", Valid: true},
	})
	require.NoError(t, err)

	dishes, err := testStore.SearchDishesGlobal(context.Background(), SearchDishesGlobalParams{
		UserLat: 39.9282,
		UserLng: 116.4507,
		RegionID: pgtype.Int8{
			Int64: merchant.RegionID,
			Valid: true,
		},
		Keyword:          pgtype.Text{String: uniqueName, Valid: true},
		ExcludePackaging: false,
		TagID:            pgtype.Int8{Valid: false},
		Limit:            10,
		Offset:           0,
	})
	require.NoError(t, err)
	require.Empty(t, dishes)

	count, err := testStore.CountSearchDishesGlobal(context.Background(), CountSearchDishesGlobalParams{
		RegionID: pgtype.Int8{
			Int64: merchant.RegionID,
			Valid: true,
		},
		Keyword:          pgtype.Text{String: uniqueName, Valid: true},
		ExcludePackaging: false,
		TagID:            pgtype.Int8{Valid: false},
	})
	require.NoError(t, err)
	require.Zero(t, count)
}

func TestSearchDishesGlobalExcludesUnavailable(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	_, err := testStore.UpdateMerchantStatus(context.Background(), UpdateMerchantStatusParams{
		ID:     merchant.ID,
		Status: "active",
	})
	require.NoError(t, err)

	category := createRandomDishCategory(t)
	uniqueName := "UnavailableGlobal_" + util.RandomString(10)
	dish, err := testStore.CreateDish(context.Background(), CreateDishParams{
		MerchantID:  merchant.ID,
		CategoryID:  pgtype.Int8{Int64: category.ID, Valid: true},
		Name:        uniqueName,
		Price:       util.RandomMoney(),
		IsAvailable: false,
		IsOnline:    true,
	})
	require.NoError(t, err)

	dishes, err := testStore.SearchDishesGlobal(context.Background(), SearchDishesGlobalParams{
		UserLat: 39.9282,
		UserLng: 116.4507,
		RegionID: pgtype.Int8{
			Int64: merchant.RegionID,
			Valid: true,
		},
		Keyword:          pgtype.Text{String: uniqueName, Valid: true},
		ExcludePackaging: false,
		TagID:            pgtype.Int8{Valid: false},
		Limit:            10,
		Offset:           0,
	})
	require.NoError(t, err)
	require.Empty(t, dishes, "不可售菜品不应出现在全局搜索结果")

	count, err := testStore.CountSearchDishesGlobal(context.Background(), CountSearchDishesGlobalParams{
		RegionID: pgtype.Int8{
			Int64: merchant.RegionID,
			Valid: true,
		},
		Keyword:          pgtype.Text{String: uniqueName, Valid: true},
		ExcludePackaging: false,
		TagID:            pgtype.Int8{Valid: false},
	})
	require.NoError(t, err)
	require.Zero(t, count)

	ids, err := testStore.SearchDishIDsGlobal(context.Background(), SearchDishIDsGlobalParams{
		Keyword:          pgtype.Text{String: uniqueName, Valid: true},
		ExcludePackaging: false,
	})
	require.NoError(t, err)
	require.NotContains(t, ids, dish.ID, "不可售菜品不应出现在推荐关键词过滤ID结果")
}

func TestSearchDishesGlobal_Pagination(t *testing.T) {
	// 创建已激活的商户
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	_, err := testStore.UpdateMerchantStatus(context.Background(), UpdateMerchantStatusParams{
		ID:     merchant.ID,
		Status: "active",
	})
	require.NoError(t, err)

	category := createRandomDishCategory(t)

	// 创建多个菜品
	prefix := "TestPagination_" + util.RandomString(4) + "_"
	for i := 0; i < 5; i++ {
		_, err := testStore.CreateDish(context.Background(), CreateDishParams{
			MerchantID:  merchant.ID,
			CategoryID:  pgtype.Int8{Int64: category.ID, Valid: true},
			Name:        prefix + util.RandomString(4),
			Price:       util.RandomMoney(),
			IsAvailable: true,
			IsOnline:    true,
		})
		require.NoError(t, err)
	}

	// 第一页
	page1, err := testStore.SearchDishesGlobal(context.Background(), SearchDishesGlobalParams{
		UserLat: 39.9282,
		UserLng: 116.4507,
		RegionID: pgtype.Int8{
			Int64: merchant.RegionID,
			Valid: true,
		},
		Keyword:          pgtype.Text{String: prefix, Valid: true},
		ExcludePackaging: false,
		TagID:            pgtype.Int8{Valid: false},
		Limit:            2,
		Offset:           0,
	})
	require.NoError(t, err)
	require.Len(t, page1, 2)

	// 第二页
	page2, err := testStore.SearchDishesGlobal(context.Background(), SearchDishesGlobalParams{
		UserLat: 39.9282,
		UserLng: 116.4507,
		RegionID: pgtype.Int8{
			Int64: merchant.RegionID,
			Valid: true,
		},
		Keyword:          pgtype.Text{String: prefix, Valid: true},
		ExcludePackaging: false,
		TagID:            pgtype.Int8{Valid: false},
		Limit:            2,
		Offset:           2,
	})
	require.NoError(t, err)
	require.Len(t, page2, 2)

	// 确保两页不重复
	for _, d1 := range page1 {
		for _, d2 := range page2 {
			require.NotEqual(t, d1.ID, d2.ID)
		}
	}
}

func TestCountSearchDishesGlobal(t *testing.T) {
	// 创建已激活的商户
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	_, err := testStore.UpdateMerchantStatus(context.Background(), UpdateMerchantStatusParams{
		ID:     merchant.ID,
		Status: "active",
	})
	require.NoError(t, err)

	category := createRandomDishCategory(t)

	prefix := "TestCount_" + util.RandomString(6) + "_"

	// 创建3个菜品
	for i := 0; i < 3; i++ {
		_, err := testStore.CreateDish(context.Background(), CreateDishParams{
			MerchantID:  merchant.ID,
			CategoryID:  pgtype.Int8{Int64: category.ID, Valid: true},
			Name:        prefix + util.RandomString(4),
			Price:       util.RandomMoney(),
			IsAvailable: true,
			IsOnline:    true,
		})
		require.NoError(t, err)
	}

	// 计数
	count, err := testStore.CountSearchDishesGlobal(context.Background(), CountSearchDishesGlobalParams{
		RegionID: pgtype.Int8{
			Int64: merchant.RegionID,
			Valid: true,
		},
		Keyword:          pgtype.Text{String: prefix, Valid: true},
		ExcludePackaging: false,
		TagID:            pgtype.Int8{Valid: false},
	})
	require.NoError(t, err)
	require.Equal(t, int64(3), count)
}

func TestCountSearchDishesByName(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)

	prefix := "TestCountByName_" + util.RandomString(6) + "_"

	// 创建4个菜品
	for i := 0; i < 4; i++ {
		_, err := testStore.CreateDish(context.Background(), CreateDishParams{
			MerchantID:  merchant.ID,
			CategoryID:  pgtype.Int8{Int64: category.ID, Valid: true},
			Name:        prefix + util.RandomString(4),
			Price:       util.RandomMoney(),
			IsAvailable: true,
			IsOnline:    true,
		})
		require.NoError(t, err)
	}

	// 计数
	count, err := testStore.CountSearchDishesByName(context.Background(), CountSearchDishesByNameParams{
		MerchantID:       merchant.ID,
		NameQuery:        pgtype.Text{String: prefix, Valid: true},
		ExcludePackaging: false,
	})
	require.NoError(t, err)
	require.Equal(t, int64(4), count)
}

func TestCountSearchDishesByName_EmptyResult(t *testing.T) {
	merchant := createRandomMerchantForDish(t)

	count, err := testStore.CountSearchDishesByName(context.Background(), CountSearchDishesByNameParams{
		MerchantID:       merchant.ID,
		NameQuery:        pgtype.Text{String: "NonExistentDishName12345", Valid: true},
		ExcludePackaging: false,
	})
	require.NoError(t, err)
	require.Equal(t, int64(0), count)
}

// ============================================
// GetDishesWithMerchantByIDs 测试（推荐流用）
// ============================================

func TestGetDishesWithMerchantByIDs(t *testing.T) {
	// 创建商户和菜品
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish1 := createRandomDish(t, merchant.ID, category.ID)
	dish2 := createRandomDish(t, merchant.ID, category.ID)

	// 批量查询
	dishIDs := []int64{dish1.ID, dish2.ID}
	results, err := testStore.GetDishesWithMerchantByIDs(context.Background(), GetDishesWithMerchantByIDsParams{
		DishIds:          dishIDs,
		ExcludePackaging: false,
	})
	require.NoError(t, err)
	require.Len(t, results, 2)

	// 验证返回数据结构
	for _, r := range results {
		require.NotZero(t, r.ID)
		require.NotEmpty(t, r.Name)
		require.NotZero(t, r.Price)
		require.NotZero(t, r.MerchantID)
		require.NotEmpty(t, r.MerchantName)
		require.NotZero(t, r.MerchantRegionID)
		// MonthlySales 新菜品应该是0
		require.GreaterOrEqual(t, r.MonthlySales, int32(0))
	}
}

func TestGetDishesWithMerchantByIDs_WithMemberPrice(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)

	// 创建带会员价的菜品
	memberPrice := int64(1500) // 15元会员价
	arg := CreateDishParams{
		MerchantID:  merchant.ID,
		CategoryID:  pgtype.Int8{Int64: category.ID, Valid: true},
		Name:        "会员价测试菜品_" + util.RandomString(6),
		Price:       2000, // 20元原价
		MemberPrice: pgtype.Int8{Int64: memberPrice, Valid: true},
		IsAvailable: true,
		IsOnline:    true,
	}
	dish, err := testStore.CreateDish(context.Background(), arg)
	require.NoError(t, err)

	// 查询
	results, err := testStore.GetDishesWithMerchantByIDs(context.Background(), GetDishesWithMerchantByIDsParams{
		DishIds:          []int64{dish.ID},
		ExcludePackaging: false,
	})
	require.NoError(t, err)
	require.Len(t, results, 1)

	// 验证会员价
	require.True(t, results[0].MemberPrice.Valid)
	require.Equal(t, memberPrice, results[0].MemberPrice.Int64)
}

func TestGetDishesWithMerchantByIDs_EmptyIDs(t *testing.T) {
	// 空ID列表应该返回空结果
	results, err := testStore.GetDishesWithMerchantByIDs(context.Background(), GetDishesWithMerchantByIDsParams{
		DishIds:          []int64{},
		ExcludePackaging: false,
	})
	require.NoError(t, err)
	require.Empty(t, results)
}

func TestGetDishesWithMerchantByIDs_NonExistentIDs(t *testing.T) {
	// 不存在的ID应该返回空结果
	results, err := testStore.GetDishesWithMerchantByIDs(context.Background(), GetDishesWithMerchantByIDsParams{
		DishIds:          []int64{999999999},
		ExcludePackaging: false,
	})
	require.NoError(t, err)
	require.Empty(t, results)
}

func TestGetDishesWithMerchantByIDs_FilterOffline(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)

	// 创建一个下架菜品
	arg := CreateDishParams{
		MerchantID:  merchant.ID,
		CategoryID:  pgtype.Int8{Int64: category.ID, Valid: true},
		Name:        "下架菜品_" + util.RandomString(6),
		Price:       1000,
		IsAvailable: true,
		IsOnline:    false, // 下架
	}
	offlineDish, err := testStore.CreateDish(context.Background(), arg)
	require.NoError(t, err)

	// 查询应该过滤掉下架菜品
	results, err := testStore.GetDishesWithMerchantByIDs(context.Background(), GetDishesWithMerchantByIDsParams{
		DishIds:          []int64{offlineDish.ID},
		ExcludePackaging: false,
	})
	require.NoError(t, err)
	require.Empty(t, results, "下架菜品不应被返回")
}

func TestGetDishesByIDsFiltersUnavailable(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)

	unavailableDish, err := testStore.CreateDish(context.Background(), CreateDishParams{
		MerchantID:  merchant.ID,
		CategoryID:  pgtype.Int8{Int64: category.ID, Valid: true},
		Name:        "推荐不可售菜品_" + util.RandomString(6),
		Price:       1000,
		IsAvailable: false,
		IsOnline:    true,
	})
	require.NoError(t, err)

	results, err := testStore.GetDishesByIDs(context.Background(), GetDishesByIDsParams{
		DishIds:          []int64{unavailableDish.ID},
		ExcludePackaging: false,
	})
	require.NoError(t, err)
	require.Empty(t, results, "不可售菜品不应被推荐详情批量查询返回")
}

func TestGetDishesWithMerchantByIDsFilterUnavailable(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)

	unavailableDish, err := testStore.CreateDish(context.Background(), CreateDishParams{
		MerchantID:  merchant.ID,
		CategoryID:  pgtype.Int8{Int64: category.ID, Valid: true},
		Name:        "推荐流不可售菜品_" + util.RandomString(6),
		Price:       1000,
		IsAvailable: false,
		IsOnline:    true,
	})
	require.NoError(t, err)

	results, err := testStore.GetDishesWithMerchantByIDs(context.Background(), GetDishesWithMerchantByIDsParams{
		DishIds:          []int64{unavailableDish.ID},
		ExcludePackaging: false,
	})
	require.NoError(t, err)
	require.Empty(t, results, "不可售菜品不应被推荐流返回")
}

func TestListDishesForMenuFiltersUnavailable(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)

	availableDish, err := testStore.CreateDish(context.Background(), CreateDishParams{
		MerchantID:  merchant.ID,
		CategoryID:  pgtype.Int8{Int64: category.ID, Valid: true},
		Name:        "扫码菜单可售菜品_" + util.RandomString(6),
		Price:       1000,
		IsAvailable: true,
		IsOnline:    true,
	})
	require.NoError(t, err)
	unavailableDish, err := testStore.CreateDish(context.Background(), CreateDishParams{
		MerchantID:  merchant.ID,
		CategoryID:  pgtype.Int8{Int64: category.ID, Valid: true},
		Name:        "扫码菜单不可售菜品_" + util.RandomString(6),
		Price:       1000,
		IsAvailable: false,
		IsOnline:    true,
	})
	require.NoError(t, err)

	menuDishes, err := testStore.ListDishesForMenu(context.Background(), ListDishesForMenuParams{
		MerchantID:       merchant.ID,
		ExcludePackaging: false,
	})
	require.NoError(t, err)

	menuIDs := make([]int64, 0, len(menuDishes))
	for _, dish := range menuDishes {
		menuIDs = append(menuIDs, dish.ID)
	}
	require.Contains(t, menuIDs, availableDish.ID)
	require.NotContains(t, menuIDs, unavailableDish.ID, "不可售菜品不应出现在扫码菜单")
}
