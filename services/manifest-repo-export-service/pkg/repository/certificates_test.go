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

package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	git "github.com/libgit2/git2go/v34"
)

const example_known_hosts = "github.com ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBEmKSENjQEezOmxkZMy7opKgwFB9nkt5YRrYMjNuG5N87uRgg6CLrbo5wAdT/y6v0mKV0U2w0WZ2YB/++Tpockg="

func TestCertificateStore(t *testing.T) {
	tcs := []struct {
		Name       string
		KnownHosts string
		Host       string
		HashSHA256 [32]byte
		Expected   error
	}{
		{
			Name:       "github.com working example",
			KnownHosts: example_known_hosts,
			Host:       "github.com",
			HashSHA256: [32]uint8{0x9d, 0x38, 0x5b, 0x83, 0xa9, 0x17, 0x52, 0x92, 0x56, 0x1a, 0x5e, 0xc4, 0xd4, 0x81, 0x8e, 0xa, 0xca, 0x51, 0xa2, 0x64, 0xf1, 0x74, 0x20, 0x11, 0x2e, 0xf8, 0x8a, 0xc3, 0xa1, 0x39, 0x49, 0x8f},
			Expected:   nil,
		},
		{
			Name:       "github.com bad hash",
			KnownHosts: example_known_hosts,
			Host:       "github.com",
			HashSHA256: [32]uint8{},
			Expected:   fmt.Errorf("certificates error"),
		},
		{
			Name:       "github.com wrong hostname",
			KnownHosts: example_known_hosts,
			Host:       "gitlab.com",
			HashSHA256: [32]uint8{0x9d, 0x38, 0x5b, 0x83, 0xa9, 0x17, 0x52, 0x92, 0x56, 0x1a, 0x5e, 0xc4, 0xd4, 0x81, 0x8e, 0xa, 0xca, 0x51, 0xa2, 0x64, 0xf1, 0x74, 0x20, 0x11, 0x2e, 0xf8, 0x8a, 0xc3, 0xa1, 0x39, 0x49, 0x8f},
			Expected:   fmt.Errorf("certificates error"),
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			file := writeFile(t, tc.KnownHosts)
			certs := Certificates{
				KnownHostsFile: file,
			}
			store, err := certs.load()
			if err != nil {
				t.Fatal(err)
			}
			cert := git.Certificate{
				Kind: git.CertificateHostkey,
				Hostkey: git.HostkeyCertificate{
					HashSHA256: tc.HashSHA256,
				},
			}
			cb := store.CertificateCheckCallback(context.Background())
			result := cb(&cert, false, tc.Host)
			if result == nil && tc.Expected != nil {
				t.Errorf(" Expected an error but got nil %s", tc.Expected)
			}
			if tc.Expected != nil && result != nil && result.Error() != tc.Expected.Error() {
				t.Errorf("wrong check result: expected %s, actual %s", tc.Expected, result)
			}
		})
	}
}

func writeFile(t *testing.T, content string) string {
	d := t.TempDir()
	p := filepath.Join(d, "ssh_known_hosts")
	file, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	fmt.Fprint(file, content)
	return p
}
