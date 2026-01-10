-- Create storage buckets
insert into storage.buckets (id, name, public, file_size_limit, allowed_mime_types)
values 
  ('assets', 'assets', true, 10485760, '{image/jpeg,image/png,image/webp}'),
  ('identity', 'identity', false, 10485760, '{image/jpeg,image/png,image/webp}')
on conflict (id) do nothing;

-- Set up RLS for 'assets' bucket (Publicly readable by authenticated users)
-- Note: User mentioned "anyone who opens the miniprogram is silently logged in", 
-- so 'authenticated' role is sufficient.
create policy "Assets are publicly readable by authenticated users"
on storage.objects for select
to authenticated
using ( bucket_id = 'assets' );

create policy "Users can upload their own assets"
on storage.objects for insert
to authenticated
with check (
  bucket_id = 'assets' AND
  (storage.foldername(name))[1] = auth.uid()::text
);

-- Set up RLS for 'identity' bucket (Private, only owner can read)
create policy "Identity documents are readable by owners only"
on storage.objects for select
to authenticated
using (
  bucket_id = 'identity' AND
  (storage.foldername(name))[1] = auth.uid()::text
);

create policy "Users can upload their own identity documents"
on storage.objects for insert
to authenticated
with check (
  bucket_id = 'identity' AND
  (storage.foldername(name))[1] = auth.uid()::text
);

-- Note: We might need a policy for service_role to manage all files
create policy "Service role has full access"
on storage.objects for all
to service_role
using ( true )
with check ( true );
