/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

Copyright 2021 freiheit.com*/
package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	git "github.com/libgit2/git2go/v31"
)

const example_known_hosts = "github.com ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEAq2A7hRGmdnm9tUDbO9IDSwBK6TbQa+PXYPCPy6rbTrTtw7PHkccKrpp0yVhp5HdEIcKr6pLlVDBfOLX9QUsyCOV0wzfjIJNlGEYsdlLJizHhbn2mUjvSAHQqZETYP81eFzLQNnPHt4EVVUh7VfDESU84KezmD5QlWpXLmvU31/yMf+Se8xhHTvKSCZIFImWwoG6mbUoWf9nzpIoaSjB+weqqUUmpaaasXVal72J+UX2B+2RPW3RcT0eOzQgqlJL3RKrTJvdsjE3JEAvGq3lGHSZXy28G3skua2SmVi/w4yCE6gbODqnTWlg7+wC604ydGXA8VJiS5ap43JXiUFFAaQ=="

func TestCertificateStore(t *testing.T) {
	tcs := []struct {
		Name       string
		KnownHosts string
		Host       string
		HashSHA256 [32]byte
		Expected   git.ErrorCode
	}{
		{
			Name:       "github.com working example",
			KnownHosts: example_known_hosts,
			Host:       "github.com",
			HashSHA256: [32]uint8{0x9d, 0x38, 0x5b, 0x83, 0xa9, 0x17, 0x52, 0x92, 0x56, 0x1a, 0x5e, 0xc4, 0xd4, 0x81, 0x8e, 0xa, 0xca, 0x51, 0xa2, 0x64, 0xf1, 0x74, 0x20, 0x11, 0x2e, 0xf8, 0x8a, 0xc3, 0xa1, 0x39, 0x49, 0x8f},
		},
		{
			Name:       "github.com bad hash",
			KnownHosts: example_known_hosts,
			Host:       "github.com",
			HashSHA256: [32]uint8{},
			Expected:   git.ErrorCodeCertificate,
		},
		{
			Name:       "github.com wrong hostname",
			KnownHosts: example_known_hosts,
			Host:       "gitlab.com",
			HashSHA256: [32]uint8{0x9d, 0x38, 0x5b, 0x83, 0xa9, 0x17, 0x52, 0x92, 0x56, 0x1a, 0x5e, 0xc4, 0xd4, 0x81, 0x8e, 0xa, 0xca, 0x51, 0xa2, 0x64, 0xf1, 0x74, 0x20, 0x11, 0x2e, 0xf8, 0x8a, 0xc3, 0xa1, 0x39, 0x49, 0x8f},
			Expected:   git.ErrorCodeCertificate,
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
			if result != tc.Expected {
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
