This directory contains files that can be used with `evans` to make grpc calls on the command line (non-interactive).

Example to call "delete env from app" via evans:
```shell
cat batch-delete-app-env.json | \
evans --header author-name=YXV0aG9y --header author-email=YXV0aG9yQGF1dGhvcg== --host localhost --port 8443 -r cli call api.v1.BatchService.ProcessBatch
```
