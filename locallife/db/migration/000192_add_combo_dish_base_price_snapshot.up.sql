ALTER TABLE "combo_dishes"
ADD COLUMN "dish_base_price_snapshot" bigint;

UPDATE "combo_dishes" cd
SET "dish_base_price_snapshot" = d.price
FROM "dishes" d
WHERE d.id = cd.dish_id;

ALTER TABLE "combo_dishes"
ALTER COLUMN "dish_base_price_snapshot" SET NOT NULL;