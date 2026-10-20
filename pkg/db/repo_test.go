package db

import (
	"testing"

	"github.com/miru-project/miru-core/ent/enttest"
	"github.com/miru-project/miru-core/ext"
	_ "modernc.org/sqlite"
)

// setRepoTestClient points the package-global ent client at a private
// in-memory database for the duration of one test.
func setRepoTestClient(t *testing.T, name string) {
	t.Helper()
	client := enttest.Open(t, "sqlite3", "file:"+name+"?mode=memory&cache=shared&_fk=1&_pragma=foreign_keys(1)")
	t.Cleanup(func() {
		client.Close()
		ext.SetEntClientForTest(nil)
	})
	ext.SetEntClientForTest(client)
}

// Removing one repository must leave the others alone: the delete used to run
// without a predicate and wiped every row.
func TestRemoveExtensionRepo_LeavesOtherRepos(t *testing.T) {
	setRepoTestClient(t, "remove-one")
	seed := []struct{ name, link string }{
		{"Official", "https://raw.githubusercontent.com/appdevelpo/repo/refs/heads/miru_alpha/index.json"},
		{"Miru Project", "https://raw.githubusercontent.com/miru-project/repo/main/index.json"},
	}
	for _, r := range seed {
		if err := SetRepository(r.name, r.link); err != nil {
			t.Fatalf("seed %s: %v", r.name, err)
		}
	}
	if err := RemoveExtensionRepo(seed[0].link); err != nil {
		t.Fatalf("remove failed: %v", err)
	}
	repos, err := GetAllRepositories()
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 || repos[0].Name != "Miru Project" {
		t.Fatalf("remaining repos = %v, want only Miru Project", repos)
	}
}

// Re-adding a repo with the same link must update its name, not duplicate it.
func TestSetRepository_UpsertsByLink(t *testing.T) {
	setRepoTestClient(t, "upsert-by-link")
	if err := SetRepository("Old", "https://miru-repo.example.com/index.json"); err != nil {
		t.Fatal(err)
	}
	if err := SetRepository("New", "https://miru-repo.example.com/index.json"); err != nil {
		t.Fatal(err)
	}
	repos, err := GetAllRepositories()
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 || repos[0].Name != "New" {
		t.Fatalf("repos = %v, want one entry named New", repos)
	}
}
