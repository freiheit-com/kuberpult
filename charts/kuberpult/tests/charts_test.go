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

	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	k8sYaml "sigs.k8s.io/yaml" //Needed to properly parse the yaml generated by helm, "gopkg.in/yaml.v3" cannot do it properly
)

func runHelm(t *testing.T, valuesData []byte, dirName string) string {
	testId := strconv.Itoa(rand.Intn(9999))
	tempValuesFile := "vals" + "_" + testId + ".yaml"
	tempValuesFile = dirName + "/" + tempValuesFile
	t.Logf("input file: \n%s\n", valuesData)

	err := os.WriteFile(tempValuesFile, valuesData, 0644)
	if err != nil {
		t.Fatalf("Error writing vals file . %v", err)
	}
	outputFile := "tmp_" + testId + ".tmpl"
	outputFile = dirName + "/" + outputFile

	execOutput, err := exec.Command("sh", "-c", "helm template ./.. --values "+tempValuesFile+" > "+outputFile).CombinedOutput()

	if err != nil {
		t.Fatalf("Error executing helm: Helm output: '%s'\nError: %v\n", string(execOutput), err)
	}

	fileContent, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Error reading file '%s' content: \n%s\n", outputFile, string(fileContent))
		return ""
	}
	t.Logf("output file: \n%s\n", fileContent)

	return outputFile
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
  bootstrap_mode: true
  environment_configs_json: "{}"
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_GIT_URL",
					Value: "testURL",
				},
				{
					Name:  "KUBERPULT_BOOTSTRAP_MODE",
					Value: "true",
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
				{
					Name:  "KUBERPULT_ARGO_CD_SERVER",
					Value: "",
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
  sendWebhooks: true
  server: testServer
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_GIT_URL",
					Value: "testURL",
				},
				{
					Name:  "KUBERPULT_ARGO_CD_SERVER",
					Value: "testServer",
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
cd:
  db:
    dbOption: NO_DB
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_DB_OPTION",
					Value: "NO_DB",
				},
			},
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
			Name: "Database cloudsql enabled 1",
			Values: `
git:
  url: "testURL"
ingress:
  domainName: "kuberpult-example.com"
cd:
  db:
    dbOption: cloudsql
    location: "127.0.0.1"
    dbName: dbName
    dbUser: dbUser
    dbPassword: dbPassword
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_DB_OPTION",
					Value: "cloudsql",
				},
				{
					Name:  "KUBERPULT_DB_LOCATION",
					Value: "127.0.0.1",
				},
				{
					Name:  "KUBERPULT_DB_NAME",
					Value: "dbName",
				},
				{
					Name:  "KUBERPULT_DB_USER_NAME",
					Value: "dbUser",
				},
				{
					Name:  "KUBERPULT_DB_USER_PASSWORD",
					Value: "dbPassword",
				},
			},
			ExpectedMissing: []core.EnvVar{},
		},
		{
			Name: "Database cloudsql enabled 2",
			Values: `
git:
  url: "testURL"
ingress:
  domainName: "kuberpult-example.com"
cd:
  db:
    dbOption: sqlite
    location: /kp/database
    dbName: does
    dbUser: not
    dbPassword: matter
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_DB_OPTION",
					Value: "sqlite",
				},
				{
					Name:  "KUBERPULT_DB_LOCATION",
					Value: "/kp/database",
				},
			},
			ExpectedMissing: []core.EnvVar{
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
ingress:
  domainName: "kuberpult-example.com"
cd:
  releaseVersionsLimit: 15
`,
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_RELEASE_VERSIONS_LIMIT",
					Value: "15",
				},
			},
			ExpectedMissing: []core.EnvVar{},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			testDirName := t.TempDir()
			outputFile := runHelm(t, []byte(tc.Values), testDirName)
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
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			testDirName := t.TempDir()
			outputFile := runHelm(t, []byte(tc.Values), testDirName)
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
