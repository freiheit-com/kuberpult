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

// Main file for microservice cd-service.
package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/storage/memory"
)

func main() {
	// Filesystem abstraction based on memory
	fs := memfs.New()
	storer := memory.NewStorage()
	os.RemoveAll("./kp")
	privateKeyPath := "<PATH_TO_PRIV_KEY>"
	authMethod, err := ssh.NewPublicKeysFromFile("git", privateKeyPath, "")
	if err != nil {
		log.Fatal(err)
	}
	// if err != nil {
	// 	log.Fatal(err)
	// }
	cloneOptions := &git.CloneOptions{
		URL:             "git@github.com:freiheit-com/kuberpult.git",
		InsecureSkipTLS: false,
		Auth:            authMethod,
		Progress:        os.Stdout,
		Depth:           1,
		Tags:            git.AllTags,
		SingleBranch:    true,
		ReferenceName:   "main",
	}
	// Clones the repository into the worktree (fs) and stores all the .git
	// content into the storer
	fmt.Printf("Starting memory clone")
	t := time.Now()
	rep, err := git.Clone(storer, fs, cloneOptions)
	if err != nil {
		log.Fatal(err)
	}
	// err = rep.Fetch(&git.FetchOptions{
	// 	RefSpecs: []config.RefSpec{"+refs/tags/*:refs/tags/*"},
	// 	Auth:     authMethod,
	// })
	// if err != nil {
	// 	log.Fatal(err)
	// }
	fmt.Printf("Memory clone done in %dms\n", time.Since(t)/time.Millisecond)
	itr, _ := rep.References()
	err = itr.ForEach(func(r *plumbing.Reference) error {
		fmt.Println(r.String())
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	wt, err := rep.Worktree()
	if err != nil {
		log.Fatal(err)
	}
	// err = fs.Rename("applications", "apps2")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// _, _ = wt.Add("apps2")

	commitOps := &git.CommitOptions{
		All:               true,
		AllowEmptyCommits: true,
		Author: &object.Signature{Name: "Ahmed Noursss",
			Email: "blabla@bla.bla"},
	}
	fmt.Println("Creating commit")
	t = time.Now()
	commitId, err := wt.Commit("Rename", commitOps)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Commit %s done in %dms\n", commitId, time.Since(t)/time.Millisecond)
	// commit, err := rep.CommitObject(commitId)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// files, err := commit.Files()
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// files.ForEach(func(f *object.File) error {
	// 	fmt.Println(f.Name)
	// 	return nil
	// })
	// Prints the content of the CHANGELOG file from the cloned repository
	// changelog, err := fs.Open("CHANGELOG")
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// io.Copy(os.Stdout, changelog)
}
