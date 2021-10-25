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
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"

	"github.com/freiheit-com/fdc-continuous-delivery/pkg/logger"
	git "github.com/libgit2/git2go/v31"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

type Credentials struct {
	SshKey string
}

func base64encode(c []byte) string {
	var buf bytes.Buffer
	enc := base64.NewEncoder(base64.StdEncoding, &buf)
	enc.Write(c)
	enc.Close()
	return buf.String()
}

func (c *Credentials) load() (*credentialsStore, error) {
	store := &credentialsStore{}
	if c.SshKey != "" {
		pkey, err := os.Open(c.SshKey)
		if err != nil {
			return nil, err
		}
		defer pkey.Close()
		privKeyContent, err := io.ReadAll(pkey)
		if err != nil {
			return nil, err
		}
		priv, err := ssh.ParsePrivateKey(privKeyContent)
		if err != nil {
			return nil, err
		}
		pubKeyContent := fmt.Sprintf("%s %s\n", priv.PublicKey().Type(), base64encode(priv.PublicKey().Marshal()))
		store.sshPrivateKey = string(privKeyContent)
		store.sshPublicKey = pubKeyContent
	}
	return store, nil
}

type credentialsStore struct {
	sshPrivateKey string
	sshPublicKey  string
}

func (c *credentialsStore) CredentialsCallback(ctx context.Context) git.CredentialsCallback {
	if c == nil {
		return func(url string, username_from_url string, allowed_types git.CredentialType) (*git.Credential, error) {
			return nil, nil
		}
	}
	logger := logger.FromContext(ctx)
	return func(url string, username_from_url string, allowed_types git.CredentialType) (*git.Credential, error) {
		logger.WithFields(logrus.Fields{
			"url":      url,
			"username": username_from_url,
		}).Debug("git.credentialsCallback")
		if c.sshPrivateKey != "" && allowed_types|git.CredTypeSshKey != 0 {
			return git.NewCredentialSSHKeyFromMemory(username_from_url, c.sshPublicKey, c.sshPrivateKey, "")
		}
		return nil, nil
	}
}
