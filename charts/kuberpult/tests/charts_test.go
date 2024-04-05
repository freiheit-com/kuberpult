package main_test

import (
	"bytes"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"testing"
)

type ValuesDoc struct { //Auto generated using https://zhwt.github.io/yaml-to-go/. Just post your values.yaml there
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

// Output depends on the input so does it makes sense to auto generate it aswell?
type OutputDoc struct {
	Metadata Metadata
	Kind     string
	Spec     Spec
}
type Metadata struct {
	Name string
}

type Spec struct {
	Replicas int
	Template Template
}

type Template struct {
	Spec TemplateSpec
}

type TemplateSpec struct {
	Containers []Container
}

type Container struct {
	Name string
	Env  []EnvVar
}

type EnvVar struct {
	Name  string
	Value string
}

func (t *OutputDoc) CheckForEnv(target EnvVar, strict bool) bool {
	for _, container := range t.Spec.Template.Spec.Containers {
		for _, env := range container.Env {
			if env.Name == target.Name && (!strict || env.Value == target.Value) {
				return true
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
		return nil, fmt.Errorf("Document decode failed: %w", err)
	}
	return &d, nil
}

//func filter(f string) string {
//	strs := strings.Split(f, "\n")
//	var r []string
//
//	for _, str := range strs {
//		if len(str) > 0 && str[0] != '#' {
//			r = append(r, str)
//		}
//	}
//	return strings.Join(r, "\n")
//}

//	func readOutputFile(fileName string, docName string) (*OutputDoc, error) {
//		data, err := os.ReadFile(fileName)
//		if err != nil {
//			return nil, fmt.Errorf("os.ReadFile: fileName: %w", err)
//		}
//		decoder := yaml.NewDecoder(bytes.NewBufferString(string(data)))
//		for {
//			var d OutputDoc
//			if err := decoder.Decode(&d); err != nil {
//				if err == io.EOF {
//					break
//				}
//				return nil, fmt.Errorf("Document %s decode failed: %w", fileName, err)
//			}
//			deploymentYAML := strings.Split(string(data), "---")
//			targetText := filter(deploymentYAML[5])
//			fmt.Println(targetText)
//			var target apps.Deployment
//			err2 := yaml.Unmarshal([]byte(targetText), &target)
//			if err2 != nil {
//				return nil, err2
//			}
//			fmt.Println(target.Spec.Template.Spec.Containers[0].Resources.Limits.Memory())
//			if d.Metadata.Name == docName && d.Kind == "Deployment" {
//				return &d, nil
//			}
//		}
//		return nil, fmt.Errorf("Could not find document with name: %s in %s.", docName, fileName)
//	}
func readOutputFile(fileName string, docName string) (*OutputDoc, error) {
	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("os.ReadFile: fileName: %w", err)
	}
	decoder := yaml.NewDecoder(bytes.NewBufferString(string(data)))
	for {
		var d OutputDoc
		if err := decoder.Decode(&d); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("Document %s decode failed: %w", fileName, err)
		}

		if d.Metadata.Name == docName && d.Kind == "Deployment" {
			return &d, nil
		}
	}
	return nil, fmt.Errorf("Could not find document with name: %s in %s.", docName, fileName)
}
func TestHelmChartsEnvVariables(t *testing.T) {

	tcs := []struct {
		Name            string
		Setup           func(t *testing.T, values *ValuesDoc)
		ExpectedEnvs    []EnvVar
		ExpectedMissing []EnvVar
		checkValues     bool //some values might be more complex than others. For now each test can decide if it wants to check for the values
	}{
		{
			Name: "Initial Test",
			Setup: func(t *testing.T, values *ValuesDoc) {
				values.Git.URL = "testURL"
			},
			ExpectedEnvs: []EnvVar{
				{
					Name:  "KUBERPULT_GIT_URL",
					Value: "testURL",
				},
			},
			ExpectedMissing: []EnvVar{},
			checkValues:     true,
		},
		{
			Name: "Change Git URL",
			Setup: func(t *testing.T, values *ValuesDoc) {
				values.Git.URL = "checkThisValue"
			},
			ExpectedEnvs: []EnvVar{
				{
					Name:  "KUBERPULT_GIT_URL",
					Value: "checkThisValue",
				},
			},
			ExpectedMissing: []EnvVar{},
			checkValues:     true,
		},
		{
			Name: "Argo CD disabled",
			Setup: func(t *testing.T, values *ValuesDoc) {
				values.Git.URL = "testURL"
				values.Argocd.GenerateFiles = false
			},
			ExpectedEnvs: []EnvVar{
				{
					Name:  "KUBERPULT_ARGO_CD_GENERATE_FILES",
					Value: "false",
				},
				{
					Name:  "KUBERPULT_GIT_URL",
					Value: "testURL",
				},
			},
			ExpectedMissing: []EnvVar{},
			checkValues:     true,
		},
		{
			Name: "Argo CD enabled simple test",
			Setup: func(t *testing.T, values *ValuesDoc) {
				values.Git.URL = "testURL"
				values.Argocd.GenerateFiles = true
			},
			ExpectedEnvs: []EnvVar{
				{
					Name:  "KUBERPULT_ARGO_CD_GENERATE_FILES",
					Value: "true",
				},
				{
					Name:  "KUBERPULT_GIT_URL",
					Value: "testURL",
				},
			},
			ExpectedMissing: []EnvVar{},
			checkValues:     true,
		},
		{
			Name: "DD Metrics disabled",
			Setup: func(t *testing.T, values *ValuesDoc) {
				values.Git.URL = "testURL"
				values.DatadogTracing.Enabled = false
			},
			ExpectedEnvs: []EnvVar{
				{
					Name:  "KUBERPULT_GIT_URL",
					Value: "testURL",
				},
			},
			ExpectedMissing: []EnvVar{
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
			checkValues: false,
		},
		{
			Name: "DD Tracing enabled",
			Setup: func(t *testing.T, values *ValuesDoc) {
				values.Git.URL = "testURL"
				values.DatadogTracing.Enabled = true
			},
			ExpectedEnvs: []EnvVar{
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
			ExpectedMissing: []EnvVar{},
			checkValues:     false, //Dont check the values
		},
		{
			Name: "Two variables involved (web hook disabled)",
			Setup: func(t *testing.T, values *ValuesDoc) {
				values.Git.URL = "testURL"
				values.Argocd.SendWebhooks = false
			},
			ExpectedEnvs: []EnvVar{
				{
					Name:  "KUBERPULT_ARGO_CD_SERVER",
					Value: "",
				},
			},
			ExpectedMissing: []EnvVar{},
			checkValues:     true, //Dont check the values
		},
		{
			Name: "Two variables involved (web hook enabled)",
			Setup: func(t *testing.T, values *ValuesDoc) {
				values.Git.URL = "testURL"
				values.Argocd.SendWebhooks = true
				values.Argocd.Server = "TestServer"
			},
			ExpectedEnvs: []EnvVar{
				{
					Name:  "KUBERPULT_ARGO_CD_SERVER",
					Value: "TestServer",
				},
			},
			ExpectedMissing: []EnvVar{},
			checkValues:     true, //Dont check the values
		},
	}
	dirName := "testfiles"
	defer os.RemoveAll("testfiles")
	if err := os.Mkdir(dirName, os.ModePerm); err != nil {
		t.Fatalf("Could not create test file dir! %v", err)
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {

			if values, err := readValuesFile(); err != nil {
				t.Fatalf(fmt.Sprintf("err: %v", err))
			} else {
				testId := strconv.Itoa(rand.Intn(9999))
				values.Ingress.DomainName = "kuberpult.example.com"
				tc.Setup(t, values)
				if yamlData, err := yaml.Marshal(values); err != nil {
					t.Fatal()
				} else {

					valsFile := "vals" + "_" + testId + ".yaml"

					valsFile = dirName + "/" + valsFile
					err := os.WriteFile(valsFile, yamlData, 0644)
					if err != nil {
						t.Fatalf("Error writing vals file . %v", err)
					}
					outputFile := "tmp_" + testId + ".tmpl"
					outputFile = dirName + "/" + outputFile

					execOutput, err := exec.Command("sh", "-c", "helm template ./.. --values "+valsFile+" > "+outputFile).CombinedOutput()
					if err != nil {
						t.Fatalf("Error executing helm: Helm output: %s\nError: %v\n", string(execOutput), err)
						return
					}
					if out, err := readOutputFile(outputFile, "kuberpult-cd-service"); err != nil {
						t.Fatalf(fmt.Sprintf("%v", err))
					} else {
						for _, env := range tc.ExpectedEnvs {
							if !out.CheckForEnv(env, tc.checkValues) {
								t.Fatalf("Expected environment variable: %s with value %s, but did not find it. ", env.Name, env.Value)
							}
						}
						for _, env := range tc.ExpectedMissing {
							if out.CheckForEnv(env, tc.checkValues) {
								t.Fatalf("Did not environment variable: %s %s, but found it.", env.Name, env.Value)
							}
						}
					}

				}
			}
		})
	}
}
