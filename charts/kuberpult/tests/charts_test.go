package main_test

import (
	"bytes"
	"fmt"
	"github.com/freiheit-com/kuberpult/charts/kuberpult/tests/teststructs"
	"gopkg.in/yaml.v3"
	"io"
	"math/rand/v2"
	"os"
	"os/exec"
	"strconv"
	"testing"
)

func readValuesFile() (*teststructs.ValuesDoc, error) {
	data, err := os.ReadFile("../values.yaml")
	if err != nil {
		return nil, fmt.Errorf("../values.yaml: %w", err)
	}
	decoder := yaml.NewDecoder(bytes.NewBufferString(string(data)))

	var d teststructs.ValuesDoc
	if err := decoder.Decode(&d); err != nil {
		return nil, fmt.Errorf("Document decode failed: %w", err)
	}
	return &d, nil
}

func readOutputFile(fileName string, docName string) (*teststructs.OutputDoc, error) {
	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("os.ReadFile: fileName: %w", err)
	}
	decoder := yaml.NewDecoder(bytes.NewBufferString(string(data)))
	for {
		var d teststructs.OutputDoc
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
		Setup           func(t *testing.T, doc *teststructs.ValuesDoc)
		ExpectedEnvs    []teststructs.EnvVar
		ExpectedMissing []teststructs.EnvVar
		checkValues     bool //some values might be more complex than others. For now each test can decide if it wants to check for the values
	}{
		{
			Name: "Initial Test",
			Setup: func(t *testing.T, doc *teststructs.ValuesDoc) {
				doc.Git.Url = "testURL"
			},
			ExpectedEnvs: []teststructs.EnvVar{
				{
					Name:  "KUBERPULT_GIT_URL",
					Value: "testURL",
				},
			},
			ExpectedMissing: []teststructs.EnvVar{
				{
					Name:  "KUBERPULT_ENABLE_METRICS",
					Value: "true",
				},
				{
					Name:  "KUBERPULT_ENABLE_EVENTS",
					Value: "false",
				},
			},
			checkValues: true,
		},
		{
			Name: "Change Git URL",
			Setup: func(t *testing.T, doc *teststructs.ValuesDoc) {
				doc.Git.Url = "checkThisValue"
			},
			ExpectedEnvs: []teststructs.EnvVar{
				{
					Name:  "KUBERPULT_GIT_URL",
					Value: "checkThisValue",
				},
			},
			ExpectedMissing: []teststructs.EnvVar{
				{
					Name:  "KUBERPULT_ENABLE_METRICS",
					Value: "true",
				},
				{
					Name:  "KUBERPULT_ENABLE_EVENTS",
					Value: "false",
				},
			},
			checkValues: true,
		},
		{
			Name: "Argo CD disabled",
			Setup: func(t *testing.T, doc *teststructs.ValuesDoc) {
				doc.Git.Url = "testURL"
				doc.ArgoCd.GenerateFiles = false
			},
			ExpectedEnvs: []teststructs.EnvVar{
				{
					Name:  "KUBERPULT_ARGO_CD_GENERATE_FILES",
					Value: "false",
				},
			},
			ExpectedMissing: []teststructs.EnvVar{
				{
					Name:  "KUBERPULT_ENABLE_METRICS",
					Value: "true",
				},
				{
					Name:  "KUBERPULT_ENABLE_EVENTS",
					Value: "false",
				},
			},
			checkValues: true,
		},
		{
			Name: "Argo CD enabled simple test",
			Setup: func(t *testing.T, doc *teststructs.ValuesDoc) {
				doc.Git.Url = "testURL"
				doc.ArgoCd.GenerateFiles = true
			},
			ExpectedEnvs: []teststructs.EnvVar{
				{
					Name:  "KUBERPULT_ARGO_CD_GENERATE_FILES",
					Value: "true",
				},
			},
			ExpectedMissing: []teststructs.EnvVar{
				{
					Name:  "KUBERPULT_ENABLE_METRICS",
					Value: "true",
				},
				{
					Name:  "KUBERPULT_ENABLE_EVENTS",
					Value: "false",
				},
			},
			checkValues: true,
		},
		{
			Name: "DD Metrics disabled",
			Setup: func(t *testing.T, doc *teststructs.ValuesDoc) {
				doc.Git.Url = "testURL"
				doc.DataDogTracing.Enabled = false
			},
			ExpectedEnvs: []teststructs.EnvVar{},
			ExpectedMissing: []teststructs.EnvVar{
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
			Setup: func(t *testing.T, doc *teststructs.ValuesDoc) {
				doc.Git.Url = "testURL"
				doc.DataDogTracing.Enabled = true
			},
			ExpectedEnvs: []teststructs.EnvVar{
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
			ExpectedMissing: []teststructs.EnvVar{},
			checkValues:     false, //Dont check the values
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

			if doc, err := readValuesFile(); err != nil {
				t.Fatalf(fmt.Sprintf("err: %v", err))
			} else {
				testId := strconv.Itoa(rand.IntN(9999))

				tc.Setup(t, doc)
				if yamlData, err := yaml.Marshal(doc); err != nil {
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
								t.Fatalf("Expected environment variable: %s %s, but did not find it. ", env.Name, env.Value)
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
