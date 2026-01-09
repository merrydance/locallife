-- Enable RLS on core tables
ALTER TABLE merchants ENABLE ROW LEVEL SECURITY;
ALTER TABLE dishes ENABLE ROW LEVEL SECURITY;
ALTER TABLE orders ENABLE ROW LEVEL SECURITY;
ALTER TABLE users ENABLE ROW LEVEL SECURITY;

-- 1. Merchants Table
-- Public Read
CREATE POLICY "Public read access" ON merchants
FOR SELECT USING (true);

-- Owner Write
CREATE POLICY "Owner update access" ON merchants
FOR UPDATE USING (auth.uid() = owner_user_id);

-- 2. Dishes Table
-- Public Read
CREATE POLICY "Public read access" ON dishes
FOR SELECT USING (true);

-- Merchant Owner Write
CREATE POLICY "Merchant Owner write access" ON dishes
FOR ALL USING (
  EXISTS (
    SELECT 1 FROM merchants
    WHERE merchants.id = dishes.merchant_id
    AND merchants.owner_user_id = auth.uid()
  )
);

-- 3. Orders Table
-- User Read Own Orders
CREATE POLICY "User read own orders" ON orders
FOR SELECT USING (auth.uid() = user_id);

-- Merchant Read Orders
CREATE POLICY "Merchant read orders" ON orders
FOR SELECT USING (
  EXISTS (
    SELECT 1 FROM merchants
    WHERE merchants.id = orders.merchant_id
    AND merchants.owner_user_id = auth.uid()
  )
);

-- User Insert
CREATE POLICY "User create order" ON orders
FOR INSERT WITH CHECK (auth.uid() = user_id);


