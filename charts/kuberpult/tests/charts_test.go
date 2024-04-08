/*
This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the Expat(MIT) License as published by
the Free Software Foundation.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
MIT License for more details.

You should have received a copy of the MIT License
along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

Copyright 2023 freiheit.com
*/
package main_test

import (
	"bytes"
	"fmt"
	"gopkg.in/yaml.v3"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	"math/rand"
	"os"
	"os/exec"
	k8sYaml "sigs.k8s.io/yaml" //Needed to properly parse the yaml generated by helm, "gopkg.in/yaml.v3" cannot do it properly
	"strconv"
	"strings"
	"testing"
)

type ValuesDoc struct { //Auto generated using https://zhwt.github.io/yaml-to-go/. In case you made any changes, post the content of values.yaml there and replace this struct
	Git struct {
		URL             interface{} `yaml:"url"`
		WebURL          interface{} `yaml:"webUrl"`
		Branch          string      `yaml:"branch"`
		ManifestRepoURL string      `yaml:"manifestRepoUrl"`
		SourceRepoURL   string      `yaml:"sourceRepoUrl"`
		Author          struct {
			Name  string `yaml:"name"`
			Email string `yaml:"email"`
		} `yaml:"author"`
		NetworkTimeout          string `yaml:"networkTimeout"`
		EnableWritingCommitData bool   `yaml:"enableWritingCommitData"`
		MaximumCommitsPerPush   int    `yaml:"maximumCommitsPerPush"`
	} `yaml:"git"`
	Hub string `yaml:"hub"`
	Log struct {
		Format string `yaml:"format"`
		Level  string `yaml:"level"`
	} `yaml:"log"`
	Cd struct {
		Image         string `yaml:"image"`
		BackendConfig struct {
			Create     bool `yaml:"create"`
			TimeoutSec int  `yaml:"timeoutSec"`
			QueueSize  int  `yaml:"queueSize"`
		} `yaml:"backendConfig"`
		Resources struct {
			Limits struct {
				CPU    int    `yaml:"cpu"`
				Memory string `yaml:"memory"`
			} `yaml:"limits"`
			Requests struct {
				CPU    int    `yaml:"cpu"`
				Memory string `yaml:"memory"`
			} `yaml:"requests"`
		} `yaml:"resources"`
		EnableSqlite bool `yaml:"enableSqlite"`
		Probes       struct {
			Liveness struct {
				PeriodSeconds       int `yaml:"periodSeconds"`
				SuccessThreshold    int `yaml:"successThreshold"`
				TimeoutSeconds      int `yaml:"timeoutSeconds"`
				FailureThreshold    int `yaml:"failureThreshold"`
				InitialDelaySeconds int `yaml:"initialDelaySeconds"`
			} `yaml:"liveness"`
			Readiness struct {
				PeriodSeconds       int `yaml:"periodSeconds"`
				SuccessThreshold    int `yaml:"successThreshold"`
				TimeoutSeconds      int `yaml:"timeoutSeconds"`
				FailureThreshold    int `yaml:"failureThreshold"`
				InitialDelaySeconds int `yaml:"initialDelaySeconds"`
			} `yaml:"readiness"`
		} `yaml:"probes"`
	} `yaml:"cd"`
	Frontend struct {
		Image   string `yaml:"image"`
		Service struct {
			Annotations struct {
			} `yaml:"annotations"`
		} `yaml:"service"`
		Resources struct {
			Limits struct {
				CPU    string `yaml:"cpu"`
				Memory string `yaml:"memory"`
			} `yaml:"limits"`
			Requests struct {
				CPU    string `yaml:"cpu"`
				Memory string `yaml:"memory"`
			} `yaml:"requests"`
		} `yaml:"resources"`
		MaxWaitDuration string `yaml:"maxWaitDuration"`
		BatchClient     struct {
			Timeout string `yaml:"timeout"`
		} `yaml:"batchClient"`
	} `yaml:"frontend"`
	Rollout struct {
		Enabled   bool   `yaml:"enabled"`
		Image     string `yaml:"image"`
		Resources struct {
			Limits struct {
				CPU    string `yaml:"cpu"`
				Memory string `yaml:"memory"`
			} `yaml:"limits"`
			Requests struct {
				CPU    string `yaml:"cpu"`
				Memory string `yaml:"memory"`
			} `yaml:"requests"`
		} `yaml:"resources"`
		PodAnnotations struct {
		} `yaml:"podAnnotations"`
	} `yaml:"rollout"`
	Ingress struct {
		Create      bool `yaml:"create"`
		Annotations struct {
			NginxIngressKubernetesIoProxyReadTimeout int `yaml:"nginx.ingress.kubernetes.io/proxy-read-timeout"`
		} `yaml:"annotations"`
		DomainName interface{} `yaml:"domainName"`
		Iap        struct {
			Enabled    bool        `yaml:"enabled"`
			SecretName interface{} `yaml:"secretName"`
		} `yaml:"iap"`
		TLS struct {
			Host       interface{} `yaml:"host"`
			SecretName string      `yaml:"secretName"`
		} `yaml:"tls"`
	} `yaml:"ingress"`
	SSH struct {
		Identity   string `yaml:"identity"`
		KnownHosts string `yaml:"known_hosts"`
	} `yaml:"ssh"`
	Pgp struct {
		KeyRing interface{} `yaml:"keyRing"`
	} `yaml:"pgp"`
	Argocd struct {
		BaseURL      string `yaml:"baseUrl"`
		Token        string `yaml:"token"`
		Server       string `yaml:"server"`
		Insecure     bool   `yaml:"insecure"`
		SendWebhooks bool   `yaml:"sendWebhooks"`
		Refresh      struct {
			Enabled     bool `yaml:"enabled"`
			Concurrency int  `yaml:"concurrency"`
		} `yaml:"refresh"`
		GenerateFiles bool `yaml:"generateFiles"`
	} `yaml:"argocd"`
	DatadogTracing struct {
		Enabled     bool   `yaml:"enabled"`
		Debugging   bool   `yaml:"debugging"`
		Environment string `yaml:"environment"`
	} `yaml:"datadogTracing"`
	DatadogProfiling struct {
		Enabled bool   `yaml:"enabled"`
		APIKey  string `yaml:"apiKey"`
	} `yaml:"datadogProfiling"`
	DogstatsdMetrics struct {
		Enabled        bool   `yaml:"enabled"`
		EventsEnabled  bool   `yaml:"eventsEnabled"`
		Address        string `yaml:"address"`
		HostSocketPath string `yaml:"hostSocketPath"`
	} `yaml:"dogstatsdMetrics"`
	ImagePullSecrets []interface{} `yaml:"imagePullSecrets"`
	Gke              struct {
		BackendServiceID   string `yaml:"backend_service_id"`
		BackendServiceName string `yaml:"backend_service_name"`
		ProjectNumber      string `yaml:"project_number"`
	} `yaml:"gke"`
	EnvironmentConfigs struct {
		BootstrapMode          bool        `yaml:"bootstrap_mode"`
		EnvironmentConfigsJSON interface{} `yaml:"environment_configs_json"`
	} `yaml:"environment_configs"`
	Auth struct {
		AzureAuth struct {
			Enabled       bool   `yaml:"enabled"`
			CloudInstance string `yaml:"cloudInstance"`
			ClientID      string `yaml:"clientId"`
			TenantID      string `yaml:"tenantId"`
		} `yaml:"azureAuth"`
		DexAuth struct {
			Enabled      bool   `yaml:"enabled"`
			InstallDex   bool   `yaml:"installDex"`
			PolicyCsv    string `yaml:"policy_csv"`
			ClientID     string `yaml:"clientId"`
			ClientSecret string `yaml:"clientSecret"`
			BaseURL      string `yaml:"baseURL"`
			Scopes       string `yaml:"scopes"`
		} `yaml:"dexAuth"`
		API struct {
			EnableDespiteNoAuth bool `yaml:"enableDespiteNoAuth"`
		} `yaml:"api"`
	} `yaml:"auth"`
	Dex struct {
		EnvVars []interface{} `yaml:"envVars"`
		Config  struct {
		} `yaml:"config"`
	} `yaml:"dex"`
	Revolution struct {
		Dora struct {
			Enabled     bool   `yaml:"enabled"`
			URL         string `yaml:"url"`
			Token       string `yaml:"token"`
			Concurrency int    `yaml:"concurrency"`
		} `yaml:"dora"`
	} `yaml:"revolution"`
	ManageArgoApplications struct {
		Enabled bool          `yaml:"enabled"`
		Filter  []interface{} `yaml:"filter"`
	} `yaml:"manageArgoApplications"`
}

func runHelm(t *testing.T, valuesData []byte, dirName string) string {
	testId := strconv.Itoa(rand.Intn(9999))
	tempValuesFile := "vals" + "_" + testId + ".yaml"
	tempValuesFile = dirName + "/" + tempValuesFile

	err := os.WriteFile(tempValuesFile, valuesData, 0644)
	if err != nil {
		t.Fatalf("Error writing vals file . %v", err)
	}
	outputFile := "tmp_" + testId + ".tmpl"
	outputFile = dirName + "/" + outputFile

	//execOutput, err := exec.Command("sh", "-c", "helm version").CombinedOutput()

	execOutput, err := exec.Command("sh", "-c", "helm template ./.. --values "+tempValuesFile+" > "+outputFile).CombinedOutput()
	if err != nil {
		t.Fatalf("Error executing helm: Helm output: '%s'\nError: %v\n", string(execOutput), err)
	}
	return outputFile
}
func CheckForEnvVariable(t *testing.T, target core.EnvVar, strict bool, deployment *apps.Deployment) bool {
	for _, container := range deployment.Spec.Template.Spec.Containers {
		for _, env := range container.Env {
			if env.Name == target.Name {
				if !strict || env.Value == target.Value {
					return true
				} else {
					t.Logf("Found '%s' env variable. Value mismatch: wanted: '%s', got: '%s'.\n", target.Name, target.Value, env.Value)
				}
			}
		}
	}
	return false
}

func readValuesFile() (*ValuesDoc, error) {
	data, err := os.ReadFile("../values.yaml")
	if err != nil {
		return nil, fmt.Errorf("../values.yaml: %w", err)
	}
	decoder := yaml.NewDecoder(bytes.NewBufferString(string(data)))

	var d ValuesDoc
	if err := decoder.Decode(&d); err != nil {
		return nil, fmt.Errorf("document decode failed: %w", err)
	}
	return &d, nil
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
		Setup           func(t *testing.T, values *ValuesDoc) //Runs before each test case. Edit the values you want to test here.
		ExpectedEnvs    []core.EnvVar
		ExpectedMissing []core.EnvVar
		checkValues     bool //some values might be more complex than others. For now each test can decide if it wants to check for the values
	}{
		{
			Name: "Basic Parsing works",
			Setup: func(t *testing.T, values *ValuesDoc) {
				values.Git.URL = "testURL"
				values.Pgp.KeyRing = ""
				values.Ingress.DomainName = "kuberpult-example.com"
				values.EnvironmentConfigs.BootstrapMode = true //makes values.EnvironmentConfigs.EnvironmentConfigsJSON needed
				values.EnvironmentConfigs.EnvironmentConfigsJSON = "{}"
			},
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
			checkValues: true,
		},
		{
			Name: "Change Git URL",
			Setup: func(t *testing.T, values *ValuesDoc) {
				values.Git.URL = "checkThisValue"
			},
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_GIT_URL",
					Value: "checkThisValue",
				},
			},
			ExpectedMissing: []core.EnvVar{},
			checkValues:     true,
		},
		{
			Name: "Argo CD disabled",
			Setup: func(t *testing.T, values *ValuesDoc) {
				values.Git.URL = "testURL"
				values.Argocd.GenerateFiles = false
			},
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
			checkValues:     true,
		},
		{
			Name: "Argo CD enabled simple test",
			Setup: func(t *testing.T, values *ValuesDoc) {
				values.Git.URL = "testURL"
				values.Argocd.GenerateFiles = true
			},
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
			checkValues:     true,
		},
		{
			Name: "DD Metrics disabled",
			Setup: func(t *testing.T, values *ValuesDoc) {
				values.Git.URL = "testURL"
				values.DatadogTracing.Enabled = false
			},
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
			checkValues: false, //Don't check the actual values of the env variables
		},
		{
			Name: "DD Tracing enabled",
			Setup: func(t *testing.T, values *ValuesDoc) {
				values.Git.URL = "testURL"
				values.DatadogTracing.Enabled = true
			},
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
					Value: "",
				},
				{
					Name:  "DD_TRACE_DEBUG",
					Value: "",
				},
			},
			ExpectedMissing: []core.EnvVar{},
			checkValues:     false,
		},
		{
			Name: "Two variables involved web hook disabled",
			Setup: func(t *testing.T, values *ValuesDoc) {
				values.Git.URL = "testURL"
				values.Argocd.SendWebhooks = false
			},
			ExpectedEnvs: []core.EnvVar{
				{
					Name:  "KUBERPULT_ARGO_CD_SERVER",
					Value: "",
				},
			},
			ExpectedMissing: []core.EnvVar{},
			checkValues:     true,
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			testDirName := t.TempDir()
			values, err := readValuesFile()
			if err != nil {
				t.Fatalf(fmt.Sprintf("Error reading values file. Err: %v", err))
			}
			values.Ingress.DomainName = "kuberpult.example.com"
			tc.Setup(t, values)
			yamlValuesData, err := yaml.Marshal(values)
			if err != nil {
				t.Fatalf("Error Marshaling values file. err: %v\n", err)
			}
			outputFile := runHelm(t, yamlValuesData, testDirName)
			if out, err := getDeployments(outputFile); err != nil {
				t.Fatalf(fmt.Sprintf("%v", err))
			} else {
				targetDocument := out["kuberpult-cd-service"]
				for _, env := range tc.ExpectedEnvs {
					if !CheckForEnvVariable(t, env, tc.checkValues, &targetDocument) {
						t.Fatalf("Environment variable '%s' with value '%s' was expected, but not found.", env.Name, env.Value)
					}
				}
				for _, env := range tc.ExpectedMissing {
					if CheckForEnvVariable(t, env, tc.checkValues, &targetDocument) {
						t.Fatalf("Found enviroment variable '%s' with value '%s', but was not expecting it.", env.Name, env.Value)
					}
				}

			}
		})
	}
}
