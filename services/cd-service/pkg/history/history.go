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
package history

import (
	"fmt"
	"strings"

	git "github.com/libgit2/git2go/v33"
)

type History struct {
	repository *git.Repository
	commits    map[git.Oid]*CommitHistory
}

type NotExists struct {
	Path  []string
	inner error
}

func (n *NotExists) Error() string {
	return fmt.Sprintf("(pkg/history) not exists: %q", strings.Join(n.Path, "/"))
}

func (n *NotExists) Unwrap() error {
	return n.inner
}

var (
	_ error = (*NotExists)(nil)
)

// Returns the first commit after a certain path was changed.
func (h *History) Change(from *git.Commit, path []string) (*git.Commit, error) {
	var err error
	ch := h.commits[*from.Id()]
	if ch == nil {
		ch, err = NewCommitHistory(h.repository, from)
		if err != nil {
			return nil, err
		}
		h.commits[*from.Id()] = ch
	}
	return ch.Change(path)
}

func NewHistory(repo *git.Repository) *History {
	if repo == nil {
		panic("nil repository passed to NewHistory")
	}
	return &History{
		repository: repo,
		commits:    map[git.Oid]*CommitHistory{},
	}
}

type CommitHistory struct {
	repository *git.Repository
	commit     *git.Commit
	current    *git.Commit
	root       *treeNode
}

func NewCommitHistory(repo *git.Repository, commit *git.Commit) (*CommitHistory, error) {
	entry := git.TreeEntry{
		Id:       commit.TreeId(),
		Type:     git.ObjectTree,
		Filemode: git.FilemodeTree,
	}
	root, err := newTreeNode(repo, &entry)
	if err != nil {
		return nil, err
	}
	return &CommitHistory{
		repository: repo,
		commit:     commit,
		current:    commit,
		root:       root,
	}, nil
}

func (h *CommitHistory) Change(path []string) (*git.Commit, error) {
	for {
		commit, err := h.root.seek(h.repository, 0, path)
		if err != nil {
			return nil, err
		}
		if commit != nil {
			return commit, nil
		}
		if h.current != nil {
			parent := h.current.Parent(0)
			parentTreeId := &oidZero
			if parent != nil {
				parentTreeId = parent.TreeId()
			}
			h.root.push(h.current, &git.TreeEntry{
				Id:       parentTreeId,
				Type:     git.ObjectTree,
				Filemode: git.FilemodeTree,
			})
			h.current = parent
		}
	}
}

type treeEntry struct {
	name string
	oid  *git.Oid
}

type queueEntry struct {
	commit *git.Commit
	entry  *git.TreeEntry
}

type treeNode struct {
	entry    *git.TreeEntry
	children map[string]*treeNode
	queue    []queueEntry
	commit   *git.Commit
}

func newTreeNode(r *git.Repository, t *git.TreeEntry) (*treeNode, error) {
	var children map[string]*treeNode
	if t.Type == git.ObjectTree {
		tree, err := r.LookupTree(t.Id)
		if err != nil {
			return nil, fmt.Errorf("error looking up tree %q: %w", t.Id, err)
		}
		children = make(map[string]*treeNode, tree.EntryCount())
		for i := uint64(0); i < tree.EntryCount(); i++ {
			entry := tree.EntryByIndex(i)
			child, err := newTreeNode(r, entry)
			if err != nil {
				return nil, err
			}
			children[entry.Name] = child

		}
	}
	return &treeNode{
		entry:    t,
		children: children,
		queue:    []queueEntry{},
	}, nil
}

func (t *treeNode) work(r *git.Repository) error {
	if len(t.queue) == 0 {
		return nil
	}
	queue := t.queue
	t.queue = []queueEntry{}
	for i, q := range queue {
		if q.entry.Id == &oidZero {
			for _, node := range t.children {
				node.push(q.commit, nil)
			}
		} else {
			tree, err := r.LookupTree(q.entry.Id)
			if err != nil {
				t.queue = queue[i:]
				return err
			}
			for name, node := range t.children {
				entry := tree.EntryByName(name)
				node.push(q.commit, entry)
			}
		}
	}
	return nil
}

func (t *treeNode) seek(r *git.Repository, i int, path []string) (*git.Commit, error) {
	err := t.work(r)
	if err != nil {
		return nil, err
	}
	if len(path) == i {
		return t.commit, nil
	}
	needle := path[i]
	child := t.children[needle]
	if child == nil {
		return nil, &NotExists{Path: path}
	}
	return child.seek(r, i+1, path)
}

var oidZero git.Oid = [20]byte{0}

func (t *treeNode) push(commit *git.Commit, entry *git.TreeEntry) {
	if entry == nil {
		entry = &git.TreeEntry{Id: &oidZero}
	}
	if t.commit == nil {
		if !t.entry.Id.Equal(entry.Id) {
			t.commit = commit
		}
	}
	if t.entry.Filemode == git.FilemodeTree {
		if len(t.queue) == 0 {
			if t.entry.Id.Equal(entry.Id) {
				return
			}
		} else if t.queue[len(t.queue)-1].entry.Id == entry.Id {
			return
		}
		t.queue = append(t.queue, queueEntry{
			commit: commit,
			entry:  entry,
		})
	}
}
