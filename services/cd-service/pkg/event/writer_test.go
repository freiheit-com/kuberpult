package event

import (
	"reflect"
	"testing"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/google/go-cmp/cmp"
)

func Test_write(t *testing.T) {

	type named struct {
		Field string
	}

	example := "test"

	for _, test := range []struct {
		Name  string
		Event any
	}{
		{
			Name:  "string",
			Event: "hello",
		},
		{
			Name: "struct",
			Event: struct {
				Field1 string `fs:"field1"`
				Field2 string
			}{
				Field1: "f1",
				Field2: "f2",
			},
		},
		{
			Name: "map",
			Event: map[string]string{
				"file1": "content1",
				"file2": "content2",
			},
		},
		{
			Name: "named",
			Event: named{
				Field: "test",
			},
		},
		{
			Name: "map-empty",
			Event: map[string]struct{}{
				"x": {},
				"y": {},
			},
		},
		{
			Name:  "pointer",
			Event: &example,
		},
		{
			Name:  "nil-pointer",
			Event: (*string)(nil),
		},
	} {
		test := test
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()
			fs := memfs.New()
			err := write(fs, "test", test.Event)
			if err != nil {
				t.Fatal("writing event:", err)
			}
			result := reflect.New(reflect.TypeOf(test.Event))
			err = read(fs, "test", result.Interface())
			if err != nil {
				t.Fatal("reading event:", err)
			}
			if diff := cmp.Diff(test.Event, result.Elem().Interface()); diff != "" {
				t.Error("wrong result:\n", diff)
			}
		})
	}
}
