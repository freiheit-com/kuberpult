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