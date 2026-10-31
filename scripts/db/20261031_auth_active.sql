-- Repair legacy auth activation columns without dropping data or changing known states.
-- Run with psql -v ON_ERROR_STOP=1 against the API database; schema = current_schema().
BEGIN;
SET LOCAL lock_timeout = '10s';

DO $migration$
DECLARE
    target text;
    target_schema text := current_schema();
BEGIN
    FOREACH target IN ARRAY ARRAY['companies', 'users'] LOOP
        IF to_regclass(format('%I.%I', target_schema, target)) IS NULL THEN
            CONTINUE;
        END IF;
        EXECUTE format('ALTER TABLE %I.%I ADD COLUMN IF NOT EXISTS active boolean', target_schema, target);
        IF EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = target_schema AND table_name = target AND column_name = 'is_active'
        ) THEN
            EXECUTE format('UPDATE %I.%I SET active = COALESCE(is_active, false) WHERE active IS NULL', target_schema, target);
        ELSE
            EXECUTE format('UPDATE %I.%I SET active = false WHERE active IS NULL', target_schema, target);
        END IF;
        EXECUTE format('ALTER TABLE %I.%I ALTER COLUMN active SET NOT NULL', target_schema, target);
    END LOOP;
END
$migration$;

COMMIT;
