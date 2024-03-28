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
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"

	"github.com/freiheit-com/kuberpult/pkg/logger"
	git "github.com/libgit2/git2go/v34"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
)

type Credentials struct {
	SshKey string
}

func base64encode(c []byte) string {
	var buf bytes.Buffer
	enc := base64.NewEncoder(base64.StdEncoding, &buf)
	enc.Write(c) //nolint: errcheck
	enc.Close()
	return buf.String()
}

func (c *Credentials) load() (*credentialsStore, error) {
	store := &credentialsStore{
		sshPrivateKey: "",
		sshPublicKey:  "",
	}
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
		logger.Debug("git.credentialsCallback",
			zap.String("url", url),
			zap.String("username", username_from_url),
		)
		if c.sshPrivateKey != "" && allowed_types|git.CredTypeSshKey != 0 {
			return git.NewCredentialSSHKeyFromMemory(username_from_url, c.sshPublicKey, c.sshPrivateKey, "")
		}
		return nil, nil
	}
}
