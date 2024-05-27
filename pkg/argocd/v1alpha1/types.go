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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

type SyncOptions []string

// See https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#ObjectMeta for more fields, if necessary
type ObjectMeta struct {
	Name        string            `json:"name"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Finalizers  []string          `json:"finalizers,omitempty"`
}

// This file is a subset of https://github.com/argoproj/argo-cd/blob/v1.8.7/pkg/apis/application/v1alpha1/types.go
// Importing the argocd project directly would drag in a huge number of dependencies.
var ApplicationTypeMeta metav1.TypeMeta = metav1.TypeMeta{
	APIVersion: "argoproj.io/v1alpha1",
	Kind:       "Application",
}

type Application struct {
	metav1.TypeMeta `json:",inline"`
	ObjectMeta      `json:"metadata"`
	Spec            ApplicationSpec `json:"spec"`
}

type ApplicationSpec struct {
	// Source is a reference to the location ksonnet application definition
	Source ApplicationSource `json:"source" protobuf:"bytes,1,opt,name=source"`
	// Destination overrides the kubernetes server and namespace defined in the environment ksonnet app.yaml
	Destination ApplicationDestination `json:"destination" protobuf:"bytes,2,name=destination"`
	// Project is a application project name. Empty name means that application belongs to 'default' project.
	Project string `json:"project" protobuf:"bytes,3,name=project"`
	// SyncPolicy controls when a sync will be performed
	SyncPolicy *SyncPolicy `json:"syncPolicy,omitempty" protobuf:"bytes,4,name=syncPolicy"`
	// IgnoreDifferences controls resources fields which should be ignored during comparison
	IgnoreDifferences []ResourceIgnoreDifferences `json:"ignoreDifferences,omitempty" protobuf:"bytes,5,name=ignoreDifferences"`
}

// ApplicationSource contains information about github repository, path within repository and target application environment.
type ApplicationSource struct {
	// RepoURL is the repository URL of the application manifests
	RepoURL string `json:"repoURL" protobuf:"bytes,1,opt,name=repoURL"`
	// Path is a directory path within the Git repository
	Path string `json:"path,omitempty" protobuf:"bytes,2,opt,name=path"`
	// TargetRevision defines the commit, tag, or branch in which to sync the application to.
	// If omitted, will sync to HEAD
	TargetRevision string `json:"targetRevision,omitempty" protobuf:"bytes,4,opt,name=targetRevision"`
}

type ApplicationDestination struct {
	// Server overrides the environment server value in the ksonnet app.yaml
	Server string `json:"server,omitempty" protobuf:"bytes,1,opt,name=server"`
	// Namespace overrides the environment namespace value in the ksonnet app.yaml
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,2,opt,name=namespace"`
	// Name of the destination cluster which can be used instead of server (url) field
	Name string `json:"name,omitempty" protobuf:"bytes,3,opt,name=name"`
}

type SyncPolicy struct {
	// Automated will keep an application synced to the target revision
	Automated *SyncPolicyAutomated `json:"automated,omitempty" protobuf:"bytes,1,opt,name=automated"`
	// SyncOptions provide per-sync sync-options, e.g. Validate=false
	SyncOptions SyncOptions `json:"syncOptions,omitempty" protobuf:"bytes,9,opt,name=syncOptions"`
}

type SyncPolicyAutomated struct {
	// Prune will prune resources automatically as part of automated sync (default: false)
	Prune bool `json:"prune,omitempty" protobuf:"bytes,1,opt,name=prune"`
	// SelfHeal enables auto-syncing if  (default: false)
	SelfHeal bool `json:"selfHeal,omitempty" protobuf:"bytes,2,opt,name=selfHeal"`
	// AllowEmpty allows apps have zero live resources (default: false)
	AllowEmpty bool `json:"allowEmpty,omitempty" protobuf:"bytes,3,opt,name=allowEmpty"`
}

var AppProjectTypeMeta metav1.TypeMeta = metav1.TypeMeta{
	APIVersion: "argoproj.io/v1alpha1",
	Kind:       "AppProject",
}

type AppProject struct {
	metav1.TypeMeta `json:",inline"`
	ObjectMeta      `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
	Spec            AppProjectSpec `json:"spec" protobuf:"bytes,2,opt,name=spec"`
}

// AccessEntry can be used for blacklists and whitelists, see https://argo-cd.readthedocs.io/en/stable/user-guide/projects/#configuring-global-projects-v18
type AccessEntry struct {
	// the group is meant for security restrictions, that we don't use. For us this will probably always be '*'.
	Group string `json:"group,omitempty" protobuf:"bytes,1,opt,name=group"`
	// the kind of k8s resource that is allowed/forbidden, e.g. "ClusterSecretStore"
	Kind string `json:"kind,omitempty" protobuf:"bytes,2,opt,name=kind"`
}

// AppProjectSpec is the specification of an AppProject
type AppProjectSpec struct {
	// SourceRepos contains list of repository URLs which can be used for deployment
	SourceRepos []string `json:"sourceRepos,omitempty" protobuf:"bytes,1,name=sourceRepos"`
	// Destinations contains list of destinations available for deployment
	Destinations []ApplicationDestination `json:"destinations,omitempty" protobuf:"bytes,2,name=destination"`
	// Description contains optional project description
	Description string `json:"description,omitempty" protobuf:"bytes,3,opt,name=description"`
	// SyncWindows controls when syncs can be run for apps in this project
	SyncWindows SyncWindows `json:"syncWindows,omitempty" protobuf:"bytes,8,opt,name=syncWindows"`
	// By default, cluster wide resources are forbidden in argo. With this whitelist we can allow them
	ClusterResourceWhitelist []AccessEntry `json:"clusterResourceWhitelist,omitempty" protobuf:"bytes,9,opt,name=clusterResourceWhitelist"`
}

// SyncWindows is a collection of sync windows in this project
type SyncWindows []*SyncWindow

// SyncWindow contains the kind, time, duration and attributes that are used to assign the syncWindows to apps
type SyncWindow struct {
	// Kind defines if the window allows or blocks syncs
	Kind string `json:"kind,omitempty" protobuf:"bytes,1,opt,name=kind"`
	// Schedule is the time the window will begin, specified in cron format
	Schedule string `json:"schedule,omitempty" protobuf:"bytes,2,opt,name=schedule"`
	// Duration is the amount of time the sync window will be open
	Duration string `json:"duration,omitempty" protobuf:"bytes,3,opt,name=duration"`
	// Applications contains a list of applications that the window will apply to
	Applications []string `json:"applications,omitempty" protobuf:"bytes,4,opt,name=applications"`

	// NOTE: Applications, Clusters and Namespaces are an OR-expression in ArgoCd!
	// Example: You set Application="my-app" and Clusters="*"
	// This means: It applies to ALL applications.
	// To avoid this, we omit the settings for Clusters and Namespaces completely.

	// Namespaces contains a list of namespaces that the window will apply to
	//Namespaces []string `json:"namespaces,omitempty" protobuf:"bytes,5,opt,name=namespaces"`
	// Clusters contains a list of clusters that the window will apply to
	//Clusters []string `json:"clusters,omitempty" protobuf:"bytes,6,opt,name=clusters"`
	// ManualSync enables manual syncs when they would otherwise be blocked
	ManualSync bool `json:"manualSync,omitempty" protobuf:"bytes,7,opt,name=manualSync"`
}

// ResourceIgnoreDifferences contains resource filter and list of json paths which should be ignored during comparison with live state.
type ResourceIgnoreDifferences struct {
	Group                 string   `json:"group,omitempty" protobuf:"bytes,1,opt,name=group"`
	Kind                  string   `json:"kind" protobuf:"bytes,2,opt,name=kind"`
	Name                  string   `json:"name,omitempty" protobuf:"bytes,3,opt,name=name"`
	Namespace             string   `json:"namespace,omitempty" protobuf:"bytes,4,opt,name=namespace"`
	JSONPointers          []string `json:"jsonPointers,omitempty" protobuf:"bytes,5,opt,name=jsonPointers"`
	JqPathExpressions     []string `json:"jqPathExpressions,omitempty" protobuf:"bytes,6,opt,name=jqPathExpressions"`
	ManagedFieldsManagers []string `json:"managedFieldsManagers,omitempty" protobuf:"bytes,7,opt,name=managedFieldsManagers"`
}

// Types for Git Webhooks

type Repository struct {
	ID       int64  `json:"id"`
	NodeID   string `json:"node_id"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Owner    struct {
		Login             string `json:"login"`
		ID                int64  `json:"id"`
		NodeID            string `json:"node_id"`
		AvatarURL         string `json:"avatar_url"`
		GravatarID        string `json:"gravatar_id"`
		URL               string `json:"url"`
		HTMLURL           string `json:"html_url"`
		FollowersURL      string `json:"followers_url"`
		FollowingURL      string `json:"following_url"`
		GistsURL          string `json:"gists_url"`
		StarredURL        string `json:"starred_url"`
		SubscriptionsURL  string `json:"subscriptions_url"`
		OrganizationsURL  string `json:"organizations_url"`
		ReposURL          string `json:"repos_url"`
		EventsURL         string `json:"events_url"`
		ReceivedEventsURL string `json:"received_events_url"`
		Type              string `json:"type"`
		SiteAdmin         bool   `json:"site_admin"`
	} `json:"owner"`
	Private          bool      `json:"private"`
	HTMLURL          string    `json:"html_url"`
	Description      string    `json:"description"`
	Fork             bool      `json:"fork"`
	URL              string    `json:"url"`
	ForksURL         string    `json:"forks_url"`
	KeysURL          string    `json:"keys_url"`
	CollaboratorsURL string    `json:"collaborators_url"`
	TeamsURL         string    `json:"teams_url"`
	HooksURL         string    `json:"hooks_url"`
	IssueEventsURL   string    `json:"issue_events_url"`
	EventsURL        string    `json:"events_url"`
	AssigneesURL     string    `json:"assignees_url"`
	BranchesURL      string    `json:"branches_url"`
	TagsURL          string    `json:"tags_url"`
	BlobsURL         string    `json:"blobs_url"`
	GitTagsURL       string    `json:"git_tags_url"`
	GitRefsURL       string    `json:"git_refs_url"`
	TreesURL         string    `json:"trees_url"`
	StatusesURL      string    `json:"statuses_url"`
	LanguagesURL     string    `json:"languages_url"`
	StargazersURL    string    `json:"stargazers_url"`
	ContributorsURL  string    `json:"contributors_url"`
	SubscribersURL   string    `json:"subscribers_url"`
	SubscriptionURL  string    `json:"subscription_url"`
	CommitsURL       string    `json:"commits_url"`
	GitCommitsURL    string    `json:"git_commits_url"`
	CommentsURL      string    `json:"comments_url"`
	IssueCommentURL  string    `json:"issue_comment_url"`
	ContentsURL      string    `json:"contents_url"`
	CompareURL       string    `json:"compare_url"`
	MergesURL        string    `json:"merges_url"`
	ArchiveURL       string    `json:"archive_url"`
	DownloadsURL     string    `json:"downloads_url"`
	IssuesURL        string    `json:"issues_url"`
	PullsURL         string    `json:"pulls_url"`
	MilestonesURL    string    `json:"milestones_url"`
	NotificationsURL string    `json:"notifications_url"`
	LabelsURL        string    `json:"labels_url"`
	ReleasesURL      string    `json:"releases_url"`
	CreatedAt        int64     `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	PushedAt         int64     `json:"pushed_at"`
	GitURL           string    `json:"git_url"`
	SSHURL           string    `json:"ssh_url"`
	CloneURL         string    `json:"clone_url"`
	SvnURL           string    `json:"svn_url"`
	Homepage         *string   `json:"homepage"`
	Size             int64     `json:"size"`
	StargazersCount  int64     `json:"stargazers_count"`
	WatchersCount    int64     `json:"watchers_count"`
	Language         *string   `json:"language"`
	HasIssues        bool      `json:"has_issues"`
	HasDownloads     bool      `json:"has_downloads"`
	HasWiki          bool      `json:"has_wiki"`
	HasPages         bool      `json:"has_pages"`
	ForksCount       int64     `json:"forks_count"`
	MirrorURL        *string   `json:"mirror_url"`
	OpenIssuesCount  int64     `json:"open_issues_count"`
	Forks            int64     `json:"forks"`
	OpenIssues       int64     `json:"open_issues"`
	Watchers         int64     `json:"watchers"`
	DefaultBranch    string    `json:"default_branch"`
	Stargazers       int64     `json:"stargazers"`
	MasterBranch     string    `json:"master_branch"`
}

// PushPayload was copied from argoCd's payload.go, to ensure we use the same format
// PushPayload contains the information for GitHub's push hook event
type PushPayload struct {
	Ref        string   `json:"ref"`
	Before     string   `json:"before"`
	After      string   `json:"after"`
	Created    bool     `json:"created"`
	Deleted    bool     `json:"deleted"`
	Forced     bool     `json:"forced"`
	BaseRef    *string  `json:"base_ref"`
	Compare    string   `json:"compare"`
	Commits    []Commit `json:"commits"`
	HeadCommit struct {
		ID        string `json:"id"`
		NodeID    string `json:"node_id"`
		TreeID    string `json:"tree_id"`
		Distinct  bool   `json:"distinct"`
		Message   string `json:"message"`
		Timestamp string `json:"timestamp"`
		URL       string `json:"url"`
		Author    struct {
			Name     string `json:"name"`
			Email    string `json:"email"`
			Username string `json:"username"`
		} `json:"author"`
		Committer struct {
			Name     string `json:"name"`
			Email    string `json:"email"`
			Username string `json:"username"`
		} `json:"committer"`
		Added    []string `json:"added"`
		Removed  []string `json:"removed"`
		Modified []string `json:"modified"`
	} `json:"head_commit"`
	Repository Repository `json:"repository"`
	Pusher     struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"pusher"`
	Sender struct {
		Login             string `json:"login"`
		ID                int64  `json:"id"`
		NodeID            string `json:"node_id"`
		AvatarURL         string `json:"avatar_url"`
		GravatarID        string `json:"gravatar_id"`
		URL               string `json:"url"`
		HTMLURL           string `json:"html_url"`
		FollowersURL      string `json:"followers_url"`
		FollowingURL      string `json:"following_url"`
		GistsURL          string `json:"gists_url"`
		StarredURL        string `json:"starred_url"`
		SubscriptionsURL  string `json:"subscriptions_url"`
		OrganizationsURL  string `json:"organizations_url"`
		ReposURL          string `json:"repos_url"`
		EventsURL         string `json:"events_url"`
		ReceivedEventsURL string `json:"received_events_url"`
		Type              string `json:"type"`
		SiteAdmin         bool   `json:"site_admin"`
	} `json:"sender"`
	Installation struct {
		ID int `json:"id"`
	} `json:"installation"`
}

type Commit struct {
	Sha       string `json:"sha"`
	ID        string `json:"id"`
	NodeID    string `json:"node_id"`
	TreeID    string `json:"tree_id"`
	Distinct  bool   `json:"distinct"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
	URL       string `json:"url"`
	Author    struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Username string `json:"username"`
	} `json:"author"`
	Committer struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Username string `json:"username"`
	} `json:"committer"`
	Added    []string `json:"added"`
	Removed  []string `json:"removed"`
	Modified []string `json:"modified"`
}
