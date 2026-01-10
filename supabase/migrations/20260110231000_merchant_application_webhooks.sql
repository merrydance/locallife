-- 触发器函数：当证照图片更新时调用 OCR Edge Function
CREATE OR REPLACE FUNCTION public.tr_on_merchant_application_image_update()
RETURNS TRIGGER AS $$
DECLARE
  ocr_type text;
  ocr_side text := NULL;
  img_url text;
BEGIN
  -- 营业执照
  IF NEW.business_license_image_url IS DISTINCT FROM OLD.business_license_image_url AND NEW.business_license_image_url IS NOT NULL THEN
    PERFORM net.http_post(
      url := current_setting('app.settings.edge_function_url') || '/ocr-service',
      headers := jsonb_build_object('Content-Type', 'application/json', 'Authorization', 'Bearer ' || current_setting('app.settings.service_role_key')),
      body := jsonb_build_object(
        'application_id', NEW.id,
        'image_url', NEW.business_license_image_url,
        'type', 'business_license'
      )
    );
  END IF;

  -- 食品经营许可证
  IF NEW.food_permit_url IS DISTINCT FROM OLD.food_permit_url AND NEW.food_permit_url IS NOT NULL THEN
    PERFORM net.http_post(
      url := current_setting('app.settings.edge_function_url') || '/ocr-service',
      headers := jsonb_build_object('Content-Type', 'application/json', 'Authorization', 'Bearer ' || current_setting('app.settings.service_role_key')),
      body := jsonb_build_object(
        'application_id', NEW.id,
        'image_url', NEW.food_permit_url,
        'type', 'food_permit'
      )
    );
  END IF;

  -- 身份证正面
  IF NEW.legal_person_id_front_url IS DISTINCT FROM OLD.legal_person_id_front_url AND NEW.legal_person_id_front_url IS NOT NULL THEN
    PERFORM net.http_post(
      url := current_setting('app.settings.edge_function_url') || '/ocr-service',
      headers := jsonb_build_object('Content-Type', 'application/json', 'Authorization', 'Bearer ' || current_setting('app.settings.service_role_key')),
      body := jsonb_build_object(
        'application_id', NEW.id,
        'image_url', NEW.legal_person_id_front_url,
        'type', 'id_card',
        'side', 'Front'
      )
    );
  END IF;

  -- 身份证背面
  IF NEW.legal_person_id_back_url IS DISTINCT FROM OLD.legal_person_id_back_url AND NEW.legal_person_id_back_url IS NOT NULL THEN
    PERFORM net.http_post(
      url := current_setting('app.settings.edge_function_url') || '/ocr-service',
      headers := jsonb_build_object('Content-Type', 'application/json', 'Authorization', 'Bearer ' || current_setting('app.settings.service_role_key')),
      body := jsonb_build_object(
        'application_id', NEW.id,
        'image_url', NEW.legal_person_id_back_url,
        'type', 'id_card',
        'side', 'Back'
      )
    );
  END IF;

  RETURN NEW;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 绑定触发器到 merchant_applications 表
DROP TRIGGER IF EXISTS tr_merchant_application_ocr ON public.merchant_applications;
CREATE TRIGGER tr_merchant_application_ocr
AFTER UPDATE ON public.merchant_applications
FOR EACH ROW
WHEN (
    NEW.business_license_image_url IS DISTINCT FROM OLD.business_license_image_url OR
    NEW.food_permit_url IS DISTINCT FROM OLD.food_permit_url OR
    NEW.legal_person_id_front_url IS DISTINCT FROM OLD.legal_person_id_front_url OR
    NEW.legal_person_id_back_url IS DISTINCT FROM OLD.legal_person_id_back_url
)
EXECUTE FUNCTION public.tr_on_merchant_application_image_update();
