package event

import (
	"testing"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/google/go-cmp/cmp"
)

func Test_roundtrip(t *testing.T) {
	for _, test := range []struct {
		Name  string
		Event Event
	}{
		{
			Name: "new-release",
			Event: &NewRelease{
				Environments: map[string]struct{}{
					"env1": {},
					"env2": {},
				},
			},
		},
		{
			Name: "deployment",
			Event: &Deployment{
				AppLocks: map[string]AppLock{
					"lock1": {
						App:     "app1",
						Env:     "env1",
						Message: "msg1",
					},
					"lock2": {
						App:     "app2",
						Env:     "env2",
						Message: "msg2",
					},
				},
				EnvLocks: map[string]EnvLock{
					"lock3": {
						Env:     "env3",
						Message: "msg3",
					},
				},
			},
		},
	} {
		test := test
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()
			fs := memfs.New()
			if err := Write(fs, "test", test.Event); err != nil {
				t.Fatal("writing event:", err)
			}
			result, err := Read(fs, "test")
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(test.Event, result); diff != "" {
				t.Error("wrong result:\n", diff)
			}
		})
	}
}
