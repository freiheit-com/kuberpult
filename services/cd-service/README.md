cd-service
==================

Configuration
-------------------

Configuration is done using environment variables. The following environment variables are supported:

`KUBERPULT_GIT_URL` sets the git url of the remote. This can be any url understood by git but currently authentication is only implemented for ssh.

`KUBERPULT_PGP_KEY_RING` sets the pgp key ring. The pgp key ring is a file containing all public keys in armored form. To export a keyring use `gpg --armor --export`.


Uploading manifests
----------------------

The cd-service exposes a REST-endpoint for uploading manifests on `/release`.


Signing manifests
-----------------------

The cd-service can verify the signatures of manifests uploaded to `/release`.

In order to have the signature verified, export all valid public keys to an armored keyring.

```
$> gpg --armor --export ci@yourcompany.com > keyring.gpg 
```

Start the cd server with the environment variable `KUBERPULT_PGP_KEY_RING` set to the path of the `keyring.gpg` file.

If you are using helm you can set the value `pgp.keyRing` to the content of the `keyring.gpg` file.

Kuberpult will now reject all mannifests without valid signature.

Now sign your manifest files.

```
# given that the manifests.yaml contains a valid manifest
$> gpg --armor --detach --sign < manifest.ymal > manifest.yaml.sig
$> curl -F "application=test" -F "manifests[production]=@manifests.yaml" -F "signatures[production]=@manifests.yaml.sig"  https://kuberpult.yourcompany.com/release
$> curl -F "application=test" -F "manifests[production]=@manifests.yaml" -F "signatures[production]=@manifests.yaml.sig"  https://kuberpult.yourcompany.com/release
```

Datadog Metrics
---------------------
The cd-service uploads multiple metrics to datadog.

These metrics include:
* `env_lock_count` - the count of current environment locks, for a given environment; \[❗ deprecated\]
* `environment_lock_count` - the count of current environment locks, for a given environment;
* `app_lock_count` - the count of current environment application locks, for a given application in a given environment; \[❗ deprecated\]
* `application_lock_count` - the count of current environment application locks, for a given application in a given environment;
* `lastDeployed` - the time since the last deployment in minutes;
* `request_queue_size` - the current size of the request queue;


