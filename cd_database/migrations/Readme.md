# Migrations

## Prefix
Migrations are sorted by the filename, oldest first.

To create a proper unix timestamp - seconds since 1970 in UTC - run
```shell
date +%s%6N
```
If you are on macOS make sure you are running the coreutils version of the date command: gdate


## Postfix

All files must end in `.up.sql`.

