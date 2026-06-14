CREATE TABLE roles (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(50) UNIQUE NOT NULL,
    description TEXT,
    is_system   BOOLEAN DEFAULT false,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE permissions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    resource    VARCHAR(50) NOT NULL,
    action      VARCHAR(50) NOT NULL,
    description TEXT,
    UNIQUE(resource, action)
);

CREATE TABLE role_permissions (
    role_id       UUID REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

-- Seed built-in permissions
INSERT INTO permissions (resource, action, description) VALUES
  ('users',      'view',   'View users list'),
  ('users',      'create', 'Create new users'),
  ('users',      'edit',   'Edit existing users'),
  ('users',      'delete', 'Delete users'),
  ('products',   'view',   'View products'),
  ('products',   'manage', 'Create/edit/delete products'),
  ('categories', 'view',   'View categories'),
  ('categories', 'manage', 'Create/edit/delete categories'),
  ('inventory',  'view',   'View inventory'),
  ('inventory',  'manage', 'Update inventory'),
  ('orders',     'view',   'View orders'),
  ('orders',     'create', 'Create orders'),
  ('orders',     'manage', 'Update/delete orders'),
  ('roles',      'manage', 'Manage roles and permissions');

-- Seed built-in roles
INSERT INTO roles (name, description, is_system) VALUES
  ('admin',   'Full system access', true),
  ('manager', 'Operational access', true),
  ('staff',   'Basic access',       true);

-- Seed role_permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'admin';  -- admin gets all

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r
JOIN permissions p ON (p.resource || ':' || p.action) IN (
  'users:view','users:create',
  'products:view','products:manage',
  'categories:view','categories:manage',
  'inventory:view','inventory:manage',
  'orders:view','orders:create','orders:manage'
) WHERE r.name = 'manager';

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r
JOIN permissions p ON (p.resource || ':' || p.action) IN (
  'products:view','categories:view',
  'inventory:view','orders:view','orders:create'
) WHERE r.name = 'staff';
