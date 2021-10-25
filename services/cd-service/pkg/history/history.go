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

	git "github.com/libgit2/git2go/v31"
)

type History struct {
	repository *git.Repository
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
	commit, _, err := h.traverse(from, path)
	return commit, err
}

// Returns the first commit after a certain path was changed and the oid of the tree node after the change to aid subsequent iterations.
func (h *History) traverse(from *git.Commit, path []string) (*git.Commit, *git.Oid, error) {
	if len(path) == 0 {
		treeId := from.TreeId()
		current := from
		for {
			next := current.Parent(0)
			if next == nil {
				return current, nil, nil
			}
			if !treeId.Equal(next.TreeId()) {
				return current, next.TreeId(), nil
			}
			current = next
		}
	}
	head := path[0 : len(path)-1]
	rest := path[len(path)-1]
	oid, err := h.lookup(from, path)
	if err != nil {
		return nil, nil, err
	}

	current := from
	for {
		next, tree, err := h.traverse(current, head)
		if err != nil || next.Parent(0) == nil {
			return next, nil, err
		}
		if tree == nil {
			return next, nil, err
		}
		node, err := h.repository.LookupTree(tree)
		if err != nil {
			return nil, nil, err
		}
		entry := node.EntryByName(rest)
		if entry == nil {
			return next, nil, nil
		} else if !oid.Equal(entry.Id) {
			return next, entry.Id, nil
		}
		current = next.Parent(0)
	}
}

func (h *History) lookup(from *git.Commit, path []string) (*git.Oid, error) {
	current, err := from.Tree()
	if err != nil {
		return nil, err
	}
	for i, elem := range path[0 : len(path)-1] {
		node := current.EntryByName(elem)
		if node == nil {
			return nil, &NotExists{
				Path: path[0 : i+1],
			}
		}
		next, err := h.repository.Lookup(node.Id)
		if err != nil {
			return nil, err
		}
		tree, err := next.AsTree()
		if err != nil {
			return nil, &NotExists{
				Path:  path[0 : i+2],
				inner: err,
			}
		}
		current = tree
	}
	last := current.EntryByName(path[len(path)-1])
	if last == nil {
		return nil, &NotExists{
			Path: path,
		}
	}
	return last.Id, nil
}

func NewHistory(repo *git.Repository) *History {
	if repo == nil {
		panic("nil repository passed to NewHistory")
	}
	return &History{
		repository: repo,
	}
}
