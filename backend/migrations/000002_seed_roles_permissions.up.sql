-- Seed Roles
INSERT INTO public.roles (name, description, is_system) VALUES 
('admin', 'System administrator with full access', true),
('manager', 'Manager with access to most resources', true),
('staff', 'Standard staff member', true)
ON CONFLICT (name) DO NOTHING;

-- Seed Permissions
INSERT INTO public.permissions (resource, action, description) VALUES
('users', 'view', 'View users'),
('users', 'create', 'Create users'),
('users', 'edit', 'Edit users'),
('users', 'delete', 'Delete users'),
('products', 'view', 'View products'),
('products', 'manage', 'Manage products'),
('categories', 'view', 'View categories'),
('categories', 'manage', 'Manage categories'),
('inventory', 'view', 'View inventory'),
('inventory', 'manage', 'Manage inventory'),
('orders', 'view', 'View orders'),
('orders', 'create', 'Create orders'),
('orders', 'manage', 'Manage orders'),
('roles', 'manage', 'Manage roles'),
('locations', 'manage', 'Manage org locations')
ON CONFLICT (resource, action) DO NOTHING;

-- Map Permissions to Admin
INSERT INTO public.role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM public.roles r, public.permissions p
WHERE r.name = 'admin'
ON CONFLICT DO NOTHING;

-- Map Permissions to Manager (View all, manage most except roles/locations)
INSERT INTO public.role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM public.roles r, public.permissions p
WHERE r.name = 'manager' AND p.resource IN ('products', 'categories', 'inventory', 'orders')
ON CONFLICT DO NOTHING;

-- Map Permissions to Staff (View only + create orders)
INSERT INTO public.role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM public.roles r, public.permissions p
WHERE r.name = 'staff' AND (p.action = 'view' OR (p.resource = 'orders' AND p.action = 'create'))
ON CONFLICT DO NOTHING;
