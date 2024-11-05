# Stress Tests - Data Generation Scripts

## Generating the environments

The generate_environments.sh script is setup to receive two parameters:

1. The base name of the environments that you want to create, BASE_ENV_NAME,
2. The number of environments you want to create, NUMBER_ENVS,
3. Upstream environment name, UPSTREAM_ENV.

Running the script as such:
```shell
./generate_environments.sh qa 32 testing
```
Creates 32 environments under the qa environment group (each name qa-${COUNTRY_CODE}, where 
the country codes are listed under the country_codes.csv file). All these environments have as upstream
environment the testing environment, also created by the script.

Running this script not only generates the necessary files under ./environments, but also publishes them to a 
local instance of kuberpult.


## Generating applications data

For generating application data, we have the create_releases.sh script. It is setup to accept three parameters:
1. The base app name of the applications that you want to create,
2. The team of these applications,
3. The total number of applications you want to create.

Running the script as such:
```shell
./create-releases.sh stress-testing-app sreteam 1000
```
creates 1000 applications name stress-testing-app-{0-999} under the sreteam. If you use the generate_environments.sh 
these applications will be automatically deployed to testing and have a manifest for all the qa environments.

For each app, the script also creates a random number of releases, between 10 and 20.


## Know Issues

### Time

First of all, creating this much data takes a long time. The manifest-repo-export-service takes and increasingly 
bigger amount of time processing one event the greater the number of apps. This is because of the [UpdateArgoApps](https://github.com/freiheit-com/kuberpult/blob/127d6ec37801b0c420688847f0c0ee113459eb77/services/manifest-repo-export-service/pkg/repository/repository.go#L700) steps 
that is run after each transformer. Our recommendation is to comment out this step if your desire is to simply generate
a lot of data.

### Space 

It is not uncommon for the postgres container to throw an error akin to something like:
```
no space left on device
```

This can usually be solved by running:
```shell
docker system prune
```
It is recommended that you run this command before you start generating a lot of data.



