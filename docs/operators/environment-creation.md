#### Environment Creation

Kuberpult offers an API endpoint for environment creation. This endpoint is expecting the following information:
* **IAP Token**:
  * If IAP is enabled ( in helm: `ingress.iap.enabled: false`) You need to provide kuberpult with your own IAP access token. We use google cloud authentication in order generate it. For more information on programmatic authentication, please refere to [this resource](https://cloud.google.com/iap/docs/authentication-howto).
* **Environment Name**
  * The name of the environment you are trying to create.
* **Configuration Data**:
  * You need to provide some configuration specifications to your environment. The provided data must be in JSON format. [This doc](../users/3_environment.md) goes into detail regarding the information that must be contained within your configuration file. You can find an example file [here](../infrastructure/scripts/create-testdata/testdata_template/environments/staging/config.json). Note that it only accepts ArgoCD configurations as `argo_configs` but not for `argocd`.
* **Dry Run**:
  * This is for the validation purpose of the request data, and is optional

```shell
curl -f -X POST -H "Authorization: Bearer $IAPToken" \
                -H "multipart/form-data" --form-string "config=$DATA" \
                $KUBERPULT_API_URL/api/environments/$ENVIRONMENT_NAME
```

**Example:**

Below you can find an example of the curl request that creates an environment named `staging`. It accesses a kuberpult instance running locally on port `8081` and reads a `config.json` file from the local files system that contains the configuration data for the environment.
For local development, the IAPToken can be omitted.

```shell
DATA=$(cat config.json)
curl -f -X POST -H "multipart/form-data" --form-string "config=$DATA" \
                http://localhost:8081/api/environments/staging
```

**IMPORTANT**

In the past, the common way to change the environment configuration (`config.json` files) was to directly edit the files in the manifest repo and push.
This does not work anymore, the environment configuration must be set via the REST endpoint mentioned above.
Kuberpult should be the only one writing to the manifest repository. **Direct manipulation of the manifest repository should be avoided.**