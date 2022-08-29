# Background
Kuberpult uses git-data (like time & author of a commit) and displays this in the UI.
This git-data requires parsing a long list of git commits, and is therefor slow and memory intensive.
We can improve this, by storing data explicitly, instead of relying on the git-history.
We do this anyway for some things like author.
* The releases script goes over all releases for all applications, and writes the commit date
  (the date when this release was created) to a new file `created_at`
* The locks script goes over all env and app locks, removes the lock **file** and creates a **directory**
  with the same name instead. Then it writes the commit date (the date when this lock was created)
  as well as the author name and email and the message into files in the new lock directory.
* The scripts only create the files and do not add or push to GitHub.

The new kuberpult will only read the data in the new format and will ignore old data which is normally deleted by the script.

## Deployment with Downtime
1) Shut down kuberpult: Delete StatefulSet
2) Write data to manifest repo, this could happen locally during downtime
   * clone the manifests repository depends on where Kuberpult is deployed.
   * cd and run the scripts in the top dir.
   * Add the created files. and commit the changes to the manifests
     repo (use a different branch to get approval first).
     ```bash
     git clone $MANIFEST_REPO
     git checkout -b kuberpult_migrate
     ./create-metadata-locks.sh
     ./create-metadata-releases.sh
     git add .
     git commit -m "migration to new kuberpult"
     git push
     ```
3) Merge and then Deploy new kuberpult version that reads new data
