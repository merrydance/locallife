-- Trigger Function to sync auth.users -> public.users
CREATE OR REPLACE FUNCTION public.handle_new_user() 
RETURNS TRIGGER AS $$
BEGIN
  INSERT INTO public.users (id, wechat_openid, full_name)
  VALUES (
    new.id,
    new.raw_user_meta_data->>'openid', -- Pass openid via metadata during SignUp
    '微信用户'
  )
  ON CONFLICT (id) DO NOTHING; -- Handle potential duplicates gracefully
  RETURN new;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- The Trigger
DROP TRIGGER IF EXISTS on_auth_user_created ON auth.users;
CREATE TRIGGER on_auth_user_created
  AFTER INSERT ON auth.users
  FOR EACH ROW EXECUTE PROCEDURE public.handle_new_user();
