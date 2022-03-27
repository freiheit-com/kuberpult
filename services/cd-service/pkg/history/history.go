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
	"sync"

	"github.com/hashicorp/golang-lru"
	git "github.com/libgit2/git2go/v33"
)

type resultNode struct {
	mx        sync.Mutex
	children  map[string]*resultNode
	changedAt *git.Commit
}

func (c *resultNode) get(name string) *resultNode {
	if c == nil {
		return nil
	}
	c.mx.Lock()
	defer c.mx.Unlock()
	child := c.children[name]
	if child == nil {
		child = newCacheNode()
		c.children[name] = child
	}
	return child
}

func (c *resultNode) found(commit *git.Commit) {
	c.mx.Lock()
	defer c.mx.Unlock()
	c.changedAt = commit
}

func (c *resultNode) changed() *git.Commit {
	if c == nil {
		return nil
	}
	c.mx.Lock()
	defer c.mx.Unlock()
	return c.changedAt
}

func newCacheNode() *resultNode {
	return &resultNode{
		children: map[string]*resultNode{},
	}
}

type Cache struct {
	roots *lru.Cache
}

func (c *Cache) put(commitId [20]byte) *resultNode {
	if c == nil {
		return newCacheNode()
	}
	node := newCacheNode()
	previous, ok, _ := c.roots.PeekOrAdd(commitId, node)
	if ok {
		return previous.(*resultNode)
	} else {
		return node
	}
}
func (c *Cache) get(commitId [20]byte) *resultNode {
	if c == nil {
		return nil
	}
	root, ok := c.roots.Get(commitId)
	if ok {
		return root.(*resultNode)
	}
	return nil
}

type History struct {
	repository *git.Repository
	commits    map[git.Oid]*CommitHistory
	cache      *Cache
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
		ch, err = NewCommitHistory(h.repository, from, h.cache)
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
	roots, err := lru.New(10)
	if err != nil {
		// lru.New returns an error when size is negative
		panic(err)
	}
	return &History{
		repository: repo,
		commits:    map[git.Oid]*CommitHistory{},
		cache: &Cache{
			roots: roots,
		},
	}
}

func (h *History) Of(from *git.Commit) (*CommitHistory, error) {
	return NewCommitHistory(h.repository, from, h.cache)
}

type CommitHistory struct {
	repository *git.Repository
	commit     *git.Commit
	current    *git.Commit
	root       *treeNode
	cache      *Cache
}

func NewCommitHistory(repo *git.Repository, commit *git.Commit, cache *Cache) (*CommitHistory, error) {
	entry := git.TreeEntry{
		Id:       commit.TreeId(),
		Type:     git.ObjectTree,
		Filemode: git.FilemodeTree,
	}
	root, err := newTreeNode(repo, &entry, cache.put(*commit.Id()))
	if err != nil {
		return nil, err
	}
	return &CommitHistory{
		repository: repo,
		commit:     commit,
		current:    commit,
		root:       root,
		cache:      cache,
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
			cache := h.cache.get(*h.current.Id())
			h.root.push(h.current, &git.TreeEntry{
				Id:       parentTreeId,
				Type:     git.ObjectTree,
				Filemode: git.FilemodeTree,
			}, cache)
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
	cache  *resultNode
}

type treeNode struct {
	entry    *git.TreeEntry
	children map[string]*treeNode
	queue    []queueEntry
	result   *resultNode
}

func newTreeNode(r *git.Repository, t *git.TreeEntry, writeCache *resultNode) (*treeNode, error) {
	var children map[string]*treeNode
	if t.Type == git.ObjectTree {
		tree, err := r.LookupTree(t.Id)
		if err != nil {
			return nil, fmt.Errorf("error looking up tree %q: %w", t.Id, err)
		}
		children = make(map[string]*treeNode, tree.EntryCount())
		for i := uint64(0); i < tree.EntryCount(); i++ {
			entry := tree.EntryByIndex(i)
			child, err := newTreeNode(r, entry, writeCache.get(entry.Name))
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
		result:   writeCache,
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
				node.push(q.commit, nil, nil)
			}
		} else {
			tree, err := r.LookupTree(q.entry.Id)
			if err != nil {
				t.queue = queue[i:]
				return err
			}
			for name, node := range t.children {
				entry := tree.EntryByName(name)
				node.push(q.commit, entry, q.cache.get(name))
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
		return t.result.changed(), nil
	}
	needle := path[i]
	child := t.children[needle]
	if child == nil {
		return nil, &NotExists{Path: path}
	}
	return child.seek(r, i+1, path)
}

var oidZero git.Oid = [20]byte{0}
var zeroEntry git.TreeEntry = git.TreeEntry{Id: &oidZero}

func (t *treeNode) push(commit *git.Commit, entry *git.TreeEntry, cache *resultNode) {
	if entry == nil {
		entry = &zeroEntry
	}
	if t.result.changed() == nil {
		if !t.entry.Id.Equal(entry.Id) {
			t.result.found(commit)
		}
		if c := cache.changed(); c != nil {
			t.result.found(commit)
		}
	}
	if t.entry.Filemode == git.FilemodeTree {
		if cache == nil {
			if len(t.queue) == 0 {
				if t.entry.Id.Equal(entry.Id) {
					return
				}
			} else if t.queue[len(t.queue)-1].entry.Id == entry.Id {
				return
			}
		}
		t.queue = append(t.queue, queueEntry{
			commit: commit,
			entry:  entry,
			cache:  cache,
		})
	}
}
