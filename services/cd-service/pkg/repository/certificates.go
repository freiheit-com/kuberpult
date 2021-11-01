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
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"

	"github.com/freiheit-com/kuberpult/pkg/logger"
	git "github.com/libgit2/git2go/v31"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

var defaultKnownHostFiles = []string{"/etc/ssh/ssh_known_hosts"}

type Certificates struct {
	KnownHostsFile string
}

func (c *Certificates) load() (*certificateStore, error) {
	store := &certificateStore{
		sha256Hashes: map[string][]byte{},
	}
	if c.KnownHostsFile != "" {
		file, err := os.Open(c.KnownHostsFile)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			_, hosts, pubKey, _, _, err := ssh.ParseKnownHosts(scanner.Bytes())
			if err != nil {
				return nil, err
			}
			hasher := sha256.New()
			hasher.Write(pubKey.Marshal())
			sum := hasher.Sum(nil)
			for _, h := range hosts {
				store.sha256Hashes[h] = sum
			}

		}
	}
	return store, nil
}

type certificateStore struct {
	sha256Hashes map[string][]byte
}

func (store *certificateStore) CertificateCheckCallback(ctx context.Context) func(cert *git.Certificate, valid bool, hostname string) git.ErrorCode {
	if store == nil {
		return func(cert *git.Certificate, valid bool, hostname string) git.ErrorCode {
			return git.ErrorCodeCertificate // should never be called
		}
	}
	logger := logger.FromContext(ctx)
	return func(cert *git.Certificate, valid bool, hostname string) git.ErrorCode {
		if cert.Kind == git.CertificateHostkey {
			if hsh, ok := store.sha256Hashes[hostname]; ok {
				if bytes.Compare(hsh, cert.Hostkey.HashSHA256[:]) == 0 {
					return git.ErrOk
				} else {
					logger.WithFields(logrus.Fields{
						"hostname":         hostname,
						"hostkey.expected": fmt.Sprintf("%x", hsh),
						"hostkey.actual":   fmt.Sprintf("%x", cert.Hostkey.HashSHA256),
					}).Error("git.ssh.hostkeyMismatch")
				}
			} else {
				logger.WithFields(logrus.Fields{
					"hostname": hostname,
				}).Error("git.ssh.hostnameUnknown")
			}
		}
		return git.ErrorCodeCertificate
	}
}
