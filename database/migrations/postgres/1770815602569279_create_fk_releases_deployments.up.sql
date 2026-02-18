ALTER TABLE deployments
DROP CONSTRAINT IF EXISTS fk_releases_deployments;

ALTER TABLE deployments
ADD CONSTRAINT fk_releases_deployments FOREIGN KEY (
    appname,
    releaseVersion,
    revision
) REFERENCES releases (
    appname,
    releaseVersion,
    revision
) NOT VALID;