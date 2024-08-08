/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the Expat(MIT) License as published by
the Free Software Foundation.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
MIT License for more details.

You should have received a copy of the MIT License
along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

Copyright freiheit.com*/

package main_test

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sYaml "sigs.k8s.io/yaml" //Needed to properly parse the yaml generated by helm, "gopkg.in/yaml.v3" cannot do it properly
)

func runHelm(t *testing.T, valuesData []byte, dirName string) (string, error) {
	testId := strconv.Itoa(rand.Intn(9999))
	tempValuesFile := "vals" + "_" + testId + ".yaml"
	tempValuesFile = dirName + "/" + tempValuesFile

	err := os.WriteFile(tempValuesFile, valuesData, 0644)
	if err != nil {
		t.Fatalf("Error writing vals file . %v", err)
	}
	outputFile := "tmp_" + testId + ".tmpl"
	outputFile = dirName + "/" + outputFile

	op, err := exec.Command("sh", "-c", "helm template ./.. --values "+tempValuesFile+" > "+outputFile).CombinedOutput()

	if err != nil {
		t.Logf("Error generating chart: %v: %s", err, string(op))
		return "", err
	}

	fileContent, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Error reading file '%s' content: \n%s\n", outputFile, string(fileContent))
		return "", nil
	}
	t.Logf("output file: \n%s\n", fileContent)

	return outputFile, nil
}

func CheckForEnvVariable(t *testing.T, target core.EnvVar, deployment *apps.Deployment) bool {
	for _, container := range deployment.Spec.Template.Spec.Containers {
		for _, env := range container.Env {
			if env.Name == target.Name {
				if env.Value == target.Value {
					return true
				} else {
					t.Logf("Found '%s' env variable. Value mismatch: wanted: '%s', got: '%s'.\n", target.Name, target.Value, env.Value)
				}
			}
		}
	}
	return false
}

func searchSimpleKeyValuePair(yaml []string, key string, value string) bool {
	keyTarget := key + ":"
	for _, line := range yaml {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, keyTarget) {
			if strings.Contains(line[len(keyTarget):], value) {
				return true
			}
		}
	}
	return false
}

func checkDeployment(lines []string) bool {
	return searchSimpleKeyValuePair(lines, "kind", "Deployment")
}

// Check template files at ../templates/
func getDeployments(fileName string) (map[string]apps.Deployment, error) {
	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("error reading yaml file '%s': %w", fileName, err)
	}
	output := map[string]apps.Deployment{}               // Name of deployment -> configuration
	deploymentYAML := strings.Split(string(data), "---") //Split into documents
	for _, document := range deploymentYAML {
		if checkDeployment(strings.Split(document, "\n")) {
			var target apps.Deployment
			if err := k8sYaml.Unmarshal([]byte(document), &target); err != nil {
				return nil, fmt.Errorf("Unmarshalling deployment failed failed: %s\n", document)
			}
			output[target.Name] = target
		}
	}
	return output, nil
}

func checkIngress(lines []string) bool {
	return searchSimpleKeyValuePair(lines, "kind", "Ingress")
}

func getIngress(fileName string) (*networking.Ingress, error) {
	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("error reading yaml file '%s': %w", fileName, err)
	}
	output := networking.Ingress{}
	allYamls := strings.Split(string(data), "---")

	for _, document := range allYamls {
		if checkIngress(strings.Split(document, "\n")) {
			if err := k8sYaml.Unmarshal([]byte(document), &output); err != nil {
				return nil, fmt.Errorf("Unmarshalling ingress failed: %s\n", document)
			}
		}
	}
	return &output, nil
}

func TestHelmChartsKuberpultCdEnvVariables(t *testing.T) {
	tcs := []struct {
		Name            string
		Values          string
		ExpectedEnvs    []core.EnvVar
		ExpectedMissing []core.EnvVar
	}{
		{
			Name: "Minimal values.yaml leads to proper default values",
			Values: `
git:
  url:  "testURL"
ingress:
  domainName: "kuberpult-example.com"
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_DB_OPTION",
					Value: "NO_DB",
				},
			},
			ExpectedMissing: []core.EnvVar{},
		},
		{
			Name: "Basic Parsing works",
			Values: `
git:
  url:  "testURL"
pgp:
  keyRing: ""
ingress:
  domainName: "kuberpult-example.com"

environment_configs:
  environment_configs_json: "{}"
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_GIT_URL",
					Value: "testURL",
				},
			},
			ExpectedMissing: []core.EnvVar{
				{
					Name:  "KUBERPULT_PGP_KEY_RING_PATH",
					Value: "",
				},
			},
		},
		{
			Name: "Change Git URL",
			Values: `
git:
  url:  "checkThisValue"
ingress:
  domainName: "kuberpult-example.com"
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_GIT_URL",
					Value: "checkThisValue",
				},
			},
			ExpectedMissing: []core.EnvVar{},
		},
		{
			Name: "Argo CD disabled",
			Values: `
git:
  url: "testURL"
ingress:
  domainName: "kuberpult-example.com"
argocd:
  generateFiles: false
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_ARGO_CD_GENERATE_FILES",
					Value: "false",
				},
				{
					Name:  "KUBERPULT_GIT_URL",
					Value: "testURL",
				},
			},
			ExpectedMissing: []core.EnvVar{},
		},
		{
			Name: "Argo CD enabled simple test",
			Values: `
git:
  url: "testURL"
ingress:
  domainName: "kuberpult-example.com"
argocd:
  generateFiles: true
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_ARGO_CD_GENERATE_FILES",
					Value: "true",
				},
				{
					Name:  "KUBERPULT_GIT_URL",
					Value: "testURL",
				},
			},
			ExpectedMissing: []core.EnvVar{},
		},
		{
			Name: "DD Metrics disabled",
			Values: `
git:
  url: "testURL"
ingress:
  domainName: "kuberpult-example.com"
dataDogTracing:
  enabled: false
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_GIT_URL",
					Value: "testURL",
				},
			},
			ExpectedMissing: []core.EnvVar{
				{
					Name:  "DD_AGENT_HOST",
					Value: "",
				},
				{
					Name:  "DD_ENV",
					Value: "",
				},
				{
					Name:  "DD_SERVICE",
					Value: "",
				},
				{
					Name:  "DD_VERSION",
					Value: "",
				},
				{
					Name:  "KUBERPULT_ENABLE_TRACING",
					Value: "",
				},
				{
					Name:  "DD_TRACE_DEBUG",
					Value: "",
				},
			},
		},
		{
			Name: "DD Tracing enabled",
			Values: `
git:
  url: "testURL"
ingress:
  domainName: "kuberpult-example.com"
datadogTracing:
  enabled: true
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "DD_AGENT_HOST",
					Value: "",
				},
				{
					Name:  "DD_ENV",
					Value: "",
				},
				{
					Name:  "DD_SERVICE",
					Value: "",
				},
				{
					Name:  "DD_VERSION",
					Value: "",
				},
				{
					Name:  "KUBERPULT_ENABLE_TRACING",
					Value: "true",
				},
				{
					Name:  "DD_TRACE_DEBUG",
					Value: "false",
				},
			},
			ExpectedMissing: []core.EnvVar{},
		},
		{
			Name: "Two variables involved web hook disabled",
			Values: `
git:
  url: "testURL"
ingress:
  domainName: "kuberpult-example.com"
argocd:
  sendWebHooks: false
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_GIT_URL",
					Value: "testURL",
				},
			},
			ExpectedMissing: []core.EnvVar{},
		},
		{
			Name: "Two variables involved web hook enabled",
			Values: `
git:
  url: "testURL"
ingress:
  domainName: "kuberpult-example.com"
argocd:
  sendWebHooks: false
  server: testServer
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_GIT_URL",
					Value: "testURL",
				},
			},
			ExpectedMissing: []core.EnvVar{},
		},
		{
			Name: "Database disabled",
			Values: `
git:
  url: "testURL"
ingress:
  domainName: "kuberpult-example.com"
db:
  dbOption: NO_DB
`,
			ExpectedEnvs: []core.EnvVar{},
			ExpectedMissing: []core.EnvVar{

				{
					Name:  "KUBERPULT_DB_LOCATION",
					Value: "",
				},
				{
					Name:  "KUBERPULT_DB_NAME",
					Value: "",
				},
				{
					Name:  "KUBERPULT_DB_USER_NAME",
					Value: "",
				},
				{
					Name:  "KUBERPULT_DB_USER_PASSWORD",
					Value: "",
				},
			},
		},
		{
			Name: "Database postgreSQL enabled 1",
			Values: `
git:
  url: "testURL"
ingress:
  domainName: "kuberpult-example.com"
db:
  dbOption: postgreSQL
  location: "127.0.0.1"
  dbName: dbName
  dbUser: dbUser
  dbPassword: dbPassword
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_DB_OPTION",
					Value: "postgreSQL",
				},
				{
					Name:  "KUBERPULT_DB_LOCATION",
					Value: "127.0.0.1",
				},
				{
					Name:  "KUBERPULT_DB_NAME",
					Value: "dbName",
				},
			},
			ExpectedMissing: []core.EnvVar{
				{
					Name:  "KUBERPULT_DB_USER_NAME",
					Value: "dbUser",
				},
				{
					Name:  "KUBERPULT_DB_USER_PASSWORD",
					Value: "dbPassword",
				},
			},
		},
		{
			Name: "database username and password not in environment variables",
			Values: `
git:
  url: "testURL"
ingress:
  domainName: "kuberpult-example.com"
db:
  dbOption: postgreSQL
  location: /kp/database
  dbName: does
  dbUser: username
  dbPassword: password
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_DB_OPTION",
					Value: "postgreSQL",
				},
				{
					Name:  "KUBERPULT_DB_LOCATION",
					Value: "/kp/database",
				},
			},
			ExpectedMissing: []core.EnvVar{
				{
					Name:  "KUBERPULT_DB_USER_NAME",
					Value: "username",
				},
				{
					Name:  "KUBERPULT_DB_USER_PASSWORD",
					Value: "password",
				},
			},
		},
		{
			Name: "Database writeEslTableOnly=false ",
			Values: `
git:
  url: "testURL"
ingress:
  domainName: "kuberpult-example.com"
db:
  dbOption: postgreSQL
  location: /kp/database
  dbName: does
  dbUser: not
  dbPassword: matter
  writeEslTableOnly: false
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_DB_OPTION",
					Value: "postgreSQL",
				},
				{
					Name:  "KUBERPULT_DB_LOCATION",
					Value: "/kp/database",
				},
				{
					Name:  "KUBERPULT_DB_WRITE_ESL_TABLE_ONLY",
					Value: "false",
				},
			},
			ExpectedMissing: []core.EnvVar{
				{
					Name:  "KUBERPULT_DB_NAME",
					Value: "",
				},
				{
					Name:  "KUBERPULT_DB_USER_NAME",
					Value: "does",
				},
				{
					Name:  "KUBERPULT_DB_USER_PASSWORD",
					Value: "not",
				},
			},
		},
		{
			Name: "Database writeEslTableOnly=true",
			Values: `
git:
  url: "testURL"
ingress:
  domainName: "kuberpult-example.com"
db:
  dbOption: postgreSQL
  location: /kp/database
  dbName: does
  dbUser: not
  dbPassword: matter
  writeEslTableOnly: true
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_DB_OPTION",
					Value: "postgreSQL",
				},
				{
					Name:  "KUBERPULT_DB_LOCATION",
					Value: "/kp/database",
				},
				{
					Name:  "KUBERPULT_DB_WRITE_ESL_TABLE_ONLY",
					Value: "true",
				},
			},
			ExpectedMissing: []core.EnvVar{
				{
					Name:  "KUBERPULT_DB_NAME",
					Value: "",
				},
				{
					Name:  "KUBERPULT_DB_USER_NAME",
					Value: "does",
				},
				{
					Name:  "KUBERPULT_DB_USER_PASSWORD",
					Value: "not",
				},
			},
		},
		{
			Name: "Test default releaseVersionsLimit",
			Values: `
git:
  url: "testURL"
ingress:
  domainName: "kuberpult-example.com"
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_RELEASE_VERSIONS_LIMIT",
					Value: "20",
				},
			},
			ExpectedMissing: []core.EnvVar{},
		},
		{
			Name: "Test overwriting releaseVersionsLimit",
			Values: `
git:
  url: "testURL"
  releaseVersionsLimit: 15
ingress:
  domainName: "kuberpult-example.com"
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_RELEASE_VERSIONS_LIMIT",
					Value: "15",
				},
			},
			ExpectedMissing: []core.EnvVar{},
		},
		{
			Name: "Test default ssl mode",
			Values: `
git:
  url: "testURL"
  releaseVersionsLimit: 15
db:
  dbOption: postgreSQL
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_DB_SSL_MODE",
					Value: "verify-full",
				},
			},
			ExpectedMissing: []core.EnvVar{},
		},
		{
			Name: "Test  ssl mode required",
			Values: `
git:
  url: "testURL"
  releaseVersionsLimit: 15
db:
  dbOption: postgreSQL
  sslMode: prefer
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_DB_SSL_MODE",
					Value: "prefer",
				},
			},
			ExpectedMissing: []core.EnvVar{},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			testDirName := t.TempDir()
			outputFile, err := runHelm(t, []byte(tc.Values), testDirName)
			if err != nil {
				t.Fatalf(fmt.Sprintf("%v", err))
			}
			if out, err := getDeployments(outputFile); err != nil {
				t.Fatalf(fmt.Sprintf("%v", err))
			} else {
				targetDocument := out["kuberpult-cd-service"]
				for _, env := range tc.ExpectedEnvs {
					if !CheckForEnvVariable(t, env, &targetDocument) {
						t.Fatalf("Environment variable '%s' with value '%s' was expected, but not found.", env.Name, env.Value)
					}
				}
				for _, env := range tc.ExpectedMissing {
					if CheckForEnvVariable(t, env, &targetDocument) {
						t.Fatalf("Found enviroment variable '%s' with value '%s', but was not expecting it.", env.Name, env.Value)
					}
				}

			}
		})
	}
}

func TestHelmChartsKuberpultDeprecatedParameters(t *testing.T) {
	tcs := []struct {
		Name            string
		Values          string
		ExpectedEnvs    []core.EnvVar
		ExpectedMissing []core.EnvVar
	}{
		{
			Name: "Test Argocd.sendWebhooks being set to true",
			Values: `
git:
  url:  "checkThisValue"
ingress:
  domainName: "kuberpult-example.com"
argocd:
  sendWebhooks: true
`},
		{
			Name: "Test Bootstrap_mode being set to true",
			Values: `
git:
  url:  "checkThisValue"
ingress:
  domainName: "kuberpult-example.com"
environment_configs:
  bootstrap_mode: true
`},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			testDirname := t.TempDir()
			_, err := runHelm(t, []byte(tc.Values), testDirname)
			if err == nil {
				t.Fatalf("Paramater should be deprecated")
			}
		})
	}
}

func TestHelmChartsKuberpultManifestExportEnvVariables(t *testing.T) {
	tcs := []struct {
		Name            string
		Values          string
		ExpectedEnvs    []core.EnvVar
		ExpectedMissing []core.EnvVar
	}{
		{
			Name: "Change Git URL",
			Values: `
git:
  url:  "checkThisValue"
ingress:
  domainName: "kuberpult-example.com"
db:
  dbOption: "postgreSQL"
  writeEslTableOnly: false
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_GIT_URL",
					Value: "checkThisValue",
				},
			},
			ExpectedMissing: []core.EnvVar{},
		},
		{
			Name: "Argo CD disabled",
			Values: `
git:
  url: "testURL"
ingress:
  domainName: "kuberpult-example.com"
argocd:
  generateFiles: false
db:
  dbOption: "postgreSQL"
  writeEslTableOnly: false
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_ARGO_CD_GENERATE_FILES",
					Value: "false",
				},
				{
					Name:  "KUBERPULT_GIT_URL",
					Value: "testURL",
				},
			},
			ExpectedMissing: []core.EnvVar{},
		},
		{
			Name: "Argo CD enabled simple test",
			Values: `
git:
  url: "testURL"
ingress:
  domainName: "kuberpult-example.com"
argocd:
  generateFiles: true
db:
  dbOption: "postgreSQL"
  writeEslTableOnly: false
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_ARGO_CD_GENERATE_FILES",
					Value: "true",
				},
				{
					Name:  "KUBERPULT_GIT_URL",
					Value: "testURL",
				},
			},
			ExpectedMissing: []core.EnvVar{},
		},
		{
			Name: "DD Metrics disabled",
			Values: `
git:
  url: "testURL"
ingress:
  domainName: "kuberpult-example.com"
dataDogTracing:
  enabled: false
db:
  dbOption: "postgreSQL"
  writeEslTableOnly: false
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_GIT_URL",
					Value: "testURL",
				},
			},
			ExpectedMissing: []core.EnvVar{
				{
					Name:  "DD_AGENT_HOST",
					Value: "",
				},
				{
					Name:  "DD_ENV",
					Value: "",
				},
				{
					Name:  "DD_SERVICE",
					Value: "",
				},
				{
					Name:  "DD_VERSION",
					Value: "",
				},
				{
					Name:  "KUBERPULT_ENABLE_TRACING",
					Value: "",
				},
				{
					Name:  "DD_TRACE_DEBUG",
					Value: "",
				},
			},
		},
		{
			Name: "DD Tracing enabled",
			Values: `
git:
  url: "testURL"
ingress:
  domainName: "kuberpult-example.com"
datadogTracing:
  enabled: true
db:
  dbOption: "postgreSQL"
  writeEslTableOnly: false
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "DD_AGENT_HOST",
					Value: "",
				},
				{
					Name:  "DD_ENV",
					Value: "",
				},
				{
					Name:  "DD_SERVICE",
					Value: "",
				},
				{
					Name:  "DD_VERSION",
					Value: "",
				},
				{
					Name:  "KUBERPULT_ENABLE_TRACING",
					Value: "true",
				},
				{
					Name:  "DD_TRACE_DEBUG",
					Value: "false",
				},
			},
			ExpectedMissing: []core.EnvVar{},
		},
		{
			Name: "Database disabled",
			Values: `
git:
  url: "testURL"
ingress:
  domainName: "kuberpult-example.com"
db:
  dbOption: NO_DB
  writeEslTableOnly: false
`,
			ExpectedEnvs: []core.EnvVar{},
			ExpectedMissing: []core.EnvVar{

				{
					Name:  "KUBERPULT_DB_LOCATION",
					Value: "",
				},
				{
					Name:  "KUBERPULT_DB_NAME",
					Value: "",
				},
				{
					Name:  "KUBERPULT_DB_USER_NAME",
					Value: "",
				},
				{
					Name:  "KUBERPULT_DB_USER_PASSWORD",
					Value: "",
				},
			},
		},
		{
			Name: "Database postgreSQL enabled 1",
			Values: `
git:
  url: "testURL"
ingress:
  domainName: "kuberpult-example.com"
db:
  dbOption: postgreSQL
  location: "127.0.0.1"
  dbName: dbName
  dbUser: dbUser
  dbPassword: dbPassword
  writeEslTableOnly: false
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_DB_OPTION",
					Value: "postgreSQL",
				},
				{
					Name:  "KUBERPULT_DB_LOCATION",
					Value: "127.0.0.1",
				},
				{
					Name:  "KUBERPULT_DB_NAME",
					Value: "dbName",
				},
			},
			ExpectedMissing: []core.EnvVar{},
		},
		{
			Name: "database username and password not in environment variables",
			Values: `
git:
  url: "testURL"
ingress:
  domainName: "kuberpult-example.com"
db:
  dbOption: postgreSQL
  location: /kp/database
  dbName: does
  dbUser: username
  dbPassword: password
  writeEslTableOnly: false
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_DB_OPTION",
					Value: "postgreSQL",
				},
				{
					Name:  "KUBERPULT_DB_LOCATION",
					Value: "/kp/database",
				},
			},
			ExpectedMissing: []core.EnvVar{
				{
					Name:  "KUBERPULT_DB_USER_NAME",
					Value: "username",
				},
				{
					Name:  "KUBERPULT_DB_USER_PASSWORD",
					Value: "password",
				},
			},
		},
		{
			Name: "ESL processing backoff is set",
			Values: `
git:
  url: "testURL"
ingress:
  domainName: "kuberpult-example.com"
db:
  dbOption: postgreSQL
  writeEslTableOnly: false

manifestRepoExport:
  eslProcessingIdleTimeSeconds: 5
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_ESL_PROCESSING_BACKOFF",
					Value: "5",
				},
			},
		},
		{
			Name: "ESL processing backoff is not set",
			Values: `
git:
  url: "testURL"
ingress:
  domainName: "kuberpult-example.com"
db:
  dbOption: postgreSQL
  writeEslTableOnly: false
`,
			ExpectedMissing: []core.EnvVar{
				{
					Name:  "KUBERPULT_ESL_PROCESSING_BACKOFF",
					Value: "",
				},
			},
		},
		{
			Name: "Release version limit set",
			Values: `
git:
  url: "testURL"
  releaseVersionsLimit: 15
ingress:
  domainName: "kuberpult-example.com"
db:
  dbOption: postgreSQL
  writeEslTableOnly: false
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_RELEASE_VERSIONS_LIMIT",
					Value: "15",
				},
			},
		},
		{
			Name: "Release version limit default",
			Values: `
git:
  url: "testURL"
ingress:
  domainName: "kuberpult-example.com"
db:
  dbOption: postgreSQL
  writeEslTableOnly: false
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_RELEASE_VERSIONS_LIMIT",
					Value: "20",
				},
			},
		},
		{
			Name: "Test default network timeout",
			Values: `
git:
  url:  "testURL"
ingress:
  domainName: "kuberpult-example.com"
db:
  dbOption: "postgreSQL"
  writeEslTableOnly: false
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_NETWORK_TIMEOUT_SECONDS",
					Value: "120",
				},
			},
			ExpectedMissing: []core.EnvVar{},
		},
		{
			Name: "Change Network Timeout",
			Values: `
git:
  url:  "testURL"
ingress:
  domainName: "kuberpult-example.com"
db:
  dbOption: "postgreSQL"
  writeEslTableOnly: false
manifestRepoExport:
  networkTimeoutSeconds: 300
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_NETWORK_TIMEOUT_SECONDS",
					Value: "300",
				},
			},
			ExpectedMissing: []core.EnvVar{},
		},
		{
			Name: "DB ssl mode",
			Values: `
git:
  url:  "testURL"
ingress:
  domainName: "kuberpult-example.com"
db:
  dbOption: "postgreSQL"
  writeEslTableOnly: false
  sslMode: disable
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_DB_SSL_MODE",
					Value: "disable",
				},
			},
			ExpectedMissing: []core.EnvVar{},
		},
		{
			Name: "DB ssl mode activated",
			Values: `
git:
  url:  "testURL"
ingress:
  domainName: "kuberpult-example.com"
db:
  dbOption: "postgreSQL"
  writeEslTableOnly: false
  sslMode: prefer
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_DB_SSL_MODE",
					Value: "prefer",
				},
			},
			ExpectedMissing: []core.EnvVar{},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			testDirName := t.TempDir()
			outputFile, err := runHelm(t, []byte(tc.Values), testDirName)
			if err != nil {
				t.Fatalf(fmt.Sprintf("%v", err))
			}
			if out, err := getDeployments(outputFile); err != nil {
				t.Fatalf(fmt.Sprintf("%v", err))
			} else {
				for index := range out {
					t.Logf("deployment found: %s", index)
				}
				targetDocument := out["kuberpult-manifest-repo-export-service"]
				t.Logf("found document: %v", targetDocument)
				for _, env := range tc.ExpectedEnvs {
					if !CheckForEnvVariable(t, env, &targetDocument) {
						t.Fatalf("%s Environment variable '%s' with value '%s' was expected, but not found.", tc.Name, env.Name, env.Value)
					}
				}
				for _, env := range tc.ExpectedMissing {
					if CheckForEnvVariable(t, env, &targetDocument) {
						t.Fatalf("Found enviroment variable '%s' with value '%s', but was not expecting it.", env.Name, env.Value)
					}
				}

			}
		})
	}
}

func TestHelmChartsKuberpultFrontendEnvVariables(t *testing.T) {
	tcs := []struct {
		Name            string
		Values          string
		ExpectedEnvs    []core.EnvVar
		ExpectedMissing []core.EnvVar
	}{
		{
			Name: "Test out that parsing to front end works as expected with minimal value",
			Values: `
git:
  url: "testURL"
  author:
    name: "SRE"
ingress:
  domainName: "kuberpult-example.com"
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_GIT_AUTHOR_NAME",
					Value: "SRE",
				},
			},
			ExpectedMissing: []core.EnvVar{},
		},
		{
			Name: "Test out dex auth not enabled",
			Values: `
git:
  url: "testURL"
ingress:
  domainName: "kuberpult-example.com"
auth:
  dexAuth:
    enabled: false
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_DEX_ENABLED",
					Value: "false",
				},
			},
			ExpectedMissing: []core.EnvVar{
				{
					Name:  "KUBERPULT_DEX_CLIENT_ID",
					Value: "",
				},
				{
					Name:  "KUBERPULT_DEX_RBAC_POLICY_PATH",
					Value: "",
				},
			},
		},
		{
			Name: "Test out dex auth not enabled",
			Values: `
git:
  url: "testURL"
ingress:
  domainName: "kuberpult-example.com"
auth:
  dexAuth:
    enabled: true
    policy_csv: "testing"

`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_DEX_ENABLED",
					Value: "true",
				},
				{
					Name:  "KUBERPULT_DEX_RBAC_POLICY_PATH",
					Value: "/kuberpult-rbac/policy.csv",
				},
			},
			ExpectedMissing: []core.EnvVar{},
		},
		{
			Name: "Test KUBERPULT_DEX_USE_CLUSTER_INTERNAL_COMMUNICATION",
			Values: `
git:
  url: "testURL"
ingress:
  domainName: "kuberpult-example.com"
auth:
  dexAuth:
    enabled: true
    useClusterInternalCommunicationToDex: true
    policy_csv: "testing"
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_DEX_ENABLED",
					Value: "true",
				},
				{
					Name:  "KUBERPULT_DEX_RBAC_POLICY_PATH",
					Value: "/kuberpult-rbac/policy.csv",
				},
				{
					Name:  "KUBERPULT_DEX_USE_CLUSTER_INTERNAL_COMMUNICATION",
					Value: "true",
				},
			},
			ExpectedMissing: []core.EnvVar{},
		},
		{
			Name: "Test default value for grpcMaxRecvMsgSize",
			Values: `
git:
  url: "testURL"
ingress:
  domainName: "kuberpult-example.com"
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_GRPC_MAX_RECV_MSG_SIZE",
					Value: "4",
				},
			},
			ExpectedMissing: []core.EnvVar{},
		},
		{
			Name: "Test different value for grpcMaxRecvMsgSize",
			Values: `
git:
  url: "testURL"
ingress:
  domainName: "kuberpult-example.com"
frontend:
  grpcMaxRecvMsgSize: 8
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_GRPC_MAX_RECV_MSG_SIZE",
					Value: "8",
				},
			},
			ExpectedMissing: []core.EnvVar{},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			testDirName := t.TempDir()
			outputFile, err := runHelm(t, []byte(tc.Values), testDirName)
			if err != nil {
				t.Fatalf(fmt.Sprintf("%v", err))
			}
			if out, err := getDeployments(outputFile); err != nil {
				t.Fatalf(fmt.Sprintf("%v", err))
			} else {
				targetDocument := out["kuberpult-frontend-service"]
				for _, env := range tc.ExpectedEnvs {
					if !CheckForEnvVariable(t, env, &targetDocument) {
						t.Fatalf("Environment variable '%s' with value '%s' was expected, but not found.", env.Name, env.Value)
					}
				}
				for _, env := range tc.ExpectedMissing {
					if CheckForEnvVariable(t, env, &targetDocument) {
						t.Fatalf("Found enviroment variable '%s' with value '%s', but was not expecting it.", env.Name, env.Value)
					}
				}
			}
		})
	}
}

func TestIngress(t *testing.T) {
	ingressClassGcePrivate := "gce-private"
	tcs := []struct {
		Name            string
		Values          string
		ExpectedIngress *networking.Ingress
	}{
		{
			Name: "Default Ingress not enabled",
			Values: `
git:
  url: "testURL"
`,
			ExpectedIngress: &networking.Ingress{},
		},
		{
			Name: "Ingress enabled",
			Values: `
git:
  url: "testURL"
ingress:
  create: true
  domainName: "kuberpult-example.com"
`,
			ExpectedIngress: &networking.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "networking.k8s.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "kuberpult",
					Annotations: map[string]string{
						"cert-manager.io/acme-challenge-type":            "dns01",
						"cert-manager.io/cluster-issuer":                 "letsencrypt",
						"kubernetes.io/ingress.allow-http":               "false",
						"nginx.ingress.kubernetes.io/proxy-read-timeout": "300",
					},
				},
				Spec: networking.IngressSpec{
					IngressClassName: &ingressClassGcePrivate,
					TLS: []networking.IngressTLS{
						{
							Hosts: []string{
								"kuberpult-example.com",
							},
							SecretName: "kuberpult-tls-secret",
						},
					},
					Rules: []networking.IngressRule{
						{
							Host: "kuberpult-example.com",
							IngressRuleValue: networking.IngressRuleValue{
								HTTP: &networking.HTTPIngressRuleValue{
									Paths: makeAllIngressPaths(),
								},
							},
						},
					},
				},
			},
		},
		{
			Name: "Not Private Ingress",
			Values: `
git:
  url: "testURL"
ingress:
  create: true
  private: false
  domainName: "kuberpult-example.com"
`,
			ExpectedIngress: &networking.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "networking.k8s.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "kuberpult",
					Annotations: map[string]string{
						"cert-manager.io/acme-challenge-type":            "dns01",
						"cert-manager.io/cluster-issuer":                 "letsencrypt",
						"kubernetes.io/ingress.allow-http":               "false",
						"nginx.ingress.kubernetes.io/proxy-read-timeout": "300",
					},
				},
				Spec: networking.IngressSpec{
					TLS: []networking.IngressTLS{
						{
							Hosts: []string{
								"kuberpult-example.com",
							},
							SecretName: "kuberpult-tls-secret",
						},
					},
					Rules: []networking.IngressRule{
						{
							Host: "kuberpult-example.com",
							IngressRuleValue: networking.IngressRuleValue{
								HTTP: &networking.HTTPIngressRuleValue{
									Paths: makeAllIngressPaths(),
								},
							},
						},
					},
				},
			},
		},
		{
			Name: "Override annotations",
			Values: `
git:
  url: "testURL"
ingress:
  create: true
  domainName: "kuberpult-example.com"
  annotations: []
`,
			ExpectedIngress: &networking.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "networking.k8s.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "kuberpult",
					Annotations: map[string]string{
						"cert-manager.io/acme-challenge-type": "dns01",
						"cert-manager.io/cluster-issuer":      "letsencrypt",
					},
				},
				Spec: networking.IngressSpec{
					IngressClassName: &ingressClassGcePrivate,
					TLS: []networking.IngressTLS{
						{
							Hosts: []string{
								"kuberpult-example.com",
							},
							SecretName: "kuberpult-tls-secret",
						},
					},
					Rules: []networking.IngressRule{
						{
							Host: "kuberpult-example.com",
							IngressRuleValue: networking.IngressRuleValue{
								HTTP: &networking.HTTPIngressRuleValue{
									Paths: makeAllIngressPaths(),
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			testDirName := t.TempDir()
			outputFile, err := runHelm(t, []byte(tc.Values), testDirName)
			if err != nil {
				t.Fatalf(fmt.Sprintf("%v", err))
			}

			if out, err := getIngress(outputFile); err != nil {
				t.Fatalf(fmt.Sprintf("%v", err))
			} else {
				{
					// This block is effectively the same as the entire diff below.
					// It's just much easier to read the diff this way.
					if tc.ExpectedIngress != nil && tc.ExpectedIngress.Spec.Rules != nil && out.Spec.Rules != nil {
						for i := range tc.ExpectedIngress.Spec.Rules {
							expectedRule := tc.ExpectedIngress.Spec.Rules[i]
							outRule := out.Spec.Rules[i]
							if diff := cmp.Diff(expectedRule, outRule); diff != "" {
								t.Fatalf("output mismatch (-want, +got):\n%s", diff)
							}
						}
					}
				}

				if diff := cmp.Diff(tc.ExpectedIngress, out); diff != "" {
					t.Fatalf("output mismatch (-want, +got):\n%s", diff)
				}
			}
		})
	}
}

func makeIngressPrefixPath(path string) networking.HTTPIngressPath {
	pathType := networking.PathType("Prefix")
	return makeIngressPath(path, pathType)
}
func makeIngressExactPath(path string) networking.HTTPIngressPath {
	pathType := networking.PathType("Exact")
	return makeIngressPath(path, pathType)
}
func makeIngressImplementationSpecificPath(path string) networking.HTTPIngressPath {
	pathType := networking.PathType("ImplementationSpecific")
	return makeIngressPath(path, pathType)
}

func makeIngressPath(path string, pathType networking.PathType) networking.HTTPIngressPath {
	return networking.HTTPIngressPath{
		PathType: &pathType,
		Path:     path,
		Backend: networking.IngressBackend{
			Service: &networking.IngressServiceBackend{
				Name: "kuberpult-frontend-service",
				Port: networking.ServiceBackendPort{
					Name: "http",
				},
			},
		},
	}
}

func makeAllIngressPaths() []networking.HTTPIngressPath {
	return []networking.HTTPIngressPath{
		makeIngressPrefixPath("/release"),
		makeIngressPrefixPath("/environments"),
		makeIngressPrefixPath("/environment-groups"),
		makeIngressPrefixPath("/api/"),
		makeIngressPrefixPath("/dex"),
		makeIngressPrefixPath("/login"),
		makeIngressImplementationSpecificPath("/ui/*"),
		makeIngressExactPath("/"),
		makeIngressPrefixPath("/static/js/"),
		makeIngressPrefixPath("/static/css/"),
		makeIngressPrefixPath("/favicon.png"),
		makeIngressPrefixPath("/api.v1.OverviewService/"),
		makeIngressPrefixPath("/api.v1.BatchService/"),
		makeIngressPrefixPath("/api.v1.FrontendConfigService/"),
		makeIngressPrefixPath("/api.v1.RolloutService/"),
		makeIngressPrefixPath("/api.v1.GitService/"),
		makeIngressPrefixPath("/api.v1.EnvironmentService/"),
		makeIngressPrefixPath("/api.v1.ReleaseTrainPrognosisService/"),
		makeIngressPrefixPath("/api.v1.EslService/"),
	}
}
