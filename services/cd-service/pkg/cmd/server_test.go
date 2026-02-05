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

package cmd

import (
	"os"
	"testing"
	"time"

	"github.com/freiheit-com/kuberpult/pkg/errorMatcher"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestCheckReleaseVersionLimit(t *testing.T) {
	for _, test := range []struct {
		name          string
		config        Config
		expectedError error
	}{
		{
			name: "versions limit equals the minimum allowed",
			config: Config{
				ReleaseVersionsLimit: minReleaseVersionsLimit,
			},
			expectedError: nil,
		},
		{
			name: "versions limit equals the maximum allowed",
			config: Config{
				ReleaseVersionsLimit: maxReleaseVersionsLimit,
			},
			expectedError: nil,
		},
		{
			name: "default versions limit",
			config: Config{
				ReleaseVersionsLimit: 20,
			},
			expectedError: nil,
		},
		{
			name: "versions limit below minimum",
			config: Config{
				ReleaseVersionsLimit: 3,
			},
			expectedError: releaseVersionsLimitError{limit: 3},
		},
		{
			name: "versions limit above maximum",
			config: Config{
				ReleaseVersionsLimit: 45,
			},
			expectedError: releaseVersionsLimitError{limit: 45},
		},
	} {
		tc := test
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := checkReleaseVersionLimit(tc.config.ReleaseVersionsLimit)
			if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestEnvVarParsing(t *testing.T) {
	tcs := []struct {
		Name        string
		Environment map[string]string

		ExpectedConfiguration *Config
		ExpectedError         error
	}{
		{
			Name: "should work if all the required environment variables are set and optional ones are not set",
			Environment: map[string]string{
				"KUBERPULT_DB_MAX_IDLE_CONNECTIONS": "10",
				"KUBERPULT_DB_MAX_OPEN_CONNECTIONS": "100",
				"KUBERPULT_ALLOWED_DOMAINS":         "freiheit.com",
				"KUBERPULT_MIGRATION_SERVER":        "manifest-repo-export-service:8443",
				"KUBERPULT_MIGRATION_SERVER_SECURE": "true",
				"KUBERPULT_GRPC_MAX_RECV_MSG_SIZE":  "1",
				"KUBERPULT_VERSION":                 "1.0",
				"KUBERPULT_LOCK_TYPE":               "db",
			},
			ExpectedConfiguration: &Config{
				DbMaxIdleConnections: 10,
				DbMaxOpenConnections: 100,

				AllowedDomains:        []string{"freiheit.com"},
				MigrationServer:       "manifest-repo-export-service:8443",
				MigrationServerSecure: true,
				GrpcMaxRecvMsgSize:    1,

				Version:  "1.0",
				LockType: "db",

				// configuration values with defaults:
				GitBranch:                "master",
				GitNetworkTimeout:        time.Minute,
				DogstatsdAddr:            "127.0.0.1:8125",
				EnableSqlite:             true,
				DexMockRole:              "Developer",
				GitMaximumCommitsPerPush: 1,
				MaximumQueueSize:         5,
				ArgoCdGenerateFiles:      true,
				MaxNumberOfThreads:       3,
				DbOption:                 "NO_DB",
				DbLocation:               "/kp/database",
				DbAuthProxyPort:          "5432",
				ReleaseVersionsLimit:     20,
				DbSslMode:                "verify-full",
			},
			ExpectedError: nil,
		},
		{
			Name: "should use all the environment variables that are overwriten",
			Environment: map[string]string{
				"KUBERPULT_DB_MAX_IDLE_CONNECTIONS": "10",
				"KUBERPULT_DB_MAX_OPEN_CONNECTIONS": "100",
				"KUBERPULT_ALLOWED_DOMAINS":         "freiheit.com",
				"KUBERPULT_MIGRATION_SERVER":        "manifest-repo-export-service:8443",
				"KUBERPULT_MIGRATION_SERVER_SECURE": "true",
				"KUBERPULT_GRPC_MAX_RECV_MSG_SIZE":  "1",
				"KUBERPULT_VERSION":                 "1.0",
				"KUBERPULT_LOCK_TYPE":               "db",

				"KUBERPULT_GIT_URL":                      "https://git.example.com",
				"KUBERPULT_GIT_BRANCH":                   "develop",
				"KUBERPULT_GIT_NETWORK_TIMEOUT":          "2m",
				"KUBERPULT_DOGSTATSD_ADDR":               "localhost:8125",
				"KUBERPULT_ENABLE_SQLITE":                "false",
				"KUBERPULT_DEX_MOCK_ROLE":                "Admin",
				"KUBERPULT_GIT_MAXIMUM_COMMITS_PER_PUSH": "10",
				"KUBERPULT_MAXIMUM_QUEUE_SIZE":           "10",
				"KUBERPULT_ARGOCD_GENERATE_FILES":        "false",
				"KUBERPULT_MAX_NUMBER_OF_THREADS":        "6",
				"KUBERPULT_DB_OPTION":                    "SQLITE",
				"KUBERPULT_DB_LOCATION":                  "/data/database",
				"KUBERPULT_DB_AUTH_PROXY_PORT":           "5433",
				"KUBERPULT_RELEASE_VERSIONS_LIMIT":       "25",
				"KUBERPULT_DB_SSL_MODE":                  "disable",

				"KUBERPULT_ENABLE_TRACING":           "false",
				"KUBERPULT_DEX_RBAC_TEAM_PATH":       "/etc/team",
				"KUBERPULT_DEX_RBAC_POLICY_PATH":     "/etc/policy",
				"KUBERPULT_DB_USER_PASSWORD":         "1234",
				"KUBERPULT_GIT_WRITE_COMMIT_DATA":    "false",
				"KUBERPULT_DB_USER_NAME":             "toor",
				"KUBERPULT_DB_NAME":                  "kultkuber",
				"KUBERPULT_DEX_MOCK":                 "false",
				"KUBERPULT_ALLOW_LONG_APP_NAMES":     "true",
				"KUBERPULT_ENABLE_PROFILING":         "false",
				"KUBERPULT_MINOR_REGEXES":            "regex",
				"KUBERPULT_CHECK_CUSTOM_MIGRATIONS":  "true",
				"KUBERPULT_ENABLE_METRICS":           "true",
				"KUBERPULT_DEX_DEFAULT_ROLE_ENABLED": "true",
				"KUBERPULT_DATADOG_API_KEY_LOCATION": "/secret/location",
				"KUBERPULT_DB_MIGRATIONS_LOCATION":   "/path/migration",
				"KUBERPULT_DB_WRITE_ESL_TABLE_ONLY":  "true",
			},
			ExpectedConfiguration: &Config{
				DbMaxIdleConnections: 10,
				DbMaxOpenConnections: 100,

				AllowedDomains:        []string{"freiheit.com"},
				MigrationServer:       "manifest-repo-export-service:8443",
				MigrationServerSecure: true,
				GrpcMaxRecvMsgSize:    1,

				Version:  "1.0",
				LockType: "db",

				GitBranch:                "develop",
				GitNetworkTimeout:        2 * time.Minute,
				DogstatsdAddr:            "localhost:8125",
				EnableSqlite:             false,
				DexMockRole:              "Admin",
				GitMaximumCommitsPerPush: 10,
				MaximumQueueSize:         10,
				ArgoCdGenerateFiles:      true,
				MaxNumberOfThreads:       6,
				DbOption:                 "SQLITE",
				DbLocation:               "/data/database",
				DbAuthProxyPort:          "5433",
				ReleaseVersionsLimit:     25,
				DbSslMode:                "disable",

				GitUrl:                "https://git.example.com",
				EnableTracing:         false,
				DexRbacTeamPath:       "/etc/team",
				DexRbacPolicyPath:     "/etc/policy",
				DbUserPassword:        "1234",
				GitWriteCommitData:    false,
				DbUserName:            "toor",
				DbName:                "kultkuber",
				DexMock:               false,
				AllowLongAppNames:     true,
				EnableProfiling:       false,
				MinorRegexes:          "regex",
				CheckCustomMigrations: true,
				EnableMetrics:         true,
				DexDefaultRoleEnabled: true,
				DatadogApiKeyLocation: "/secret/location",
				DbMigrationsLocation:  "/path/migration",
				DbWriteEslTableOnly:   true,

				DexEnabled: false,
			},
			ExpectedError: nil,
		},
		{
			Name:                  "should return error if no environment variable is set",
			Environment:           map[string]string{},
			ExpectedConfiguration: nil,
			ExpectedError: errorMatcher.ContainsErrMatcher{
				Messages: []string{"KUBERPULT_DB_MAX_IDLE_CONNECTIONS", "could not read"},
			},
		},
		{
			Name: "should return error if GitNetworkTimeout is invalid",
			Environment: map[string]string{
				"KUBERPULT_DB_MAX_IDLE_CONNECTIONS": "10",
				"KUBERPULT_DB_MAX_OPEN_CONNECTIONS": "100",
				"KUBERPULT_ALLOWED_DOMAINS":         "freiheit.com",
				"KUBERPULT_MIGRATION_SERVER":        "manifest-repo-export-service:8443",
				"KUBERPULT_MIGRATION_SERVER_SECURE": "true",
				"KUBERPULT_GRPC_MAX_RECV_MSG_SIZE":  "1",
				"KUBERPULT_VERSION":                 "1.0",
				"KUBERPULT_LOCK_TYPE":               "db",
				"KUBERPULT_GIT_NETWORK_TIMEOUT":     "2x",
			},
			ExpectedConfiguration: nil,
			ExpectedError: errorMatcher.ContainsErrMatcher{
				Messages: []string{"KUBERPULT_GIT_NETWORK_TIMEOUT", "invalid duration"},
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			// given:
			os.Clearenv()
			for key, value := range tc.Environment {
				err := os.Setenv(key, value)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				t.Logf("set %s=%s", key, value)
			}

			// when:
			actual, err := parseEnvVars()

			// then:
			if diff := cmp.Diff(tc.ExpectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}

			if diff := cmp.Diff(actual, tc.ExpectedConfiguration); diff != "" {
				t.Logf("actual configuration: %v", actual)
				t.Logf("expected configuration: %v", tc.ExpectedConfiguration)
				t.Errorf("expected args:\n  %v\ngot:\n  %v\ndiff:\n  %s\n", actual, tc.ExpectedConfiguration, diff)
			}
		})
	}
}
