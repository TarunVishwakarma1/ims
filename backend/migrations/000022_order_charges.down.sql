ALTER TABLE orders
  DROP COLUMN IF EXISTS gst_paise,
  DROP COLUMN IF EXISTS packing_paise,
  DROP COLUMN IF EXISTS handling_paise,
  DROP COLUMN IF EXISTS surge_paise,
  DROP COLUMN IF EXISTS cod_round_paise;
