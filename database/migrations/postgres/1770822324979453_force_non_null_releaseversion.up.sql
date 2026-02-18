ALTER TABLE deployments
DROP CONSTRAINT IF EXISTS releaseversion_not_null;

ALTER TABLE deployments
ADD CONSTRAINT releaseversion_not_null CHECK (releaseversion IS NOT NULL) NOT VALID;