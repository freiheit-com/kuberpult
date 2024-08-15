# Minor Commits
Sometimes there are some commits that do not change any manifests in any environments at all. We consider these commits as "Minor".
Usually during the cleanup process kuberpult will keep some releases (20 by default, see `git.releaseVersionsLimit` in values.yaml) and remove all old releases other than that. During this process kuberpult will skip these "minor" releases, meaning that it will keep at least 20 releases, that are not minor.
