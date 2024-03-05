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

package testssh

import (
	"crypto/ed25519"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/mattn/go-shellwords"
	"github.com/mikesmitty/edkey"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type TestServer struct {
	Port       int
	KnownHosts string
	ClientKey  string
	Url        string
	l          net.Listener
	execDelay  time.Duration
}

type envReq struct {
	Env   string
	Value string
}

type execReq struct {
	Command string
}

type exitReq struct {
	Status uint32
}

func New(workdir string) *TestServer {
	//exhaustruct:ignore
	ts := TestServer{}
	// Allocate a new listening port
	//exhaustruct:ignore
	ts.l, _ = net.ListenTCP("tcp", &net.TCPAddr{})
	ts.Port = ts.l.Addr().(*net.TCPAddr).Port

	// Setup a private key for the server and write a known hosts file
	_, servPriv, _ := ed25519.GenerateKey(nil)
	ps, _ := ssh.NewSignerFromSigner(servPriv)
	kh := knownhosts.Line([]string{fmt.Sprintf("127.0.0.1")}, ps.PublicKey())
	ts.KnownHosts = filepath.Join(workdir, "known_hosts")
	os.WriteFile(ts.KnownHosts, []byte(kh), 0644)
	//exhaustruct:ignore
	sc := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			//exhaustruct:ignore
			return &ssh.Permissions{}, nil
		},
	}
	sc.AddHostKey(ps)

	// Setup a private key for the client and write it to an openssh compatible file
	_, clientPriv, _ := ed25519.GenerateKey(nil)
	ts.ClientKey = filepath.Join(workdir, "id_ed25519")
	key := pem.EncodeToMemory(&pem.Block{
		Headers: nil,
		Type:    "OPENSSH PRIVATE KEY",
		Bytes:   edkey.MarshalED25519PrivateKey(clientPriv),
	})
	os.WriteFile(ts.ClientKey, key, 0600)

	ts.Url = fmt.Sprintf("ssh://git@127.0.0.1:%d/.", ts.Port)

	go func() {
		for {
			con, err := ts.l.Accept()
			if err != nil {
				fmt.Printf("testssh: err %q\n", err)
				return
			}
			go ts.handleConn(con, workdir, sc)
		}
	}()
	return &ts
}

func (ts *TestServer) handleConn(con net.Conn, workdir string, sc *ssh.ServerConfig) {
	defer con.Close()
	sCon, chans, reqs, err := ssh.NewServerConn(con, sc)
	if err != nil {
		fmt.Printf("testssh: err %q\n", err)
		return
	}
	defer sCon.Close()
	go ssh.DiscardRequests(reqs)
	for newch := range chans {
		if newch.ChannelType() != "session" {
			newch.Reject(ssh.UnknownChannelType, "only channel type session is allowed")
		}
		ch, reqs, err := newch.Accept()
		if err != nil {
			fmt.Printf("testssh: accept err %q\n", err)
			return
		}
		env := []string{}
		for req := range reqs {
			switch req.Type {
			case "env":
				var payload envReq
				ssh.Unmarshal(req.Payload, &payload)
				env = append(env, fmt.Sprintf("%s=%s", payload.Env, payload.Value))
			case "exec":
				var payload execReq
				ssh.Unmarshal(req.Payload, &payload)
				args, _ := shellwords.Parse(payload.Command)
				if args[0] != "git-upload-pack" && args[0] != "git-receive-pack" {
					fmt.Printf("testssh: illegal command: %q\n", args[0])
					req.Reply(false, nil)
					ch.Close()
					return
				}
				args[1] = filepath.Join(workdir, args[1])
				cmd := exec.Command(args[0], args[1:len(args)]...)
				cmd.Env = env
				stdin, _ := cmd.StdinPipe()
				stdout, _ := cmd.StdoutPipe()
				stderr, _ := cmd.StderrPipe()
				go io.Copy(stdin, ch)
				time.Sleep(ts.execDelay)
				cmd.Start()
				req.Reply(true, nil)
				_, _ = io.Copy(ch, stdout)
				_, _ = io.Copy(ch.Stderr(), stderr)
				err = cmd.Wait()
				if err != nil {
					fmt.Printf("testssh: run err %q\n", err)
				}
				ch.SendRequest("exit-status", false, ssh.Marshal(&exitReq{Status: uint32(cmd.ProcessState.ExitCode())}))
				ch.Close()
			default:
				fmt.Printf("testssh: illegal req: %q\n", req.Type)
				ch.Close()
			}

		}
	}
}

func (ts *TestServer) DelayExecs(dr time.Duration) {
	ts.execDelay = dr
}

func (ts *TestServer) Close() {
	ts.l.Close()
}
