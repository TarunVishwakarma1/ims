ALTER TABLE users ADD COLUMN org_id UUID;

-- Backfill existing users to the default organization
UPDATE users SET org_id = '00000000-0000-0000-0000-000000000000';

ALTER TABLE users ALTER COLUMN org_id SET NOT NULL;
ALTER TABLE users ADD CONSTRAINT users_org_id_fkey FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE;
