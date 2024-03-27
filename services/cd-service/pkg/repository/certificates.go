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

Copyright 2023 freiheit.com*/

package repository

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"

	"github.com/freiheit-com/kuberpult/pkg/logger"
	git "github.com/libgit2/git2go/v34"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
)

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

func (store *certificateStore) CertificateCheckCallback(ctx context.Context) func(cert *git.Certificate, valid bool, hostname string) error {
	if store == nil {
		return func(cert *git.Certificate, valid bool, hostname string) error {
			return fmt.Errorf("certificates error") // should never be called
		}
	}
	logger := logger.FromContext(ctx)
	return func(cert *git.Certificate, valid bool, hostname string) error {
		if cert.Kind == git.CertificateHostkey {
			if hsh, ok := store.sha256Hashes[hostname]; ok {
				if bytes.Equal(hsh, cert.Hostkey.HashSHA256[:]) {
					return nil
				} else {
					logger.Error("git.ssh.hostkeyMismatch",
						zap.String("hostname", hostname),
						zap.String("hostkey.expected", fmt.Sprintf("%x", hsh)),
						zap.String("hostkey.actual", fmt.Sprintf("%x", cert.Hostkey.HashSHA256)),
					)
				}
			} else {
				logger.Error("git.ssh.hostnameUnknown",
					zap.String("hostname", hostname),
				)
			}
		}
		return fmt.Errorf("certificates error")
	}
}
