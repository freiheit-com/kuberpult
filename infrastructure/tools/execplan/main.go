package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yargevad/filepathx"
	"gopkg.in/yaml.v2"
)

func main() {
	useOutputFile := flag.String("o", "", "use argument as output file")
	flag.Parse()

	trigger, err := getTrigger(os.Args)
	if err != nil {
		log.Fatal(err)
	}

	// Read the stdin which will be a list of files, equivalent to the output of git diff --name-only
	changedFiles, err := readInputLines(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}

	output, fileTriggerInfo, err := getJsonFoldersToBuild(changedFiles, rootPath, trigger)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Fprintln(os.Stderr, fileTriggerInfo)
	if *useOutputFile != "" {
		err = os.WriteFile(*useOutputFile, []byte(fileTriggerInfo+output), 0o600)
		if err != nil {
			log.Fatal(err)
		}
	}
	fmt.Println(output)
}

const (
	buildFileName       = "Buildfile.yaml"
	rootPath            = "."
	dockerDirectoryPath = "infrastructure/docker"
)

type Spec struct {
	Stage      string   `yaml:"stage"`
	DependsOn  []string `yaml:"dependsOn,omitempty"`
	BuildWith  string   `yaml:"buildWith"`
	BuildImage string   `yaml:"buildImage"`
}

type (
	Actions                    []string
	IntegrationTestDirectories []string
)

type Cache struct {
	Cachefiles []string `yaml:"cachefiles"`
	Hashfiles  []string `yaml:"hashfiles"`
}

type Metadata struct {
	Name     string `yaml:"name"`
	Registry string `yaml:"registry"`
}

type InputYaml struct {
	Spec                       Spec                       `yaml:"spec"`
	Metadata                   Metadata                   `yaml:"metadata"`
	AdditionalArtifacts        Artifacts                  `yaml:"additional_artifacts"`
	Cache                      Cache                      `yaml:"cache"`
	PreBuildActions            Actions                    `yaml:"pre_build_actions"`
	PostBuildActions           Actions                    `yaml:"post_build_actions"`
	IntegrationTestDirectories IntegrationTestDirectories `yaml:"integration_tests"`
}

type Artifacts []string

type BuildFile struct {
	Directory                  string
	Stage                      string
	DependsOn                  []string
	BuildWith                  string
	BuildImage                 string
	FileTrigger                []string
	AdditionalArtifacts        Artifacts
	Cache                      Cache
	PreBuildActions            Actions
	PostBuildActions           Actions
	IntegrationTestDirectories IntegrationTestDirectories
}

type (
	BuildFileMap      map[string]*BuildFile
	ContainerCommands struct {
		Container        Container `json:"container"`
		Commands         []string  `json:"commands"`
		Artifacts        Artifacts `json:"artifacts,omitempty"`
		Directory        string    `json:"directory"`
		Cachefiles       []string  `json:"cachefiles,omitempty"`
		Cachekey         string    `json:"cacheKey,omitempty"`
		PreBuildActions  Actions   `json:"preBuildActions,omitempty"`
		PostBuildActions Actions   `json:"postBuildActions,omitempty"`
	}
)

type ExecutionPlan struct {
	StageA          []ContainerCommands `json:"stage_a"`
	StageB          []ContainerCommands `json:"stage_b"`
	IntegrationTest []ContainerCommands `json:"integration_test"`
	PublishA        []ContainerCommands `json:"publish_a"`
	PublishB        []ContainerCommands `json:"publish_b"`
	Cleanup         []ContainerCommands `json:"cleanup"`
}

type Container struct {
	Image string `json:"image"`
}

func getTrigger(args []string) (string, error) {
	if len(args) < 2 {
		return "", errors.New("please provide one of { pr, main } as command line argument ")
	}
	trigger := args[1]
	if !(trigger == "pr" || trigger == "main") {
		return "", errors.New("please provide one of { pr, main } as command line argument ")
	}

	return trigger, nil
}

func readBuildFile(root, path string) (*InputYaml, error) {
	buf, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		return nil, err
	}

	c := &InputYaml{}
	err = yaml.Unmarshal(buf, c)
	if err != nil {
		return nil, fmt.Errorf("in file %q: %w", path, err)
	}
	return c, nil
}

func findAllFilesWithName(filename, root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(filePath string, directory fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if filename == directory.Name() && !directory.IsDir() {
			// Implementation never returns error
			fileInfo, _ := directory.Info()
			if fileInfo.Mode().IsRegular() {
				// Walkdir of root cannot find a regular file not part of root. error will not be triggered
				relativePath, _ := filepath.Rel(root, filePath)
				files = append(files, relativePath)
			}
		}
		return nil
	})
	return files, err
}

func applyGlobToPaths(paths []string, basePath, rootPath string) []string {
	var dependencies []string
	for _, path := range paths {
		if strings.Contains(path, "*") {
			// Treat as glob and add all paths matching the glob into dependency list
			// Input is sanitized by getAbsolutePath and never throws error
			files, _ := glob(getAbsolutePath(path, basePath, rootPath))
			dependencies = append(dependencies, files...)
		} else {
			// Treat as normal path
			dependencies = append(dependencies, getAbsolutePath(path, basePath, rootPath))
		}
	}
	return dependencies
}

func getAbsolutePath(path, basePath, rootPath string) string {
	if strings.HasPrefix(path, "~") {
		return path
	}
	var absolutePath string
	if !strings.HasPrefix(path, "./") && !strings.HasPrefix(path, "../") {
		absolutePath = filepath.Join(rootPath, path)
	} else {
		absolutePath = filepath.Join(basePath, path)
	}
	if strings.HasPrefix(absolutePath, "*") {
		absolutePath = "./" + absolutePath
	}
	return absolutePath
}

func convertToAbsolutePaths(paths []string, basePath, rootPath string) []string {
	var absolutePaths []string
	for _, path := range paths {
		absolutePaths = append(absolutePaths, getAbsolutePath(path, basePath, rootPath))
	}
	return absolutePaths
}

// Go through file system and find all instances of Buildfile, find its Builder and DependsOn values
func getAllBuildFiles(rootPath string) (BuildFileMap, error) {
	allBuildDirs := make(BuildFileMap)
	allBuildFilePaths, err := findAllFilesWithName(buildFileName, rootPath)
	if err != nil {
		return allBuildDirs, err
	}
	for _, buildFilePath := range allBuildFilePaths {
		content, err := readBuildFile(rootPath, buildFilePath)
		if err != nil {
			return nil, err
		}
		if err != nil {
			return allBuildDirs, err
		}
		directory := filepath.Dir(buildFilePath)
		allBuildDirs[directory] = &BuildFile{
			Directory:           directory,
			Stage:               content.Spec.Stage,
			BuildWith:           content.Spec.BuildWith,
			BuildImage:          content.Spec.BuildImage,
			DependsOn:           applyGlobToPaths(content.Spec.DependsOn, directory, rootPath),
			AdditionalArtifacts: convertToAbsolutePaths(content.AdditionalArtifacts, directory, rootPath),
			Cache: Cache{
				Cachefiles: convertToAbsolutePaths(content.Cache.Cachefiles, directory, rootPath),
				Hashfiles:  convertToAbsolutePaths(content.Cache.Hashfiles, directory, rootPath),
			},
			PreBuildActions:            content.PreBuildActions,
			PostBuildActions:           content.PostBuildActions,
			IntegrationTestDirectories: convertToAbsolutePaths(content.IntegrationTestDirectories, directory, rootPath),
		}
	}
	return allBuildDirs, nil
}

func readInputLines(r io.Reader) ([]string, error) {
	var paths []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		path := scanner.Text()
		if len(path) > 0 {
			paths = append(paths, path)
		}
	}

	if err := scanner.Err(); err != nil {
		return paths, err
	}
	return paths, nil
}

func findNearestBuildFile(path string, allBuildDirs BuildFileMap) (*BuildFile, error) {
	parentDir := filepath.Dir(path)
	if buildFile, ok := allBuildDirs[parentDir]; ok {
		return buildFile, nil
	} else if parentDir == "." {
		return buildFile, errors.New("file named Buildfile.yaml could not be found")
	} else {
		return findNearestBuildFile(parentDir, allBuildDirs)
	}
}

// Find the buildFiles which is the nearest ancestor for each of the files changed
func getChangedBuildFiles(changedFiles []string, allBuildDirs BuildFileMap) BuildFileMap {
	foldersToBuild := make(BuildFileMap)
	for _, changedFile := range changedFiles {
		buildFile, err := findNearestBuildFile(changedFile, allBuildDirs)
		if err != nil {
			log.Println(changedFile, err, "- Ignoring")
		} else {
			buildFile.FileTrigger = append(buildFile.FileTrigger, changedFile)
			foldersToBuild[buildFile.Directory] = buildFile
		}
	}
	return foldersToBuild
}

// For a given list of paths, return a set of all of its ancestor paths
// Ex: input [infrastructure/execution-plan/Directory, infrastructure/docker] - output [infrastructure, infrastructure/execution-plan, infrastructure/execution-plan/Directory, infrastructure/docker]
func getAllAncestorsSet(changedFiles []string) map[string]bool {
	// Since go doesn't have set by default, a map from string to 'true' is used to represent a set
	allAncestors := make(map[string]bool)
	if len(changedFiles) > 0 {
		allAncestors["."] = true
	}
	for _, path := range changedFiles {
		for len(path) > 0 && path != "." {
			allAncestors[path] = true
			path = filepath.Dir(path)
		}
	}
	return allAncestors
}

func recurseBuildDependencies(foldersToBuild BuildFileMap, changedFiles []string, allBuildDirs BuildFileMap) (resultingFoldersToBuild BuildFileMap, resultingChangedFiles []string) {
	allAncestors := getAllAncestorsSet(changedFiles)
	for _, buildDir := range allBuildDirs {
		var allDependencies []string
		allDependencies = append(allDependencies, buildDir.DependsOn...)
		allDependencies = append(allDependencies, buildDir.BuildWith)
		for _, dependency := range allDependencies {
			if _, ok := allAncestors[dependency]; ok {
				if !hasValue(buildDir.FileTrigger, dependency+" [dependency]") {
					buildDir.FileTrigger = append(buildDir.FileTrigger, dependency+" [dependency]")
				}
				foldersToBuild[buildDir.Directory] = buildDir
				changedFiles = append(changedFiles, buildDir.Directory)
			}
		}
	}
	return foldersToBuild, changedFiles
}

func hasValue(stringArray []string, str string) bool {
	for i := 0; i < len(stringArray); i++ {
		if stringArray[i] == str {
			return true
		}
	}
	return false
}

func getCommand(directory, trigger string, makeVariables []string) string {
	if directory == "" {
		return "make " + trigger
	}
	var s []string
	if len(makeVariables) == 0 {
		s = []string{"make -C", directory, trigger}
	} else {
		s = []string{"make -C", directory, trigger, strings.Join(makeVariables, " ")}
	}
	return strings.Join(s, " ")
}

func getPublishACommand(buildDir *BuildFile, trigger string) ContainerCommands {
	return ContainerCommands{
		Container: Container{
			Image: "",
		},
		Commands: []string{
			getCommand(
				buildDir.Directory, "publish"+"-"+trigger,
				[]string{},
			),
		},
		Directory: buildDir.Directory,
	}
}

func getPublishBCommand(buildDir *BuildFile, trigger, image string) ContainerCommands {
	return ContainerCommands{
		Container: Container{
			Image: image,
		},
		Commands: []string{
			getCommand(
				buildDir.Directory, "publish"+"-"+trigger,
				[]string{},
			),
		},
		Directory: buildDir.Directory,
	}
}

func getStageACommand(buildDir *BuildFile, trigger string, imageTargets ImageTargets) (commands ContainerCommands) {
	containerCommands := ContainerCommands{
		Container: Container{
			Image: "",
		},
		Commands: []string{
			getCommand(
				buildDir.Directory, "build"+"-"+trigger,
				[]string{
					"DOCKER_REGISTRY_URI=" + imageTargets.ImageRegistryURI,
					"ADDITIONAL_IMAGE_TAGS=" + imageTargets.ImageTag,
				},
			),
		},
		Directory: buildDir.Directory,
	}
	return containerCommands
}

// Adding as separate function so that external dependency is tested as well
func glob(filePattern string) ([]string, error) {
	return filepathx.Glob(filePattern)
}

func getHash(filePatterns []string) (string, error) {
	fileList := []string{}
	sort.Strings(filePatterns)
	for _, filePattern := range filePatterns {
		fileInfo, err := os.Stat(filePattern)
		if err == nil && fileInfo.IsDir() {
			filePattern += "/**/*"
		}
		// filepath.Glob does not support ** wildcard, it treats ** similar to * instead
		// https://github.com/golang/go/issues/11862
		// to that end, we use filepathx that support ** and interprets **/*.*
		// https://github.com/yargevad/filepathx
		files, err := glob(filePattern)
		if err != nil {
			return "", err
		}
		fileList = append(fileList, files...)
	}
	sort.Strings(fileList)
	//  from https://github.com/golang/mod/blob/ce943fd02449f621243c9ea6e64098e84752b92b/sumdb/dirhash/hash.go#L44
	h := sha256.New()
	for _, file := range fileList {
		fileInfo, _ := os.Stat(file)
		if !fileInfo.IsDir() {
			r, err := os.Open(file)
			if err != nil {
				return "", err
			}
			hf := sha256.New()
			_, err = io.Copy(hf, r)
			if err != nil {
				return "", err
			}
			err = r.Close()
			if err != nil {
				return "", err
			}
			fmt.Fprintf(h, "%x  %s\n", hf.Sum(nil), file)
		}
	}
	hash := fmt.Sprintf("%x", h.Sum(nil))
	return hash, nil
}

func getIntegrationTestDirectories(foldersToBuild, allBuildDirs BuildFileMap) []*BuildFile {
	// use map to avoid duplication
	integrationTestDirectoriesMap := BuildFileMap{}
	for _, buildDir := range foldersToBuild {
		if len(buildDir.IntegrationTestDirectories) > 0 {
			for _, integrationTestDirectory := range buildDir.IntegrationTestDirectories {
				if integrationFile, ok := allBuildDirs[integrationTestDirectory]; ok {
					integrationTestDirectoriesMap[integrationTestDirectory] = integrationFile
				} else {
					log.Fatalf("Invalid integrationTestDirectory %s found\n", integrationTestDirectory)
				}
			}
		}
	}
	// change to array
	integrationTestDirectories := []*BuildFile{}
	for _, integrationFile := range integrationTestDirectoriesMap {
		integrationTestDirectories = append(integrationTestDirectories, integrationFile)
	}
	return integrationTestDirectories
}

func getCleanupCommand(trigger string) []ContainerCommands {
	// Cleanup does not output an image right now. If changing from single command, this might be used.
	cleanupCommand := ContainerCommands{}
	cleanupCommand.Commands = []string{
		getCommand("", "cleanup"+"-"+trigger, []string{}),
	}
	return []ContainerCommands{
		cleanupCommand,
	}
}

func getStageBCommand(buildDir *BuildFile, trigger, image string) (ContainerCommands, error) {
	containerCommands := ContainerCommands{}
	cacheKey := ""
	var makeVariables []string
	var err error
	if len(buildDir.Cache.Hashfiles) > 0 {
		cacheKey, err = getHash(buildDir.Cache.Hashfiles)
		if err != nil {
			return containerCommands, err
		}
	}
	return ContainerCommands{
		Container: Container{
			Image: image,
		},
		Commands: []string{
			getCommand(buildDir.Directory, "build"+"-"+trigger, makeVariables),
		},
		Artifacts:        buildDir.AdditionalArtifacts,
		Directory:        buildDir.Directory,
		Cachefiles:       buildDir.Cache.Cachefiles,
		Cachekey:         cacheKey,
		PreBuildActions:  buildDir.PreBuildActions,
		PostBuildActions: buildDir.PostBuildActions,
	}, nil
}

func getIntegrationCommand(integrationTestDirectory *BuildFile, trigger string) (ContainerCommands, error) {
	// As of now, reuse build command format, if it diverges, can add more code here
	integrationCommand, err := getStageBCommand(integrationTestDirectory, trigger, "")
	if err != nil {
		return integrationCommand, err
	}
	integrationCommand.Commands = []string{
		getCommand(integrationTestDirectory.Directory, "integration-test"+"-"+trigger, []string{}),
	}
	return integrationCommand, nil
}

// Stage_A ----> Docker images
// Stage_B ----> Micro Services, other components
func getStageAAndStageBBuildFiles(foldersToBuild BuildFileMap) (stageAFolders, stageBFolders BuildFileMap) {
	stageA := BuildFileMap{}
	stageB := BuildFileMap{}
	for _, folder := range foldersToBuild {
		switch folder.Stage {
		case "A":
			stageA[folder.Directory] = folder
		case "B":
			stageB[folder.Directory] = folder
		default:
			log.Fatalf("stage for folder %s missing or not an allowed value (A, B): %v", folder.Directory, folder.Stage)
		}
	}
	return stageA, stageB
}

func getHashForImage(imageAbsPath string) (string, error) {
	var hashProperties []string
	_, err := os.Stat(imageAbsPath)
	if !os.IsExist(err) {
		hashProperties, _ = OSReadDir(imageAbsPath)
		if err != nil {
			return "", err
		}
	}
	hash, err := getHash(hashProperties)
	if err != nil {
		return "", err
	}
	return hash, nil
}

func OSReadDir(root string) ([]string, error) {
	var files []string
	f, err := os.Open(root)
	if err != nil {
		return files, err
	}
	fileInfo, err := f.Readdir(-1)
	errFileClose := f.Close()
	if err != nil {
		return files, err
	}
	if errFileClose != nil {
		return files, err
	}

	for _, file := range fileInfo {
		files = append(files, getAbsolutePath(file.Name(), root, root))
	}
	return files, nil
}

func validateReadImageBuildFile(path string) (*InputYaml, error) {
	checkPath, _ := os.Stat(path)
	if checkPath == nil || !checkPath.IsDir() {
		return nil, errors.New("invalid build image directory")
	}
	buildFilePath := filepath.Join(path, buildFileName)
	_, err := os.Stat(buildFilePath)
	// Check if the buildFile does not exist
	if os.IsExist(err) {
		return nil, err
	}
	buf, err := os.ReadFile(buildFilePath)
	if err != nil {
		return nil, err
	}
	content := InputYaml{}
	err = yaml.Unmarshal(buf, &content)
	if err != nil {
		return nil, err
	}
	// Check if the buildFile has a buildWith
	if content.Spec.BuildWith != "" {
		return nil, fmt.Errorf("%s docker image buildfile cannot reference another buildfile: %s", path, content)
	}

	// Check if the buildFile has a dependency
	if content.Spec.DependsOn != nil {
		return nil, fmt.Errorf("%s docker image buildfile cannot depend on another service/image", path)
	}
	// Check if the buildFile has a dependency
	if content.Metadata.Registry == "" {
		return nil, fmt.Errorf("%s docker image must have registry url", path)
	}
	return &content, nil
}

type ImageTargets struct {
	ImageRegistryURI string
	ImageName        string
	ImageTag         string
}

func getImageTargets(name, path, hash string) (ImageTargets, error) {
	// TODO: Don't understand why this invokes "getAbsolutePath" of no basePath or rootPath are used
	baseImageDirectory := getAbsolutePath(path, "", "")
	content, err := validateReadImageBuildFile(baseImageDirectory)
	if err != nil {
		return ImageTargets{}, err
	}
	return ImageTargets{
		ImageRegistryURI: content.Metadata.Registry,
		ImageName:        name,
		ImageTag:         "dir" + hash,
	}, nil
}

func getImageUrl(targets ImageTargets) string {
	if targets != (ImageTargets{}) {
		return targets.ImageRegistryURI + "/" + targets.ImageName + ":" + targets.ImageTag
	}
	return ""
}

func convertToExecutionPlan(foldersToBuild, allBuildDirs BuildFileMap, trigger string) (ExecutionPlan, error) {
	executionPlan := ExecutionPlan{
		StageA:          []ContainerCommands{},
		StageB:          []ContainerCommands{},
		PublishA:        []ContainerCommands{},
		PublishB:        []ContainerCommands{},
		IntegrationTest: []ContainerCommands{},
		Cleanup:         []ContainerCommands{},
	}
	stageAFiles, stageBBuildFiles := getStageAAndStageBBuildFiles(foldersToBuild)

	for _, fileForStageA := range stageAFiles {
		imageAbsPath, err := filepath.Abs(fileForStageA.Directory)
		if err != nil {
			return ExecutionPlan{}, err
		}

		hash, err := getHashForImage(imageAbsPath)
		if err != nil {
			return ExecutionPlan{}, err
		}
		imageTargets, err := getImageTargets(fileForStageA.Directory, imageAbsPath, hash)
		if err != nil {
			return ExecutionPlan{}, err
		}

		dockerCommand := getStageACommand(fileForStageA, trigger, imageTargets)

		executionPlan.StageA = append(executionPlan.StageA, dockerCommand)
		executionPlan.PublishA = append(executionPlan.PublishA, getPublishACommand(fileForStageA, trigger))
	}

	for _, fileForStageB := range stageBBuildFiles {
		if fileForStageB.BuildWith == "" {
			return ExecutionPlan{}, fmt.Errorf("%q: this service's Buildfile must have a 'BuildWith' reference to the builder image ", fileForStageB.Directory)
		}
		err := validateServiceBuildfileDependsOn(fileForStageB, rootPath, rootPath)
		if err != nil {
			return ExecutionPlan{}, err
		}

		imageAbsPath, err := filepath.Abs(fileForStageB.BuildWith)
		if err != nil {
			return ExecutionPlan{}, err
		}

		hash, err := getHashForImage(imageAbsPath)
		if err != nil {
			return ExecutionPlan{}, err
		}

		imageTargets, err := getImageTargets(fileForStageB.BuildWith, imageAbsPath, hash)
		if err != nil {
			return ExecutionPlan{}, err
		}
		imageUrl := getImageUrl(imageTargets)
		buildCommand, err := getStageBCommand(fileForStageB, trigger, imageUrl)
		if err != nil {
			return ExecutionPlan{}, err
		}
		executionPlan.StageB = append(executionPlan.StageB, buildCommand)
		executionPlan.PublishB = append(executionPlan.PublishB, getPublishBCommand(fileForStageB, trigger, imageUrl))
	}

	integrationTestDirectories := getIntegrationTestDirectories(foldersToBuild, allBuildDirs)
	for _, integrationTestDirectory := range integrationTestDirectories {
		integrationCommand, err := getIntegrationCommand(integrationTestDirectory, trigger)
		if err != nil {
			return ExecutionPlan{}, err
		}
		executionPlan.IntegrationTest = append(executionPlan.IntegrationTest, integrationCommand)
	}
	if len(executionPlan.IntegrationTest) > 0 || len(executionPlan.StageB) > 0 {
		executionPlan.Cleanup = getCleanupCommand(trigger)
	}
	return executionPlan, nil
}

// Convert from internal structure BuildFileMap to output json structure Array<Map<BuildContainer,Array<Command>>>
func convertToJson(foldersToBuild, allBuildDirs BuildFileMap, trigger string) (string, error) {
	executionPlan, err := convertToExecutionPlan(foldersToBuild, allBuildDirs, trigger)
	if err != nil {
		return "", err
	}

	jsonCommands, err := json.MarshalIndent(executionPlan, "", "  ")
	if err != nil {
		return string(jsonCommands), err
	}
	return string(jsonCommands), nil
}

func getFoldersToBuild(changedFiles []string, rootPath string) (folders, allDirs BuildFileMap, errr error) {
	allBuildDirs, err := getAllBuildFiles(rootPath)
	if err != nil {
		return make(BuildFileMap), allBuildDirs, err
	}

	foldersToBuild := getChangedBuildFiles(changedFiles, allBuildDirs)

	// If any of the buildFiles have a dependency on the changed files, add those to be built as well, recurse till there are no more changes
	previousLength := -1
	for previousLength < len(foldersToBuild) {
		previousLength = len(foldersToBuild)
		foldersToBuild, changedFiles = recurseBuildDependencies(foldersToBuild, changedFiles, allBuildDirs)
	}
	return foldersToBuild, allBuildDirs, nil
}

func getJsonFoldersToBuild(changedFiles []string, rootPath, trigger string) (output, fileTriggers string, errr error) {
	foldersToBuild, allBuildDirs, err := getFoldersToBuild(changedFiles, rootPath)
	fileTriggerInfo := printFileTriggersForBuildfiles(foldersToBuild)
	if err != nil {
		return "", "", err
	}
	outputJson, err := convertToJson(foldersToBuild, allBuildDirs, trigger)
	if err != nil {
		return outputJson, fileTriggerInfo, err
	}
	return outputJson, fileTriggerInfo, nil
}

func printFileTriggersForBuildfiles(foldersToBuild BuildFileMap) string {
	var outputInfo string
	outputInfo = "========== File Triggers For Buildfiles =============\n"
	keys := make([]string, 0, len(foldersToBuild))
	for k := range foldersToBuild {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		outputInfo += foldersToBuild[k].Directory + " -> " + strings.Join(foldersToBuild[k].FileTrigger, ", ") + "\n"
	}
	outputInfo += "=====================================================\n"
	return outputInfo
}

func validateServiceBuildfileDependsOn(buildFile *BuildFile, basePath, rootPath string) error {
	absDockerDirectoryPath := getAbsolutePath(dockerDirectoryPath, basePath, rootPath)
	for _, dependency := range buildFile.DependsOn {
		absDependencyPath := getAbsolutePath(dependency, basePath, rootPath)
		if dependency == buildFile.Directory {
			return fmt.Errorf("%q: service buildFile cannot depend on itself", buildFile.Directory)
		}
		if dependency != "" && strings.HasPrefix(absDependencyPath, absDockerDirectoryPath) {
			return fmt.Errorf("%q: service buildFile cannot have a dependency on docker images", buildFile.Directory)
		}
	}

	return nil
}
