# Minor Commits
## Definition
Sometimes there are some commits that do not change any manifests in any environments at all or only change some specific lines. We consider these commits as "Minor".

In the helm charts under `cd.minorRegexes` you can specify a list of regexs. Any line in the manifests that match with any of the regexes are ignored during the process of finding out whether the release is minor or not.

## Cleanup Process
Usually during the cleanup process kuberpult will keep some releases (20 by default, see `git.releaseVersionsLimit` in values.yaml) and remove all old releases other than that. During this process kuberpult will skip these "minor" releases, meaning that it will keep at least 20 releases, that are not minor.

## UI
The minor releases are shown in the ui with a 💤 emoji in front of their commit message.
