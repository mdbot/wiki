# Wiki

Stores data in git.

## Encryption key

In order to persist settings (user accounts, session keys, CSRF tokens),
you must provide an encryption key either as a CLI argument (`-key`) or
an environment variable (`KEY`). The key should be 32 bytes and
hex-encoded; you can generate such a key using `openssl rand -hex 32`.

## User accounts

You can specify a default username and password using the `username`
and `password` CLI flags, or the `USERNAME` and `PASSWORD` env vars.
These will be used to create a new user if no others exist.

## Directories

All paths are relative to the working directory, in the container this is /

 - <working directory>/data - Used to store data
 - <working directory>/templates - Used to provide custom templates
 - <working directory>/static - Used to provide custom static content

## Docker

 - working directory is /
 - runs as user 65532:65532
