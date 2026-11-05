package json

import (
	"fmt"
	"slices"
	"strings"

	"github.com/tomek7667/links/internal/domain"
)

func (c *Client) Links() []domain.Link {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.db.Links)
}

func indexOf(links []domain.Link, id string) int {
	return slices.IndexFunc(links, func(l domain.Link) bool { return l.ID == id })
}

// CreateLink adds a link at position at, or at the end when at is out of range.
func (c *Client) CreateLink(l domain.Link, at int) (domain.Link, error) {
	l, err := domain.NormalizeLink(l)
	if err != nil {
		return l, err
	}
	l.ID = newID()
	err = c.mutate(func(db *Db) error {
		if at < 0 || at > len(db.Links) {
			at = len(db.Links)
		}
		db.Links = slices.Insert(db.Links, at, l)
		return nil
	})
	return l, err
}

func (c *Client) UpdateLink(id string, l domain.Link) (domain.Link, error) {
	l, err := domain.NormalizeLink(l)
	if err != nil {
		return l, err
	}
	l.ID = id
	err = c.mutate(func(db *Db) error {
		idx := indexOf(db.Links, id)
		if idx == -1 {
			return domain.ErrNotFound
		}
		db.Links[idx] = l
		return nil
	})
	return l, err
}

func (c *Client) DeleteLink(id string) error {
	return c.mutate(func(db *Db) error {
		idx := indexOf(db.Links, id)
		if idx == -1 {
			return domain.ErrNotFound
		}
		db.Links = slices.Delete(db.Links, idx, idx+1)
		return nil
	})
}

// DeleteLinkByURL keeps the pre-ID API (DELETE /api/links {"url": ...}) working.
func (c *Client) DeleteLinkByURL(url string) error {
	return c.mutate(func(db *Db) error {
		idx := slices.IndexFunc(db.Links, func(l domain.Link) bool { return l.Url == url })
		if idx == -1 {
			return domain.ErrNotFound
		}
		db.Links = slices.Delete(db.Links, idx, idx+1)
		return nil
	})
}

// ReorderLinks sorts links by the given ID order. Links missing from ids keep
// their relative order and go to the end.
func (c *Client) ReorderLinks(ids []string) error {
	pos := make(map[string]int, len(ids))
	for i, id := range ids {
		if _, dup := pos[id]; !dup {
			pos[id] = i
		}
	}
	return c.mutate(func(db *Db) error {
		slices.SortStableFunc(db.Links, func(a, b domain.Link) int {
			pa, oka := pos[a.ID]
			pb, okb := pos[b.ID]
			switch {
			case oka && okb:
				return pa - pb
			case oka:
				return -1
			case okb:
				return 1
			default:
				return 0
			}
		})
		return nil
	})
}

// ReplaceHost swaps the host/IP part of every link whose host equals from
// (case-insensitive), keeping scheme, port and path. It returns how many links changed.
func (c *Client) ReplaceHost(from, to string) (int, error) {
	from, to = strings.TrimSpace(from), strings.TrimSpace(to)
	if from == "" {
		return 0, fmt.Errorf("%w: the host to replace is required", domain.ErrInvalid)
	}
	if err := domain.ValidateHost(to, true); err != nil {
		return 0, err
	}
	n := 0
	err := c.mutate(func(db *Db) error {
		for i := range db.Links {
			p := domain.SplitURL(db.Links[i].Url)
			if !strings.EqualFold(p.Host, from) {
				continue
			}
			p.Host = to
			db.Links[i].Url = p.String()
			n++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return n, nil
}

// RenameGroup moves every link in group from to group to. An empty to ungroups them.
func (c *Client) RenameGroup(from, to string) (int, error) {
	from, to = strings.TrimSpace(from), strings.TrimSpace(to)
	if len([]rune(to)) > 64 {
		return 0, fmt.Errorf("%w: group is too long (max 64 characters)", domain.ErrInvalid)
	}
	n := 0
	err := c.mutate(func(db *Db) error {
		for i := range db.Links {
			if db.Links[i].Group == from {
				db.Links[i].Group = to
				n++
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return n, nil
}

// ImportLinks adds links from a backup. With replace, existing links are dropped
// first; otherwise links whose URL already exists are skipped. Invalid entries
// are skipped and counted.
func (c *Client) ImportLinks(links []domain.Link, replace bool) (added, skipped int, err error) {
	err = c.mutate(func(db *Db) error {
		if replace {
			db.Links = []domain.Link{}
		}
		existing := make(map[string]bool, len(db.Links))
		for _, l := range db.Links {
			existing[l.Url] = true
		}
		for _, l := range links {
			nl, err := domain.NormalizeLink(l)
			if err != nil || existing[nl.Url] {
				skipped++
				continue
			}
			nl.ID = newID()
			existing[nl.Url] = true
			db.Links = append(db.Links, nl)
			added++
		}
		return nil
	})
	if err != nil {
		return 0, 0, err
	}
	return added, skipped, nil
}
