-- Tear down the entire schema. Used by `migrate down` during dev resets.
-- pg's DROP SCHEMA CASCADE removes every table, function, type, and
-- extension that 000001 created.

DROP SCHEMA public CASCADE;
CREATE SCHEMA public;
GRANT ALL ON SCHEMA public TO ims;
GRANT ALL ON SCHEMA public TO PUBLIC;
