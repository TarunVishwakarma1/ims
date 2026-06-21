-- 000017_banners.down.sql

DROP INDEX IF EXISTS uniq_banner_hero_active;
DROP INDEX IF EXISTS idx_banners_active;
DROP TABLE IF EXISTS banners;
