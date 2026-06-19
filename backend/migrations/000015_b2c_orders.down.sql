-- Cannot safely restore NOT NULL without first ensuring no NULL values exist.
-- This is a one-way relaxation for B2C orders. Down migration is a no-op.
SELECT 1;
