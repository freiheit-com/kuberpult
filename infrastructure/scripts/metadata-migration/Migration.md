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
  
  
  
The new kuberpult will only read the data in the new format and will ignore old data which is normally deleted by the script.

## Deployment with Downtime
1) Shut down kuberpult: Delete StatefulSet
2) Write data to manifest repo, this could happen locally during downtime
3) Deploy new kuberpult version that reads new data
