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
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	billy "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/util"
	"github.com/hashicorp/golang-lru"
	git "github.com/libgit2/git2go/v33"
)

// A resultNode stores the result of calcuating the last changed commit of a file or directory.
//
// This datastructure is thread-safe a intended to be shared and cached between mutliple calculations.
type resultNode struct {
	mx        sync.Mutex
	children  map[string]*resultNode
	changedAt *git.Commit
}

func (c *resultNode) getChild(name string) *resultNode {
	if c == nil {
		return nil
	}
	c.mx.Lock()
	defer c.mx.Unlock()
	child := c.children[name]
	if child == nil {
		child = newResultNode()
		c.children[name] = child
	}
	return child
}

func (c *resultNode) store(commit *git.Commit) {
	c.mx.Lock()
	defer c.mx.Unlock()
	c.changedAt = commit
}

func (c *resultNode) storeAt(path string, commit *git.Commit) {
	if path == "" {
		c.store(commit)
	}
	idx := strings.SplitN(path, "/", 2)
	if len(idx) == 1 {
		c.getChild(path).store(commit)
	} else {
		c.getChild(idx[0]).storeAt(idx[1], commit)
	}
}

func (c *resultNode) load() *git.Commit {
	if c == nil {
		return nil
	}
	c.mx.Lock()
	defer c.mx.Unlock()
	return c.changedAt
}

func (c *resultNode) loadId() *git.Oid {
	commit := c.load()
	if commit != nil {
		return commit.Id()
	}
	return nil
}

func (c *resultNode) childNames() []string {
	if c == nil {
		return []string{}
	}
	c.mx.Lock()
	defer c.mx.Unlock()
	children := make([]string, 0, len(c.children))
	for name := range c.children {
		children = append(children, name)
	}
	sort.Strings(children)
	return children
}

func (c *resultNode) isEmpty() bool {
	if c == nil {
		return true
	}
	c.mx.Lock()
	defer c.mx.Unlock()
	return c.changedAt == nil
}

func newResultNode() *resultNode {
	return &resultNode{
		children: map[string]*resultNode{},
	}
}

// A simple LRU cache that stores the last few history results.
type Cache struct {
	roots *lru.Cache
}

func (c *Cache) getOrAdd(commitId [20]byte) *resultNode {
	if c == nil {
		return newResultNode()
	}
	node := newResultNode()
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
		cache: &Cache{
			roots: roots,
		},
	}
}

func (h *History) Of(from *git.Commit) (*CommitHistory, error) {
	return NewCommitHistory(h.repository, from, h.cache)
}

func (h *History) InjectCache(bfs billy.Filesystem, parent *git.Commit) error {
	var buf bytes.Buffer
	err := writeIndex(h, parent, &buf)
	if err != nil {
		return err
	}
	err = bfs.MkdirAll(".index", 0644)
	if err != nil {
		return err
	}
	return util.WriteFile(bfs, ".index/v1", buf.Bytes(), 0644)
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
	root, err := newTreeNode(repo, &entry, cache.getOrAdd(*commit.Id()))
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
			// try read persistend cache
			tree, err := h.current.Tree()
			if err == nil {
				err := h.readCache(tree)
				if err != nil {
					return nil, err
				}
			}
			// try get non-persistend cache
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

func (h *CommitHistory) readCache(tree *git.Tree) error {
	entry, err := tree.EntryByPath(".index/v1")
	if err != nil {
		var gErr *git.GitError
		if errors.As(err, &gErr) {
			if gErr.Code != git.ErrorCodeNotFound {
				return fmt.Errorf("error opening .index/v1: %w", err)
			}
			return nil
		} else {
			return fmt.Errorf("error opening .cache/v1: %w", err)
		}
	}
	blob, err := h.repository.LookupBlob(entry.Id)
	if err != nil {
		return fmt.Errorf("error reading .index/v1: %w", err)
	}
	_, err = readIndex(h.repository, h.cache, blob.Contents())
	if err != nil {
		return fmt.Errorf("error reading index: %w", err)
	}
	return nil
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
			child, err := newTreeNode(r, entry, writeCache.getChild(entry.Name))
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
				node.push(q.commit, entry, q.cache.getChild(name))
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
		return t.result.load(), nil
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
	if t.result.isEmpty() {
		if !t.entry.Id.Equal(entry.Id) {
			t.result.store(commit)
		}
		if cachedCommit := cache.load(); cachedCommit != nil {
			t.result.store(cachedCommit)
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
