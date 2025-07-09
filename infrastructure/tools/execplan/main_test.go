package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"gopkg.in/yaml.v2"
)

type File struct {
	FilePath                   string
	Stage                      string
	DependsOn                  []string
	BuildWith                  string
	BuildWithReference         string
	BuildWithHasFile           bool
	BuildWithIsFile            bool
	Builder                    string
	Artifacts                  []string
	RawContent                 string
	PreBuildActions            []string
	PostBuildActions           []string
	IntegrationTestDirectories []string
}

func CreateDirectories(directoryContent []*File, workdir string) error {
	for _, file := range directoryContent {
		if err := os.MkdirAll(filepath.Join(workdir, filepath.Dir(file.FilePath)), 0o700); err != nil {
			return err
		}

		if len(file.RawContent) > 0 {
			err := os.WriteFile(filepath.Join(workdir, file.FilePath), []byte(file.RawContent), 0o600)
			if err != nil {
				return err
			}
			continue
		}

		if len(file.BuildWith) > 0 {
			buildWithPath := filepath.Join(workdir, filepath.Dir(file.FilePath), file.BuildWith)
			if file.BuildWithIsFile {
				if err := os.MkdirAll(filepath.Dir(buildWithPath), 0o700); err != nil {
					return err
				}
			} else {
				if err := os.MkdirAll(buildWithPath, 0o700); err != nil {
					return err
				}
			}
			checkBuildWith, _ := os.Stat(buildWithPath)
			if checkBuildWith != nil && checkBuildWith.IsDir() {
				if err := os.WriteFile(filepath.Join(workdir, filepath.Dir(file.FilePath), file.BuildWith, "Buildfile.yaml"), func() []byte {
					inputYaml := InputYaml{
						Spec: Spec{
							Stage:     file.Stage,
							BuildWith: file.BuildWithReference,
						},
					}
					yamlString, err := yaml.Marshal(inputYaml)
					if err != nil {
						log.Fatal(err)
					}
					return yamlString
				}(), 0o600); err != nil {
					return err
				}
			}

			if len(file.BuildWithReference) > 0 {
				buildWithReferencePath := filepath.Join(workdir, filepath.Dir(file.FilePath), filepath.Dir(file.BuildWithReference))
				if err := os.MkdirAll(buildWithReferencePath, 0o700); err != nil {
					return err
				}
			}
		}

		if err := os.WriteFile(filepath.Join(workdir, file.FilePath), func() []byte {
			inputYaml := InputYaml{
				Spec: Spec{
					Stage:     file.Stage,
					DependsOn: file.DependsOn,
					BuildWith: file.BuildWith,
				},
				AdditionalArtifacts:        file.Artifacts,
				PreBuildActions:            file.PreBuildActions,
				PostBuildActions:           file.PostBuildActions,
				IntegrationTestDirectories: file.IntegrationTestDirectories,
			}
			yamlString, err := yaml.Marshal(inputYaml)
			if err != nil {
				log.Fatal(err)
			}
			return yamlString
		}(), 0o600); err != nil {
			return err
		}
	}
	return nil
}

func StringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

var directoryContent = []*File{
	{
		FilePath: "infrastructure/docker/Buildfile.yaml",
		Builder:  "golang",
		RawContent: `metadata:
  name: ci
  registry: my-favorite.xyz
spec:
  stage: A
additional_artifacts:
  - arti-docker-init
  - inner/arti-docker-build`,
	},
	{
		FilePath: "infrastructure/docker/Makefile",
	},
	{
		FilePath: "infrastructure/docker/Dockerfile",
		RawContent: `
metadata:
  name: ci
  registry: my-favorite.xyz
spec:
  stage: A
additional_artifacts:
  - arti-docker-init
  - inner/arti-docker-build
`,
	},
	{
		FilePath: "infrastructure/docker/Makefile",
	},
	{
		FilePath: "infrastructure/docker/ci/Buildfile.yaml",
		RawContent: `
metadata:
  name: ci
  registry: my-favorite.xyz
spec:
  stage: A
additional_artifacts:
  - arti-docker-init
  - inner/arti-docker-build
`,
	},
	{
		FilePath: "infrastructure/cachedData/Buildfile.yaml",
		RawContent: `
spec:
 stage: B
 buildWith:  infrastructure/docker/
cache:
  cachefiles:
  - '~/.gradle/caches'
  - '~/.gradle/wrapper'
  -  './relativepath'
  hashfiles:
  - '../../**/*.gradle'
  - '../../**/gradle-wrapper.properties'
  - '../../../shared/**/*.gradle'
`,
	},
	{
		FilePath: "infrastructure/kubernetes/k8s.yaml",
	},
	{
		FilePath: "infrastructure/kubernetes/Buildfile.yaml",
		Builder:  "k8s",
		Artifacts: []string{
			"arti-k8s-init",
			"inner/arti-k8s-init",
		},
	},
	{
		FilePath: "infrastructure/make/Makefile",
	},
	{
		FilePath: "infrastructure/make/Buildfile.yaml",
		Builder:  "golang",
		DependsOn: []string{
			"../kubernetes",
		},
		Artifacts: []string{
			"arti-make-build",
			"inner/arti-make-build",
		},
	},
	{
		FilePath:  "infrastructure/tools/A/Buildfile.yaml",
		Stage:     "B",
		BuildWith: dockerDirectoryPath,
		PreBuildActions: []string{
			"setupJava",
			"setupYarn",
		},
		PostBuildActions: []string{
			"failureNotify",
		},
	},
	{
		FilePath: "infrastructure/tools/A/Makefile",
	},
	{
		FilePath: "infrastructure/noBuildFiles/photo.png",
	},
	{
		FilePath: "infrastructure/circleone/circleone",
	},
	{
		FilePath: "infrastructure/circleone/Buildfile.yaml",
		Builder:  "golang",
		DependsOn: []string{
			"../circletwo",
		},
		Artifacts: []string{
			"arti-circleone-build",
			"inner/arti-circleone-build",
		},
	},
	{
		FilePath: "infrastructure/circletwo/circletwo",
	},
	{
		FilePath: "infrastructure/circletwo/Buildfile.yaml",
		Builder:  "golang",
		DependsOn: []string{
			"../circlethree",
		},
	},
	{
		FilePath: "infrastructure/circlethree/circlethree",
	},
	{
		FilePath: "infrastructure/circlethree/Buildfile.yaml",
		Builder:  "golang",
		DependsOn: []string{
			"../circleone",
		},
		Artifacts: []string{
			"arti-circle-three-publish",
			"inner/arti-circle-three-publish",
		},
	},
	{
		FilePath:   "services/serviceA/base/ingress.yaml",
		RawContent: "content",
	},
	{
		FilePath:   "services/serviceA/base/k8s.yaml",
		RawContent: "content",
	},
	{
		FilePath:   "services/serviceA/overlays/production/kustomization.yaml",
		RawContent: "content",
	},
	{
		FilePath:   "services/serviceA/overlays/production/resource.yaml",
		RawContent: "content",
	},
	{
		FilePath:   "services/serviceA/overlays/development/kustomization.yaml",
		RawContent: "content",
	},
	{
		FilePath:   "services/serviceA/overlays/development/resource.yaml",
		RawContent: "content",
	},
	{
		FilePath:   "services/serviceA/overlays/staging/resource.yaml",
		RawContent: "content",
	},
	{
		FilePath:   "services/serviceA/overlays/staging/kustomization.yaml",
		RawContent: "content",
	},
	{
		FilePath:   "services/serviceA/overlays/staging/dummy.jpg",
		RawContent: "content",
	},
	{
		FilePath: "globdependency/singleStar/Buildfile.yaml",
		DependsOn: []string{
			"../../services/*",
		},
	},
	{
		FilePath: "globdependency/globStar/Buildfile.yaml",
		DependsOn: []string{
			"../../services/**/*.yaml",
		},
	},
	{
		FilePath: "integrationtest/serviceA/Buildfile.yaml",
		Stage:    "B",
		Builder:  "golang",
		DependsOn: []string{
			"../make",
		},
		BuildWith: dockerDirectoryPath,
		IntegrationTestDirectories: []string{
			"../",
		},
	},
	{
		FilePath: "integrationtest/serviceB/Buildfile.yaml",
		Stage:    "B",
		Builder:  "golang",
		DependsOn: []string{
			"../make",
		},
		BuildWith:                  dockerDirectoryPath,
		IntegrationTestDirectories: []string{},
	},
	{
		FilePath: "integrationtest/Makefile",
	},
	{
		FilePath: "integrationtest/make",
	},
	{
		FilePath: "integrationtest/Buildfile.yaml",
	},
	{
		FilePath: "empty/builderfile/Buildfile.yaml",
	},
	{
		FilePath: "baseImage/reference/Buildfile.yaml",
		RawContent: `metadata:
  name: baseImage
  registry: my-favorite.xyz
spec:
  stage: A
additional_artifacts:
  - arti-docker-init
  - inner/arti-docker-build
`,
	},
}

func TestMapCreation(t *testing.T) {
	tcs := []struct {
		Name   string
		Input  []string
		Output map[string]*File
	}{
		{
			Name:   "Empty input file",
			Input:  []string{},
			Output: map[string]*File{},
		},
		{
			Name: "Single file build",
			Input: []string{
				"infrastructure/docker/Dockerfile",
			},
			Output: map[string]*File{
				"infrastructure/docker": {
					FilePath: "infrastructure/docker",
					Builder:  "golang",
				},
				"infrastructure/tools/A": {
					FilePath: "infrastructure/tools/A",
				},
				"integrationtest/serviceA": {
					FilePath: "integrationtest/serviceA",
				},
				"integrationtest/serviceB": {
					FilePath: "integrationtest/serviceB",
				},
			},
		},
		{
			Name: "Single file with dependency",
			Input: []string{
				"infrastructure/make/Makefile",
			},
			Output: map[string]*File{
				"infrastructure/make": {
					FilePath: "infrastructure/make",
					Builder:  "golang",
				},
			},
		},
		{
			Name: "Single file with recursive dependency",
			Input: []string{
				"infrastructure/kubernetes/k8s.yaml",
			},
			Output: map[string]*File{
				"infrastructure/kubernetes": {
					FilePath: "infrastructure/kubernetes",
					Builder:  "golang",
				},
				"infrastructure/make": {
					FilePath: "infrastructure/make",
					Builder:  "golang",
				},
			},
		},
		{
			Name: "Single file with no Buildfile",
			Input: []string{
				"infrastructure/noBuildFiles/photo.png",
			},
			Output: map[string]*File{},
		},
		{
			Name: "Multiple files with dependencies",
			Input: []string{
				"infrastructure/noBuildFiles/photo.png",
				"infrastructure/make/Makefile",
				"infrastructure/docker/Dockerfile",
			},
			Output: map[string]*File{
				"infrastructure/docker": {
					FilePath: "infrastructure/docker",
					Builder:  "golang",
				},
				"infrastructure/make": {
					FilePath: "infrastructure/make",
					Builder:  "golang",
				},
				"infrastructure/tools/A": {
					FilePath: "infrastructure/tools/A",
				},
				"integrationtest/serviceA": {
					FilePath: "integrationtest/serviceA",
				},
				"integrationtest/serviceB": {
					FilePath: "integrationtest/serviceB",
				},
			},
		},
		{
			Name: "Circular dependency",
			Input: []string{
				"infrastructure/circleone/circleone",
			},
			Output: map[string]*File{
				"infrastructure/circleone": {
					FilePath: "infrastructure/circleone",
					Builder:  "golang",
				},
				"infrastructure/circletwo": {
					FilePath: "infrastructure/circletwo",
					Builder:  "golang",
				},
				"infrastructure/circlethree": {
					FilePath: "infrastructure/circlethree",
					Builder:  "golang",
				},
			},
		},
		{
			Name: "Star yaml",
			Input: []string{
				"services/serviceA/base/ingress.yaml",
			},
			Output: map[string]*File{
				"globdependency/singleStar": {
					FilePath: "globdependency/singleStar",
				},
				"globdependency/globStar": {
					FilePath: "globdependency/globStar",
				},
			},
		},
		{
			Name: "Star jpg",
			Input: []string{
				"services/serviceA/overlays/staging/dummy.jpg",
			},
			Output: map[string]*File{
				"globdependency/singleStar": {
					FilePath: "globdependency/singleStar",
				},
			},
		},
	}

	workdir := t.TempDir()
	if err := CreateDirectories(directoryContent, workdir); err != nil {
		t.Fatal(err)
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			if err := os.Chdir(workdir); err != nil {
				t.Fatal(err)
			}
			foldersToBuild, _, err := getFoldersToBuild(tc.Input, workdir)
			if err != nil {
				t.Fatal(err)
			}

			if !isEqual(foldersToBuild, tc.Output) {
				t.Fatal("Output expected ", tc.Output, " instead of ", foldersToBuild)
			}
		})
	}
}

func TestJsonOutput(t *testing.T) {
	tcs := []struct {
		Name           string
		Input          []string
		Output         string
		Rootdependency bool
		Error          string
	}{
		{
			Name:  "Empty input file",
			Input: []string{},
			Output: `{
  "stage_a": [],
  "stage_b": [],
  "integration_test": [],
  "publish": [],
  "cleanup": []
}`,
		},
		{
			Name: "Single file build",
			Input: []string{
				"infrastructure/docker/Dockerfile",
			},
			Output: `{
  "stage_a": [
    {
      "container": {
        "image": ""
      },
      "commands": [
        "make -C infrastructure/docker build-pr DOCKER_REGISTRY_URI=my-favorite.xyz IMAGE_NAME=infrastructure/docker ADDITIONAL_IMAGE_TAGS=dire3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
      ],
      "directory": "infrastructure/docker"
    }
  ],
  "stage_b": [
    {
      "container": {
        "image": "my-favorite.xyz/infrastructure/docker:dire3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
      },
      "commands": [
        "make -C integrationtest/serviceA build-pr"
      ],
      "directory": "integrationtest/serviceA"
    },
    {
      "container": {
        "image": "my-favorite.xyz/infrastructure/docker:dire3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
      },
      "commands": [
        "make -C infrastructure/tools/A build-pr"
      ],
      "directory": "infrastructure/tools/A",
      "preBuildActions": [
        "setupJava",
        "setupYarn"
      ],
      "postBuildActions": [
        "failureNotify"
      ]
    },
    {
      "container": {
        "image": "my-favorite.xyz/infrastructure/docker:dire3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
      },
      "commands": [
        "make -C integrationtest/serviceB build-pr"
      ],
      "directory": "integrationtest/serviceB"
    }
  ],
  "integration_test": [
    {
      "container": {
        "image": ""
      },
      "commands": [
        "make -C integrationtest integration-test-pr"
      ],
      "directory": "integrationtest"
    }
  ],
  "publish": [
    {
      "container": {
        "image": ""
      },
      "commands": [
        "make -C infrastructure/docker publish-pr"
      ],
      "directory": "infrastructure/docker"
    },
    {
      "container": {
        "image": ""
      },
      "commands": [
        "make -C integrationtest/serviceA publish-pr"
      ],
      "directory": "integrationtest/serviceA"
    },
    {
      "container": {
        "image": ""
      },
      "commands": [
        "make -C infrastructure/tools/A publish-pr"
      ],
      "directory": "infrastructure/tools/A"
    },
    {
      "container": {
        "image": ""
      },
      "commands": [
        "make -C integrationtest/serviceB publish-pr"
      ],
      "directory": "integrationtest/serviceB"
    }
  ],
  "cleanup": [
    {
      "container": {
        "image": ""
      },
      "commands": [
        "make cleanup-pr"
      ],
      "directory": ""
    }
  ]
}`,
		},
		{
			Name: "Single file build with integration test",
			Input: []string{
				"integrationtest/serviceA/Buildfile.yaml",
			},
			Output: `{
  "stage_a": [],
  "stage_b": [
    {
      "container": {
        "image": "my-favorite.xyz/infrastructure/docker:dire3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
      },
      "commands": [
        "make -C integrationtest/serviceA build-pr"
      ],
      "directory": "integrationtest/serviceA"
    }
  ],
  "integration_test": [
    {
      "container": {
        "image": ""
      },
      "commands": [
        "make -C integrationtest integration-test-pr"
      ],
      "directory": "integrationtest"
    }
  ],
  "publish": [
    {
      "container": {
        "image": ""
      },
      "commands": [
        "make -C integrationtest/serviceA publish-pr"
      ],
      "directory": "integrationtest/serviceA"
    }
  ],
  "cleanup": [
    {
      "container": {
        "image": ""
      },
      "commands": [
        "make cleanup-pr"
      ],
      "directory": ""
    }
  ]
}`,
		},
		{
			Name: "Single file build with invalid path to integration test",
			Input: []string{
				"integrationtest/serviceB/Buildfile.yaml",
			},
			Output: `{
  "stage_a": [],
  "stage_b": [
    {
      "container": {
        "image": "my-favorite.xyz/infrastructure/docker:dire3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
      },
      "commands": [
        "make -C integrationtest/serviceB build-pr"
      ],
      "directory": "integrationtest/serviceB"
    }
  ],
  "integration_test": [],
  "publish": [
    {
      "container": {
        "image": ""
      },
      "commands": [
        "make -C integrationtest/serviceB publish-pr"
      ],
      "directory": "integrationtest/serviceB"
    }
  ],
  "cleanup": [
    {
      "container": {
        "image": ""
      },
      "commands": [
        "make cleanup-pr"
      ],
      "directory": ""
    }
  ]
}`,
		},
		{
			Name: "Cache data",
			Input: []string{
				"infrastructure/cachedData/testfile",
			},
			Output: `{
  "stage_a": [],
  "stage_b": [
    {
      "container": {
        "image": "my-favorite.xyz/infrastructure/docker/:dire3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
      },
      "commands": [
        "make -C infrastructure/cachedData build-pr"
      ],
      "directory": "infrastructure/cachedData",
      "cachefiles": [
        "~/.gradle/caches",
        "~/.gradle/wrapper",
        "infrastructure/cachedData/relativepath"
      ],
      "cacheKey": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
    }
  ],
  "integration_test": [],
  "publish": [
    {
      "container": {
        "image": ""
      },
      "commands": [
        "make -C infrastructure/cachedData publish-pr"
      ],
      "directory": "infrastructure/cachedData"
    }
  ],
  "cleanup": [
    {
      "container": {
        "image": ""
      },
      "commands": [
        "make cleanup-pr"
      ],
      "directory": ""
    }
  ]
}`,
		},
		{
			Name: "Build actions",
			Input: []string{
				"infrastructure/tools/A/testfile",
			},
			Output: `{
  "stage_a": [],
  "stage_b": [
    {
      "container": {
        "image": "my-favorite.xyz/infrastructure/docker:dire3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
      },
      "commands": [
        "make -C infrastructure/tools/A build-pr"
      ],
      "directory": "infrastructure/tools/A",
      "preBuildActions": [
        "setupJava",
        "setupYarn"
      ],
      "postBuildActions": [
        "failureNotify"
      ]
    }
  ],
  "integration_test": [],
  "publish": [
    {
      "container": {
        "image": ""
      },
      "commands": [
        "make -C infrastructure/tools/A publish-pr"
      ],
      "directory": "infrastructure/tools/A"
    }
  ],
  "cleanup": [
    {
      "container": {
        "image": ""
      },
      "commands": [
        "make cleanup-pr"
      ],
      "directory": ""
    }
  ]
}`,
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			workdir := t.TempDir()
			if err := CreateDirectories(directoryContent, workdir); err != nil {
				t.Fatal(err)
			}
			if tc.Rootdependency {
				if len(tc.Input) > 0 && strings.Contains(tc.Input[0], dockerDirectoryPath) {
					if err := CreateDirectories([]*File{
						{
							FilePath: dockerDirectoryPath + "/Buildfile.yaml",
							RawContent: `metadata:
  name: ci
  registry: my-favorite.xyz
spec:
  Stage: A
`,
						},
					}, workdir); err != nil {
						t.Fatal(err)
					}
				} else if len(tc.Input) > 0 {
					if err := CreateDirectories([]*File{
						{
							FilePath:  "Rootdependency/Buildfile.yaml",
							DependsOn: []string{"../"},
							BuildWith: dockerDirectoryPath,
						},
					}, workdir); err != nil {
						t.Fatal(err)
					}
				}
			}

			if err := os.Chdir(workdir); err != nil {
				t.Fatal(err)
			}
			jsonOutput, _, err := getJsonFoldersToBuild(tc.Input, workdir, "pr")
			if err != nil {
				t.Fatal(err)
				return
			}

			if areEqual, _ := AreEqualJSON(jsonOutput, tc.Output); areEqual == false && err != nil {
				fmt.Printf("%v", tc)
				fmt.Println(jsonOutput)
				diff := cmp.Diff(jsonOutput, tc.Output)
				t.Errorf("Output mismatch (-want +got):\n %s", diff)
			}
		})
	}
}

func AreEqualJSON(s1, s2 string) (bool, error) {
	var o1 interface{}
	var o2 interface{}

	var err error
	err = json.Unmarshal([]byte(s1), &o1)
	if err != nil {
		return false, fmt.Errorf("error mashalling JSON object 1")
	}
	err = json.Unmarshal([]byte(s2), &o2)
	if err != nil {
		return false, fmt.Errorf("error mashalling JSON object 2")
	}

	return cmp.Equal(o1, o2), nil
}

func TestJsonErrors(t *testing.T) {
	tcs := []struct {
		Name  string
		Files []*File
		Input []string
		Error string
	}{
		{
			Name: "Invalid buildfile",
			Files: []*File{
				{
					FilePath:   "test/buildfile/Buildfile.yaml",
					RawContent: "invalid:\n-invalid",
				},
			},
			Input: []string{
				"test/buildfile/file",
			},
			Error: `in file "test/buildfile/Buildfile.yaml": yaml: line 3: could not find expected ':'`,
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			workdir := t.TempDir()
			if err := CreateDirectories(tc.Files, workdir); err != nil {
				t.Fatal(err)
			}

			if err := os.Chdir(workdir); err != nil {
				t.Fatal(err)
			}
			_, _, err := getJsonFoldersToBuild(tc.Input, workdir, "pr")

			if diff := cmp.Diff(tc.Error, err.Error()); diff != "" {
				t.Errorf("Error mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGlob(t *testing.T) {
	tcs := []struct {
		Name   string
		Input  string
		Output []string
	}{
		{
			Name:   "Empty input file",
			Input:  "",
			Output: nil,
		},
		{
			Name:  "Single file",
			Input: "services/serviceA/base/ingress.yaml",
			Output: []string{
				"services/serviceA/base/ingress.yaml",
			},
		},
		{
			Name:   "Single file non existing",
			Input:  "services/serviceA/nonExisting/base/ingress.yaml",
			Output: nil,
		},
		{
			Name:  "All yaml files",
			Input: "services/serviceA/**/*.yaml",
			Output: []string{
				"services/serviceA/base/ingress.yaml",
				"services/serviceA/base/k8s.yaml",
				"services/serviceA/overlays/development/kustomization.yaml",
				"services/serviceA/overlays/development/resource.yaml",
				"services/serviceA/overlays/production/kustomization.yaml",
				"services/serviceA/overlays/production/resource.yaml",
				"services/serviceA/overlays/staging/kustomization.yaml",
				"services/serviceA/overlays/staging/resource.yaml",
			},
		},
		{
			Name:  "all Dot files",
			Input: "services/serviceA/**/*.*",
			Output: []string{
				"services/serviceA/base/ingress.yaml",
				"services/serviceA/base/k8s.yaml",
				"services/serviceA/overlays/development/kustomization.yaml",
				"services/serviceA/overlays/development/resource.yaml",
				"services/serviceA/overlays/production/kustomization.yaml",
				"services/serviceA/overlays/production/resource.yaml",
				"services/serviceA/overlays/staging/dummy.jpg",
				"services/serviceA/overlays/staging/kustomization.yaml",
				"services/serviceA/overlays/staging/resource.yaml",
			},
		},
		{
			Name:  "Single folder yaml content",
			Input: "services/serviceA/base/*.yaml",
			Output: []string{
				"services/serviceA/base/ingress.yaml",
				"services/serviceA/base/k8s.yaml",
			},
		},
		{
			Name:  "Single folder all content",
			Input: "services/serviceA/overlays/staging/*",
			Output: []string{
				"services/serviceA/overlays/staging/dummy.jpg",
				"services/serviceA/overlays/staging/kustomization.yaml",
				"services/serviceA/overlays/staging/resource.yaml",
			},
		},
		{
			Name:   "Non existent path",
			Input:  "services/serviceA/overlays/staging/nonExisting/**/*.yaml",
			Output: nil,
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			workdir := t.TempDir()
			if err := CreateDirectories(directoryContent, workdir); err != nil {
				t.Fatal(err)
			}
			if err := os.Chdir(workdir); err != nil {
				t.Fatal(err)
			}
			fileList, err := glob(tc.Input)
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(fileList, tc.Output); diff != "" {
				fmt.Println(fileList)
				t.Errorf("Output mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestHashing(t *testing.T) {
	tcs := []struct {
		Name   string
		Input  []string
		Output string
		Error  string
	}{
		{
			Name:  "Empty input file",
			Input: []string{},
			// Empty hash
			Output: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			Name: "Single file",
			Input: []string{
				"services/serviceA/base/ingress.yaml",
			},
			Output: "797cc61d94104261afd0d15300f91d37103ebe7b185deb11f84ad733a32805cf",
		},
		{
			Name: "Single file non existing",
			Input: []string{
				"services/serviceA/nonExisting/base/ingress.yaml",
			},
			// Empty hash
			Output: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			Name: "All yaml files",
			Input: []string{
				"services/serviceA/**/*.yaml",
			},
			Output: "9992d5add14562e2af53903bc20f925be9d7145e7f898a3336ccd58606232701",
		},
		{
			Name: "Combined to have all yaml files",
			Input: []string{
				"services/serviceA/base/*.yaml",
				"services/serviceA/overlays/**/*.yaml",
			},
			Output: "9992d5add14562e2af53903bc20f925be9d7145e7f898a3336ccd58606232701",
		},
		{
			Name: "all Dot files",
			Input: []string{
				"services/serviceA/**/*.*",
			},
			Output: "9611b1ea7e3a3d07cbc01e131fb4bcab12b9ff5798a182aed41c6e86e486e43b",
		},
		{
			Name: "Single folder yaml content",
			Input: []string{
				"services/serviceA/base/*.yaml",
			},
			Output: "840d27ed4adb0aafc1ecebdc9a366741dc0a4c120d3fd11bbf0ec543920bb545",
		},
		{
			Name: "Single folder all content",
			Input: []string{
				"services/serviceA/overlays/staging/*",
			},
			Output: "e7e663d56d2afbb95c63648ea86b4014905d5f6e4f622a1277c5502f05a4c1cc",
		},
		{
			Name: "Non existent path",
			Input: []string{
				"services/serviceA/overlays/staging/nonExisting/**/*.yaml",
			},
			// Empty hash
			Output: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			Name: "Invalid pattern",
			Input: []string{
				"[]",
			},
			Output: "",
			Error:  "syntax error in pattern",
		},
		{
			Name: "Directory with no sub-directories",
			Input: []string{
				"services/serviceA/overlays/staging",
			},
			Output: "e7e663d56d2afbb95c63648ea86b4014905d5f6e4f622a1277c5502f05a4c1cc",
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			workdir := t.TempDir()
			if err := CreateDirectories(directoryContent, workdir); err != nil {
				t.Fatal(err)
			}
			if err := os.Chdir(workdir); err != nil {
				t.Fatal(err)
			}
			hash, err := getHash(tc.Input)
			if len(tc.Error) > 0 {
				if err == nil {
					t.Fatalf("Expected err %s, nil found", tc.Error)
				}
				if diff := cmp.Diff(tc.Error, err.Error()); diff != "" {
					t.Errorf("Error mismatch (-want +got):\n%s", diff)
				}
			} else if err != nil {
				t.Fatal(err)
			}

			if hash != tc.Output {
				t.Errorf("Output mismatch Expected: %s, Got: %s", tc.Output, hash)
			}
		})
	}
}

func updateFile(filePath, workdir string) error {
	if err := os.MkdirAll(filepath.Join(workdir, filepath.Dir(filePath)), 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(workdir, filePath), []byte("updatedContent"), 0o600)
}

func TestHashUpdate(t *testing.T) {
	tcs := []struct {
		Name          string
		Input         []string
		UpdateFile    string
		Output        string
		UpdatedOutput string
	}{
		{
			Name: "Single file",
			Input: []string{
				"services/serviceA/base/ingress.yaml",
			},
			UpdateFile:    "services/serviceA/base/ingress.yaml",
			Output:        "797cc61d94104261afd0d15300f91d37103ebe7b185deb11f84ad733a32805cf",
			UpdatedOutput: "80b2314d86263b6e685283df15881bfb7040095409725a6e2450a228c3623032",
		},
		{
			Name: "Single file non existing",
			Input: []string{
				"services/serviceA/nonExisting/base/ingress.yaml",
			},
			UpdateFile: "services/serviceA/nonExisting/base/ingress.yaml",
			// Empty hash
			Output:        "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			UpdatedOutput: "c311c6b29c4fcd7aa53ae6a1b2d9c61b2a127605866587f4262e144bfa299356",
		},
		{
			Name: "All yaml files",
			Input: []string{
				"services/serviceA/**/*.yaml",
			},
			UpdateFile:    "services/serviceA/base/newFile.yaml",
			Output:        "9992d5add14562e2af53903bc20f925be9d7145e7f898a3336ccd58606232701",
			UpdatedOutput: "84a9e8b55dde4a0ee98abdd144bac826b5d2ef9ef01c02fb6b22407e60fe0b33",
		},
		{
			Name: "all Dot files",
			Input: []string{
				"services/serviceA/**/*.*",
			},
			UpdateFile:    "services/serviceA/overlays/staging/dummy.jpg",
			Output:        "9611b1ea7e3a3d07cbc01e131fb4bcab12b9ff5798a182aed41c6e86e486e43b",
			UpdatedOutput: "6369adbef52dc58cbc5e186c3ef52fb4a5cd8b9c498a8f2ede2487f0fea9b3d6",
		},
		{
			Name: "Single folder all content",
			Input: []string{
				"services/serviceA/overlays/staging/*",
			},
			UpdateFile:    "services/serviceA/overlays/staging/dummy.jpg",
			Output:        "e7e663d56d2afbb95c63648ea86b4014905d5f6e4f622a1277c5502f05a4c1cc",
			UpdatedOutput: "d8213f8f81c3c790a4a0616d71c300ed4e210beefa2a3bc88768e980aa64bab6",
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			workdir := t.TempDir()
			if err := CreateDirectories(directoryContent, workdir); err != nil {
				t.Fatal(err)
			}

			if err := os.Chdir(workdir); err != nil {
				t.Fatal(err)
			}
			hash, err := getHash(tc.Input)
			if err != nil {
				t.Fatal(err)
			}

			if hash != tc.Output {
				t.Errorf("Output mismatch Expected: %s, Got: %s", tc.Output, hash)
			}

			err = updateFile(tc.UpdateFile, workdir)
			if err != nil {
				t.Fatal(err)
			}

			hash, err = getHash(tc.Input)
			if err != nil {
				t.Fatal(err)
			}
			if hash != tc.UpdatedOutput {
				t.Errorf("Updated Output mismatch Expected: %s, Got: %s", tc.UpdatedOutput, hash)
			}
		})
	}
}

func isEqual(buildFiles BuildFileMap, output map[string]*File) bool {
	if len(buildFiles) != len(output) {
		return false
	}
	for _, file := range output {
		_, ok := buildFiles[file.FilePath]
		if !ok {
			return false
		}
	}
	return true
}

func TestReadBuildFile(t *testing.T) {
	tcs := []struct {
		Name     string
		FilePath string
		Error    string
		Artifact string
	}{
		{
			Name:     "Read non existent file",
			FilePath: "non/existent/file",
			Error:    "no such file or directory",
		},
		{
			Name:     "Read empty file",
			FilePath: "empty/builderfile/Buildfile.yaml",
		},
		{
			Name:     "Read file with content",
			FilePath: "infrastructure/docker/Buildfile.yaml",
			Artifact: "arti-docker-init",
		},
		{
			Name:     "Read invalid yaml file",
			FilePath: "invalid/builderfile/Buildfile.yaml",
			Error:    `in file "invalid/builderfile/Buildfile.yaml": yaml: line 3: could not find expected ':'`,
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			workdir := t.TempDir()
			if err := CreateDirectories(append(directoryContent, &File{
				FilePath: "invalid/builderfile/Buildfile.yaml",
				RawContent: `invalid:
-invalid`,
			}), workdir); err != nil {
				t.Fatal(err)
			}
			if err := os.Chdir(workdir); err != nil {
				t.Fatal(err)
			}
			inputYaml, err := readBuildFile(workdir, tc.FilePath)
			if len(tc.Error) > 0 {
				if err == nil {
					t.Fatalf("expected error %s, but got nil", tc.Error)
				} else if !strings.Contains(err.Error(), tc.Error) {
					// using strings since error type is not guaranteed
					t.Fatalf("expected error %s, but got %s", tc.Error, err.Error())
				}
			} else if err != nil {
				t.Fatalf("expected no error, but got %s", err.Error())
			}
			if len(tc.Artifact) > 0 {
				if len(inputYaml.AdditionalArtifacts) < 1 {
					t.Fatalf("Expected artifact %s, got nil", tc.Artifact)
				} else if inputYaml.AdditionalArtifacts[0] != tc.Artifact {
					t.Fatalf("Expected artifact %s, got %s", tc.Artifact, inputYaml.AdditionalArtifacts[0])
				}
			}
		})
	}
}

func TestFindAllFilesWithName(t *testing.T) {
	tcs := []struct {
		Name    string
		Path    string
		Error   string
		AddLink bool
	}{
		{
			Name:  "Read non existent directory",
			Path:  "non/existent/directory",
			Error: "lstat non/existent/directory: no such file or directory",
		},
		{
			Name:    "Ignores symlink",
			AddLink: true,
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			workdir := t.TempDir()
			if err := CreateDirectories(directoryContent, workdir); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(workdir, "directory"), 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.Chdir(workdir); err != nil {
				t.Fatal(err)
			}
			if tc.AddLink {
				if err := os.Symlink("directory", "Buildfile.yaml"); err != nil {
					t.Fatal(err)
				}
			}
			if tc.Path == "" {
				tc.Path = workdir
			}
			_, err := findAllFilesWithName("Buildfile.yaml", tc.Path)
			if len(tc.Error) > 0 {
				if err == nil {
					t.Fatalf("expected error %s, but got nil", tc.Error)
				} else if !strings.Contains(err.Error(), tc.Error) {
					t.Fatalf("expected error %s, but got %s", tc.Error, err.Error())
				}
			} else if err != nil {
				t.Fatalf("expected no error, but got %s", err.Error())
			}
		})
	}
}

func TestGetTrigger(t *testing.T) {
	tcs := []struct {
		Name           string
		Args           []string
		OutputFilename string
		ErrorExpected  string
	}{
		{
			Name:          "No arguments provided",
			Args:          []string{"noop"},
			ErrorExpected: "Please provide one of { pr, main } as command line argument ",
		},
		{
			Name:          "Invalid argument provided",
			Args:          []string{"noop", "invalid", "test"},
			ErrorExpected: "Please provide one of { pr, main } as command line argument ",
		},
		{
			Name:          "Pr argument",
			Args:          []string{"noop", "pr"},
			ErrorExpected: "",
		},
		{
			Name:          "Main argument",
			Args:          []string{"noop", "main"},
			ErrorExpected: "",
		},
		{
			Name: "Valid argument provided",
			Args: []string{"noop", "pr"},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			_, err := getTrigger(tc.Args)
			if tc.ErrorExpected != "" {
				triggerError := errors.New(tc.ErrorExpected)
				if errors.Is(err, triggerError) {
					t.Fatalf("expected error %s, but got %s", triggerError.Error(), err.Error())
				}
			} else if err != nil {
				t.Fatalf("expected no error, but got %s", err.Error())
			}
		})
	}
}

func TestReadInputLines(t *testing.T) {
	tcs := []struct {
		Name   string
		Input  string
		Output []string
	}{
		{
			Name:  "Empty input",
			Input: "",
		}, {
			Name:  "Files with space in it",
			Input: `file name with space`,
			Output: []string{
				"file name with space",
			},
		}, {
			Name: "Multiple files, some with spaces",
			Input: `file name with space
secondfile
path/to/third/file`,
			Output: []string{
				"file name with space",
				"secondfile",
				"path/to/third/file",
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			reader := strings.NewReader(tc.Input)
			changedFiles, err := readInputLines(reader)
			if err != nil {
				t.Fatalf("expected no error, but got %s", err.Error())
			}
			if diff := cmp.Diff(changedFiles, tc.Output); diff != "" {
				t.Errorf("Output mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGetDependencies(t *testing.T) {
	tcs := []struct {
		Name   string
		Input  []string
		Output []string
	}{
		{
			Name:  "Empty input",
			Input: []string{},
		}, {
			Name: "Regular files",
			Input: []string{
				"infrastructure/docker/Makefile",
				"infrastructure/docker/Dockerfile",
			}, Output: []string{
				"infrastructure/docker/Makefile",
				"infrastructure/docker/Dockerfile",
			},
		}, {
			Name: "Glob files",
			Input: []string{
				"infrastructure/docker/Makefile",
				"**/*.yaml",
			}, Output: []string{
				"infrastructure/docker/Makefile",
				"baseImage/reference/Buildfile.yaml",
				"empty/builderfile/Buildfile.yaml",
				"globdependency/globStar/Buildfile.yaml",
				"globdependency/singleStar/Buildfile.yaml",
				"infrastructure/cachedData/Buildfile.yaml",
				"infrastructure/circleone/Buildfile.yaml",
				"infrastructure/circlethree/Buildfile.yaml",
				"infrastructure/circletwo/Buildfile.yaml",
				"infrastructure/docker/Buildfile.yaml",
				"infrastructure/docker/ci/Buildfile.yaml",
				"infrastructure/kubernetes/Buildfile.yaml",
				"infrastructure/kubernetes/k8s.yaml",
				"infrastructure/make/Buildfile.yaml",
				"infrastructure/tools/A/Buildfile.yaml",
				"infrastructure/tools/A/infrastructure/docker/Buildfile.yaml",
				"integrationtest/Buildfile.yaml",
				"integrationtest/serviceA/Buildfile.yaml",
				"integrationtest/serviceA/infrastructure/docker/Buildfile.yaml",
				"integrationtest/serviceB/Buildfile.yaml",
				"integrationtest/serviceB/infrastructure/docker/Buildfile.yaml",
				"services/serviceA/base/ingress.yaml",
				"services/serviceA/base/k8s.yaml",
				"services/serviceA/overlays/development/kustomization.yaml",
				"services/serviceA/overlays/development/resource.yaml",
				"services/serviceA/overlays/production/kustomization.yaml",
				"services/serviceA/overlays/production/resource.yaml",
				"services/serviceA/overlays/staging/kustomization.yaml",
				"services/serviceA/overlays/staging/resource.yaml",
			},
		}, {
			Name: "Partial Glob files",
			Input: []string{
				"infrastructure/kubernetes/**/*.yaml",
			}, Output: []string{
				"infrastructure/kubernetes/Buildfile.yaml",
				"infrastructure/kubernetes/k8s.yaml",
			},
		}, {
			Name: "Partial Glob files",
			Input: []string{
				"infrastructure/kubernetes/**/*.yaml",
			}, Output: []string{
				"infrastructure/kubernetes/Buildfile.yaml",
				"infrastructure/kubernetes/k8s.yaml",
			},
		}, {
			Name: "Wildcard files",
			Input: []string{
				"infrastructure/kubernetes/*",
			}, Output: []string{
				"infrastructure/kubernetes/Buildfile.yaml",
				"infrastructure/kubernetes/k8s.yaml",
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			workdir := t.TempDir()
			if err := CreateDirectories(directoryContent, workdir); err != nil {
				t.Fatal(err)
			}
			if err := os.Chdir(workdir); err != nil {
				t.Fatal(err)
			}
			output := applyGlobToPaths(tc.Input, workdir, workdir)
			for index, file := range output {
				output[index] = strings.Replace(file, workdir+"/", "", -1)
			}
			if diff := cmp.Diff(output, tc.Output); diff != "" {
				t.Errorf("Output mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGetAbsolutePath(t *testing.T) {
	tcs := []struct {
		Name     string
		Input    string
		BasePath string
		RootPath string
		Output   string
	}{
		{
			Name:     "Empty input",
			Input:    "",
			BasePath: "basePath",
			RootPath: "rootPath",
			Output:   "rootPath",
		}, {
			Name:     "just base",
			Input:    "./",
			BasePath: "basePath",
			RootPath: "rootPath",
			Output:   "basePath",
		}, {
			Name:     "Regular path",
			Input:    "path/to/file",
			BasePath: "basePath",
			RootPath: "rootPath",
			Output:   "rootPath/path/to/file",
		}, {
			Name:     "path with wildcard",
			Input:    "./path/to/file*",
			BasePath: "basePath",
			RootPath: "rootPath",
			Output:   "basePath/path/to/file*",
		}, {
			Name:     "path with glob",
			Input:    "../path/to/**/file*",
			BasePath: "base/path",
			RootPath: "rootPath",
			Output:   "base/path/to/**/file*",
		}, {
			Name:     "path with wildcard from root",
			Input:    "**/*",
			BasePath: "basePath",
			RootPath: "rootPath",
			Output:   "rootPath/**/*",
		}, {
			Name:     "path with wildcard from base",
			Input:    "./**/*",
			BasePath: "basePath",
			RootPath: "rootPath",
			Output:   "basePath/**/*",
		}, {
			Name:     "Absolute path from home",
			Input:    "~/path/from/home",
			BasePath: "basePath",
			RootPath: "rootPath",
			Output:   "~/path/from/home",
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			output := getAbsolutePath(tc.Input, tc.BasePath, tc.RootPath)
			if diff := cmp.Diff(output, tc.Output); diff != "" {
				t.Errorf("Output mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGetAllBuildFiles(t *testing.T) {
	tcs := []struct {
		Name     string
		Input    []*File
		BasePath string
		Output   []string
		Error    string
	}{
		{
			Name: "No buildfiles",
			Input: []*File{
				{
					FilePath: "test/file",
				},
			},
			Output: []string{},
		},
		{
			Name: "Single buildfile",
			Input: []*File{
				{
					FilePath: "test/buildfile/Buildfile.yaml",
				},
			},
			Output: []string{
				"test/buildfile",
			},
		},
		{
			Name: "Invalid buildfile",
			Input: []*File{
				{
					FilePath:   "test/buildfile/Buildfile.yaml",
					RawContent: "invalid:\n-invalid",
				},
			},
			Error:  `in file "test/buildfile/Buildfile.yaml": yaml: line 3: could not find expected ':'`,
			Output: []string{},
		},
		{
			Name: "Invalid path",
			Input: []*File{
				{
					FilePath: "test/buildfile/Buildfile",
				},
			},
			Output:   []string{},
			BasePath: "non/existent/path",
			Error:    `lstat non/existent/path: no such file or directory`,
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			workdir := t.TempDir()
			if err := CreateDirectories(tc.Input, workdir); err != nil {
				t.Fatal(err)
			}
			basePath := tc.BasePath
			if basePath == "" {
				basePath = workdir
			}
			buildFileMap, err := getAllBuildFiles(basePath)
			if len(tc.Error) > 0 {
				if err == nil {
					t.Fatalf("expected error %s, but got nil", tc.Error)
				} else if !strings.Contains(err.Error(), tc.Error) {
					t.Fatalf("expected error %s, but got %s", tc.Error, err.Error())
				}
			} else if err != nil {
				t.Fatalf("expected no error, but got %s", err.Error())
			}
			// Convert to directories
			directories := []string{}
			for directory := range buildFileMap {
				directories = append(directories, directory)
			}

			if diff := cmp.Diff(directories, tc.Output); diff != "" {
				t.Errorf("Output mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestValidateServiceBuildfileDependsOn(t *testing.T) {
	tcs := []struct {
		Name          string
		Input         []*File
		BuildFileName string
		Error         string
	}{
		{
			Name: "Buildfile with no dependsOn",
			Input: []*File{
				{
					FilePath:  "test/Buildfile.yaml",
					BuildWith: "../testReference",
				},
			},
			BuildFileName: "test",
		},
		{
			Name: "Buildfile with dependsOn and a dependency for a docker image",
			Input: []*File{
				{
					FilePath:  "test/Buildfile.yaml",
					DependsOn: []string{"../infrastructure/docker/ci"},
					BuildWith: "../testReference",
				},
			},
			BuildFileName: "test",
			Error:         "\"test\": service buildFile cannot have a dependency on docker images",
		},
		{
			Name: "Buildfile with dependsOn but no dependency on docker images",
			Input: []*File{
				{
					FilePath: "infrastructure/terraform",
				},
				{
					FilePath: "infrastructure/make/go",
				},
				{
					FilePath:  "test/Buildfile.yaml",
					DependsOn: []string{"../infrastructure/make/go", "../infrastructure/terraform"},
					BuildWith: "../testReference",
				},
			},
			BuildFileName: "test",
		},
		{
			Name: "Buildfile with dependsOn and one dependency for a docker image",
			Input: []*File{
				{
					FilePath:  "test/Buildfile.yaml",
					DependsOn: []string{"../infrastructure/docker/ci", "../infrastructure/terraform"},
					BuildWith: "../testReference",
				},
			},
			BuildFileName: "test",
			Error:         "\"test\": service buildFile cannot have a dependency on docker images",
		},
		{
			Name: "Buildfile with dependsOn but it is empty",
			Input: []*File{
				{
					FilePath:  "test/Buildfile.yaml",
					DependsOn: []string{"./"},
					BuildWith: "../testReference",
				},
			},
			BuildFileName: "test",
			Error:         "\"test\": service buildFile cannot depend on itself",
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			workdir := t.TempDir()
			if err := CreateDirectories(tc.Input, workdir); err != nil {
				t.Fatal(err)
			}
			basePath := workdir
			buildFileMap, err := getAllBuildFiles(basePath)
			if err != nil {
				t.Fatal(err)
			}
			buildFile := buildFileMap[tc.BuildFileName]
			err = validateServiceBuildfileDependsOn(buildFile, basePath, workdir)
			if len(tc.Error) > 0 {
				if err == nil {
					t.Fatalf("expected error %s, but got nil", tc.Error)
				} else if !strings.Contains(err.Error(), tc.Error) {
					t.Fatalf("expected error %s, but got %s", tc.Error, err.Error())
				}
			} else if err != nil {
				t.Fatalf("expected no error, but got %s", err.Error())
			}
		})
	}
}

func TestGetStageACommand(t *testing.T) {
	tcs := []struct {
		Name          string
		Input         []*File
		BasePath      string
		BuildFileName string
		Output        []string
		Error         bool
	}{
		{
			Name: "Command matches the expected value without a base image",
			Input: []*File{
				{
					FilePath: "test/Buildfile.yaml",
					RawContent: `metadata:
  name: my-super-image
  registry: my-favorite.xyz
`,
				},
			},
			BuildFileName: "test",
			Output: []string{
				"make -C test build-pr DOCKER_REGISTRY_URI=my-favorite.xyz ADDITIONAL_IMAGE_TAGS=dirsomehash",
			},
		},
		{
			Name: "Command does not match the expected value",
			Input: []*File{
				{
					FilePath: "test/Buildfile.yaml",
					RawContent: `metadata:
  name: ci
  registry: my-favorite.xyz
`,
				},
			},
			BuildFileName: "test",
			Output: []string{
				"make build",
			},
			Error: true,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			workdir := t.TempDir()
			if err := CreateDirectories(tc.Input, workdir); err != nil {
				t.Fatal(err)
			}
			basePath := tc.BasePath
			if basePath == "" {
				basePath = workdir + "/" + tc.BuildFileName
			}
			buildFileMap, _ := getAllBuildFiles(workdir)

			buildFile := buildFileMap[tc.BuildFileName]

			imageTargets, err := getImageTargets(tc.BuildFileName, basePath, "somehash")
			if err != nil {
				t.Fatal(err)
			}

			containerCommands := getStageACommand(buildFile, "pr", imageTargets)
			if !StringSlicesEqual(containerCommands.Commands, tc.Output) {
				if !tc.Error {
					t.Fatalf("expected command to be %s, but got %s", tc.Output, containerCommands.Commands)
				}
			}
		})
	}
}

func TestGetStageAAndStageBBuildFiles(t *testing.T) {
	tcs := []struct {
		Name         string
		Input        []*File
		Error        string
		OutputStageA BuildFileMap
		OutputStageB BuildFileMap
	}{
		{
			Name: "All the images have builders (Valid Configuration)",
			Input: []*File{
				{
					FilePath: dockerDirectoryPath + "/Buildfile.yaml",
					RawContent: `metadata:
  name: ci
  registry: my-favorite.xyz
spec:
  stage: A
`,
				},
				{
					FilePath: "test/Buildfile.yaml",
					RawContent: `
spec:
  stage: B
  buildWith:  infrastructure/docker
`,
				},
				{
					FilePath: "dummy/Buildfile.yaml",
					RawContent: `
spec:
  stage: B
  buildWith:  infrastructure/docker
`,
				},
			},
			OutputStageA: BuildFileMap{
				dockerDirectoryPath: {
					Directory: dockerDirectoryPath,
					Stage:     "A",
				},
			},
			OutputStageB: BuildFileMap{
				"dummy": {
					Directory:        "dummy",
					Stage:            "B",
					BuildWith:        dockerDirectoryPath,
					PreBuildActions:  nil,
					PostBuildActions: nil,
				},
				"test": {
					Directory:        "test",
					Stage:            "B",
					BuildWith:        dockerDirectoryPath,
					PreBuildActions:  nil,
					PostBuildActions: nil,
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			workdir := t.TempDir()
			if err := CreateDirectories(tc.Input, workdir); err != nil {
				t.Fatal(err)
			}
			buildFileMap, err := getAllBuildFiles(workdir)
			if err != nil {
				t.Fatal(err)
			}
			filesA, filesB := getStageAAndStageBBuildFiles(buildFileMap)
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(tc.OutputStageA, filesA); diff != "" {
				t.Errorf("Output Stage A mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.OutputStageB, filesB); diff != "" {
				t.Errorf("Output Stage B mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPrintFileTriggersForBuildfiles(t *testing.T) {
	tcs := []struct {
		Name         string
		Input        []*File
		ChangedFiles []string
		Error        string
		Output       string
	}{
		{
			Name:         "No changes or dependencies are detected",
			ChangedFiles: []string{""},
			Input: []*File{
				{
					FilePath: dockerDirectoryPath + "/Buildfile.yaml",
					RawContent: `metadata:
  name: ci
  registry: my-favorite.xyz
spec:
  stage: A
`,
				},
				{
					FilePath: "test/Buildfile.yaml",
					RawContent: `
spec:
  stage: B
  buildWith:  infrastructure/docker
`,
				},
				{
					FilePath: "dummy/Buildfile.yaml",
					RawContent: `
spec:
  stage: B
  buildWith:  infrastructure/docker
`,
				},
			},
			Output: "========== File Triggers For Buildfiles =============\n=====================================================\n",
		},
		{
			Name:         "There is a direct change in a file",
			ChangedFiles: []string{"test/Buildfile.yaml"},
			Input: []*File{
				{
					FilePath: dockerDirectoryPath + "/Buildfile.yaml",
					RawContent: `metadata:
  name: ci
  registry: my-favorite.xyz
spec:
  stage: A
`,
				},
				{
					FilePath: "test/Buildfile.yaml",
					RawContent: `
spec:
  stage: B
  buildWith:  infrastructure/docker
`,
				},
				{
					FilePath: "dummy/Buildfile.yaml",
					RawContent: `
spec:
  stage: B
  buildWith:  infrastructure/docker
`,
				},
			},
			Output: "========== File Triggers For Buildfiles =============\ntest -> test/Buildfile.yaml\n=====================================================\n",
		},
		{
			Name:         "There is a dependency and a direct change in a file",
			ChangedFiles: []string{"test/Buildfile.yaml"},
			Input: []*File{
				{
					FilePath: dockerDirectoryPath + "/Buildfile.yaml",
					RawContent: `metadata:
  name: ci
  registry: my-favorite.xyz
spec:
  stage: A
`,
				},
				{
					FilePath: "test/Buildfile.yaml",
					RawContent: `
spec:
  stage: B
  buildWith:  infrastructure/docker
`,
				},
				{
					FilePath: "dummy/Buildfile.yaml",
					RawContent: `
spec:
  stage: B
  dependsOn:
  - ../test
  buildWith:  infrastructure/docker
`,
				},
			},
			Output: "========== File Triggers For Buildfiles =============\ndummy -> test [dependency]\ntest -> test/Buildfile.yaml\n=====================================================\n",
		},
		{
			Name:         "There is a dependency and a direct change in a base image",
			ChangedFiles: []string{"infrastructure/docker/Buildfile.yaml"},
			Input: []*File{
				{
					FilePath: dockerDirectoryPath + "/Buildfile.yaml",
					RawContent: `metadata:
  name: ci
  registry: my-favorite.xyz
spec:
  stage: A
`,
				},
				{
					FilePath: "test/Buildfile.yaml",
					RawContent: `
spec:
  stage: B
  buildWith:  infrastructure/docker
`,
				},
				{
					FilePath: "dummy/Buildfile.yaml",
					RawContent: `
spec:
  stage: B
  buildWith:  infrastructure/docker
`,
				},
			},
			Output: "========== File Triggers For Buildfiles =============\ndummy -> infrastructure/docker [dependency]\ninfrastructure/docker -> infrastructure/docker/Buildfile.yaml\ntest -> infrastructure/docker [dependency]\n=====================================================\n",
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			workdir := t.TempDir()
			if err := CreateDirectories(tc.Input, workdir); err != nil {
				t.Fatal(err)
			}

			foldersToBuild, _, err := getFoldersToBuild(tc.ChangedFiles, workdir)
			if err != nil {
				t.Fatal(err)
			}
			triggerInfo := printFileTriggersForBuildfiles(foldersToBuild)
			if diff := cmp.Diff(triggerInfo, tc.Output); diff != "" {
				t.Errorf("Error mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
