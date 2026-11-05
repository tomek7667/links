package json

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tomek7667/links/internal/domain"
)

func newTestClient(t *testing.T) *Client {
	t.Helper()
	return &Client{Path: filepath.Join(t.TempDir(), "links.db.json"), db: Db{Links: []domain.Link{}}}
}

func titles(links []domain.Link) []string {
	out := make([]string, len(links))
	for i, l := range links {
		out[i] = l.Title
	}
	return out
}

func TestLoadMigratesLinksWithoutIDs(t *testing.T) {
	c := newTestClient(t)
	legacy := `{"links":[{"title":"A","url":"http://a"},{"title":"B","url":"http://b"}]}`
	if err := os.WriteFile(c.Path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := c.load(); err != nil {
		t.Fatal(err)
	}
	links := c.Links()
	if len(links) != 2 || links[0].ID == "" || links[0].ID == links[1].ID {
		t.Fatalf("expected two links with distinct ids, got %+v", links)
	}
	// the migration is persisted
	again := &Client{Path: c.Path}
	if err := again.load(); err != nil {
		t.Fatal(err)
	}
	if again.Links()[0].ID != links[0].ID {
		t.Fatal("migrated ids were not saved")
	}
}

func TestReplaceHostKeepsPortAndPath(t *testing.T) {
	c := newTestClient(t)
	for _, u := range []string{"http://192.168.1.20:8096/web", "https://192.168.1.20", "http://192.168.1.200:80", "http://other:1"} {
		if _, err := c.CreateLink(domain.Link{Title: u, Url: u}, -1); err != nil {
			t.Fatal(err)
		}
	}
	n, err := c.ReplaceHost("192.168.1.20", "{ip}")
	if err != nil || n != 2 {
		t.Fatalf("ReplaceHost = %d, %v; want 2", n, err)
	}
	got := []string{}
	for _, l := range c.Links() {
		got = append(got, l.Url)
	}
	want := []string{"http://{ip}:8096/web", "https://{ip}", "http://192.168.1.200:80", "http://other:1"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("urls = %v; want %v", got, want)
		}
	}
	if _, err := c.ReplaceHost("other", "bad/host"); err == nil {
		t.Fatal("expected invalid host to be rejected")
	}
}

func TestReorderCreateAtAndImport(t *testing.T) {
	c := newTestClient(t)
	a, _ := c.CreateLink(domain.Link{Title: "A", Url: "http://a"}, -1)
	b, _ := c.CreateLink(domain.Link{Title: "B", Url: "http://b"}, -1)
	if _, err := c.CreateLink(domain.Link{Title: "Z", Url: "http://z"}, 0); err != nil {
		t.Fatal(err)
	}
	if got := titles(c.Links()); got[0] != "Z" {
		t.Fatalf("CreateLink at 0: got %v", got)
	}

	// ids missing from the order keep their relative order at the end
	if err := c.ReorderLinks([]string{b.ID, a.ID}); err != nil {
		t.Fatal(err)
	}
	if got := titles(c.Links()); got[0] != "B" || got[1] != "A" || got[2] != "Z" {
		t.Fatalf("reorder: got %v", got)
	}

	added, skipped, err := c.ImportLinks([]domain.Link{{Title: "A again", Url: "http://a"}, {Title: "C", Url: "c.lan"}, {Title: "", Url: "x"}}, false)
	if err != nil || added != 1 || skipped != 2 {
		t.Fatalf("merge import = %d added, %d skipped, %v", added, skipped, err)
	}
	added, _, err = c.ImportLinks([]domain.Link{{Title: "Only", Url: "http://only"}}, true)
	if err != nil || added != 1 || len(c.Links()) != 1 {
		t.Fatalf("replace import left %v", titles(c.Links()))
	}
}

func TestFailedValidationLeavesStateUntouched(t *testing.T) {
	c := newTestClient(t)
	l, _ := c.CreateLink(domain.Link{Title: "A", Url: "http://a"}, -1)
	if _, err := c.UpdateLink(l.ID, domain.Link{Title: "A", Url: "javascript:alert(1)"}); err == nil {
		t.Fatal("expected javascript: url to be rejected")
	}
	if err := c.DeleteLink("missing"); err == nil {
		t.Fatal("expected not found")
	}
	if got := c.Links(); len(got) != 1 || got[0].Url != "http://a" {
		t.Fatalf("state changed: %+v", got)
	}
}
