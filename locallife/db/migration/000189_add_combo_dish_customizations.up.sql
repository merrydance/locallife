ALTER TABLE "combo_dishes"
ADD COLUMN "customizations" jsonb,
ADD COLUMN "customization_extra_price" bigint NOT NULL DEFAULT 0;