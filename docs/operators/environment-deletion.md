#### Environment Deletion

This endpoint removes one environment, including all locks that are attached to it (app locks, environment locks, and team locks).

The environment deletion API endpoint is expecting the following information:
* **IAP Token**:
  * If IAP is enabled ( in helm: `ingress.iap.enabled: true`) You need to provide kuberpult with your own IAP access token. We use google cloud authentication in order generate it. For more information on programmatic authentication, please refere to [this resource](https://cloud.google.com/iap/docs/authentication-howto).
* **Environment Name**
  * The name of the environment you are trying to delete.

```shell
curl -f -X DELETE -H "Authorization: Bearer $IAPToken" $KUBERPULT_API_URL/api/environments/$ENVIRONMENT_NAME
```

**Example:**

Below you can find the example for the curl request for removing the environment `staging` on the Kuberpult API hosted at `localhost:8081`. For local development, the IAPToken can be omitted.

```shell
curl -f -X DELETE http://localhost:8081/api/environments/staging
```