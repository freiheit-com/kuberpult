package history

import (
	"fmt"
	"io"
	"path"
	"strings"

	git "github.com/libgit2/git2go/v33"
)

func writeIndex(h *History, parent *git.Commit, out io.Writer) error {
	c := h.cache
	if c == nil {
		return nil
	}
	if parent == nil {
		return nil
	}
	// 1: seek if we have a cache already.
	current := parent
	var entry *resultNode
	for {
		entry = c.get(*current.Id())
		if entry != nil {
			break
		}
		current = current.Parent(0)
		if current == nil {
			return nil
		}
	}
	// 2: serialize the index
	return writeEntry(out, "", entry)
}

func writeEntry(out io.Writer, name string, entry *resultNode) error {
	var err error
	id := entry.loadId()
	if id != nil {
		_, err = fmt.Fprintf(out, "%x %s\n", *id, name)
		if err != nil {
			return err
		}
	}
	children := entry.childNames()
	for _, cld := range children {
		childEntry := entry.getChild(cld)
		err = writeEntry(out, path.Join(name, cld), childEntry)
		if err != nil {
			return err
		}
	}
	return nil
}

func readIndex(repository *git.Repository, cache *Cache, content []byte) (*resultNode, error) {
	lines := strings.Split(string(content), "\n")
	// first line is the commit id that was used to build that index
	if string(content) == "" || string(content) == "\n" {
		return nil, nil
	}
	p := strings.SplitN(lines[0], " ", 2)
	oid, err := git.NewOid(p[0])
	if err != nil {
		return nil, fmt.Errorf("error decoding hex string %q at line 0: %w", p[0], err)
	}
	result := cache.getOrAdd(*oid)
	for i, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		name := ""
		if len(parts) == 2 {
			name = parts[1]
		}
		oid, err := git.NewOid(parts[0])
		if err != nil {
			return nil, fmt.Errorf("error decoding hex string %q at line %d: %w", parts[0], i, err)
		}
		commit, _ := repository.LookupCommit(oid)
		result.storeAt(name, commit)
	}
	return result, nil
}
