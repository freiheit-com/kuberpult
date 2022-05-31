package history

import (
	"fmt"
	"io"
	"path"

	git "github.com/libgit2/git2go/v33"
)


func writeIndex(h *History,parent *git.Commit, out io.Writer) error {
	c := h.cache
	if c == nil {
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
	_, err := fmt.Fprintf(out, "v1 %x\n", *current.Id())
	if err != nil {
		return err
	}
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
