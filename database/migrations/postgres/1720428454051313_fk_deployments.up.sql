ALTER TABLE deployments
ADD COLUMN IF NOT EXISTS transformerEslId INTEGER default 0;