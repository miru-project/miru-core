package golang

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rt "github.com/miru-project/miru-core/pkg/extension/golang/runtime"
	sdk "github.com/miru-project/miru-core/pkg/extension/golang/sdk"
	"github.com/miru-project/miru-core/pkg/network"
)

// statePersistenceExtSource is a synthetic Go extension holding its state in
// PACKAGE-LEVEL VARIABLES only -- no cache API anywhere (the JavaScript
// runtime's model: the host captures the state after every call and restores
// it before the next one, even though every call runs a FRESH VM).
//
// All state access happens through the real entry points, because those are
// the host-call boundary the capture brackets are injected into. Latest acts
// as the verb dispatcher (page = verb) so the tests can drive store / check /
// mutate / verify sequences through fresh compiles.
//
// The fixture deliberately mixes every interesting kind of state:
//   - primitives, composites and imported types (stable type identity)
//   - structs/slices/maps DECLARED in the extension source (fresh type
//     identity per compile -- the hard case for cross-call state)
//   - an inferred variable (var x = 5) and one inferred from a composite
//   - a function-typed variable and a function-call-initialized variable
//     (JSON cannot represent the first, the type of the second is
//     unknowable: capture must skip both silently instead of breaking the
//     other variables)
//
// __PKG__ is replaced with the package name at boot so tests get isolated
// state stores.
const statePersistenceExtSource = `// ==MiruExtension==
// @name         State Persistence
// @version      v0.0.1
// @author       test
// @lang         en
// @license      MIT
// @package      __PKG__
// @type         bangumi
// @webSite      https://example.com
// ==/MiruExtension==

package __PKG__

import (
	"fmt"
	"strings"
	"time"

	sdk "github.com/miru-project/miru-core/pkg/extension/golang/sdk"
)

type Item struct {
	Name string
	N    int
	Tags []string
}

type Nested struct {
	Items []Item
	When  time.Time
	Meta  map[string]string
}

// Profile is edited PROPERTY BY PROPERTY (in place, never reassigned) to
// prove the capture sees field-level mutations, not just whole-variable
// assignments.
type Profile struct {
	Name  string
	Level int
	Tags  []string
	Inner Inner
}

type Inner struct {
	Flag bool
	Note string
}

// maxLevel is a package-level CONST: consts are immutable, so they are never
// captured -- the runtime must simply leave them alone and they read back
// identically in every compile.
const maxLevel = 99

var profile Profile

var (
	token   string
	counter int
	item    Item
	items   []Item
	mitems  map[string]Item
	nested  Nested
	sdkItem sdk.ExtensionListItem
	when    time.Time
	fn      func() int
)

var inferredCount = 5
var inferredItems = []Item{{Name: "inferred"}}
var callInitVar = someInit()

func someInit() string { return "fresh" }

var lastkw string
var kwcount int

func Load() {
	token = "from-load"
}

// Latest is the verb dispatcher the tests drive: page selects the operation.
// Every call is a FRESH compile; all state flows through the package vars.
func Latest(pkg string, page int) ([]sdk.ExtensionListItem, error) {
	switch page {
	case 1: // seed every kind of state
		token = "hello"
		counter = 42
		item = Item{Name: "one", N: 7, Tags: []string{"t"}}
		items = []Item{{Name: "two", N: 2}, {Name: "three", N: 3}}
		mitems = map[string]Item{"k": {Name: "four"}}
		nested = Nested{
			Items: []Item{{Name: "five", N: 5}},
			When:  time.Unix(1700000000, 0).UTC(),
			Meta:  map[string]string{"page": "1"},
		}
		sdkItem = sdk.ExtensionListItem{Title: "six", URL: "u", Cover: "c"}
		when = time.Unix(1700000000, 0).UTC()
		inferredCount = 99
		inferredItems = []Item{{Name: "inferred-set"}}
		return nil, nil
	case 2: // read everything back and report
		verdicts := []string{
			fmt.Sprintf("token=%v", token == "hello"),
			fmt.Sprintf("counter=%v", counter == 42),
			fmt.Sprintf("item=%s %d %s", item.Name, item.N, item.Tags[0]),
			fmt.Sprintf("items=%d %s %s", len(items), items[0].Name, items[1].Name),
			fmt.Sprintf("mitems=%s", mitems["k"].Name),
			fmt.Sprintf("nested=%d %s %d %s", len(nested.Items), nested.Items[0].Name, nested.When.Unix(), nested.Meta["page"]),
			fmt.Sprintf("sdkitem=%s %s %s", sdkItem.Title, sdkItem.URL, sdkItem.Cover),
			fmt.Sprintf("when=%d", when.Unix()),
			fmt.Sprintf("inferredCount=%v", inferredCount == 99),
			fmt.Sprintf("inferredItems=%s", inferredItems[0].Name),
			fmt.Sprintf("callInitVar=%s", callInitVar),
		}
		return []sdk.ExtensionListItem{{Title: strings.Join(verdicts, "|")}}, nil
	case 3: // mutate: grow the slice
		items = append(items, Item{Name: "appended", N: len(items)})
		return []sdk.ExtensionListItem{{Title: fmt.Sprintf("len=%d", len(items))}}, nil
	case 4: // verify the mutation survived
		if len(items) != 3 {
			return []sdk.ExtensionListItem{{Title: fmt.Sprintf("MUTATION-FAIL len=%d", len(items))}}, nil
		}
		last := items[len(items)-1]
		return []sdk.ExtensionListItem{{Title: fmt.Sprintf("mutation=%s %d", last.Name, last.N)}}, nil
	case 5: // verify the state written by the failed Watch call survived
		return []sdk.ExtensionListItem{{Title: fmt.Sprintf("after-fail=%s %d", item.Name, item.N)}}, nil
	case 6: // report the raw token (Load() flow)
		return []sdk.ExtensionListItem{{Title: fmt.Sprintf("token=%s", token)}}, nil
	case 7: // seed the in-place-edit struct
		profile = Profile{Name: "seed", Level: 1, Tags: []string{"a"}, Inner: Inner{Flag: false, Note: "orig"}}
		items = []Item{{Name: "elem0"}}
		return []sdk.ExtensionListItem{{Title: "seeded"}}, nil
	case 8: // EDIT properties in place (no whole-variable reassign)
		profile.Name = "edited"
		profile.Level = profile.Level + 5
		profile.Tags = append(profile.Tags, "b")
		// Whole nested-field replace (1 level): supported.
		profile.Inner = Inner{Flag: true, Note: "edited-inner"}
		// Slice-ELEMENT struct field write: supported.
		items[0].Name = "elem-edited"
		// NOTE: a 2-level chain on a package var (profile.Inner.Flag = true)
		// is silently dropped by the Scriggo VM itself (write-back bug,
		// reproduced uninstrumented); nested edits must go through a local:
		inner := profile.Inner
		inner.Note = "edited-via-local"
		profile.Inner = inner
		return []sdk.ExtensionListItem{{Title: fmt.Sprintf("edited level=%d", profile.Level)}}, nil
	case 9: // read the struct back in a later compile: did the edits persist?
		return []sdk.ExtensionListItem{{Title: fmt.Sprintf(
			"name=%s level=%d tags=%v inner=%v %s elem=%s maxLevel=%d",
			profile.Name, profile.Level, profile.Tags, profile.Inner.Flag, profile.Inner.Note, items[0].Name, maxLevel,
		)}}, nil
	}
	return nil, nil
}

// Watch writes state and then fails the call; the deferred capture must still
// persist what was written before the failure.
func Watch(pkg, url string) (any, error) {
	item = Item{Name: "pre-failure", N: 1}
	return nil, fmt.Errorf("boom: call failed after writing state")
}

// Search drives the state flow through the REAL endpoint dispatch: the first
// call seeds package variables, a later call recovers them and builds its
// result from them.
func Search(pkg, kw string, page int, filter sdk.Filter) ([]sdk.ExtensionListItem, error) {
	if lastkw == kw && kwcount > 0 {
		return []sdk.ExtensionListItem{{Title: fmt.Sprintf("cached %s %d", kw, kwcount), URL: "https://example.com"}}, nil
	}
	kwcount = len(kw)
	lastkw = kw
	return []sdk.ExtensionListItem{{Title: "fresh " + kw, URL: "https://example.com"}}, nil
}
`

// bootStatePersistence writes the fixture into a fresh ExtensionDir under the
// given package name and clears any persisted state from earlier tests.
func bootStatePersistence(t *testing.T, pkg string) {
	t.Helper()
	dir := t.TempDir()
	src := strings.ReplaceAll(statePersistenceExtSource, "__PKG__", pkg)
	if err := os.WriteFile(filepath.Join(dir, pkg+".go"), []byte(src), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	ExtensionDir = dir
	rt.DeleteCache(pkg)
	t.Cleanup(func() { rt.DeleteCache(pkg) })
}

// latestVerb invokes the Latest entry point (a fresh compile per call) and
// returns the Title of the single result row.
func latestVerb(t *testing.T, pkg string, page int) string {
	t.Helper()
	res, err := callExtension(pkg, "Latest", pkg, page)
	if err != nil {
		t.Fatalf("Latest(page=%d): %v", page, err)
	}
	items, ok := res.([]sdk.ExtensionListItem)
	if !ok {
		t.Fatalf("Latest(page=%d): expected []sdk.ExtensionListItem, got %T", page, res)
	}
	if len(items) != 1 {
		t.Fatalf("Latest(page=%d): expected 1 row, got %d", page, len(items))
	}
	return items[0].Title
}

// TestStatePersistenceAcrossCalls is the core parity test with the JavaScript
// runtime: state written into package variables during one call (one FRESH
// compile) is recovered and fully usable in the next call (another FRESH
// compile) -- for primitives, composites, imported sdk types, time.Time, and
// structs declared in the extension source itself.
func TestStatePersistenceAcrossCalls(t *testing.T) {
	const pkg = "statepersist1"
	bootStatePersistence(t, pkg)

	if _, err := callExtension(pkg, "Latest", pkg, 1); err != nil {
		t.Fatalf("seed: %v", err)
	}
	got := latestVerb(t, pkg, 2)
	want := strings.Join([]string{
		"token=true",
		"counter=true",
		"item=one 7 t",
		"items=2 two three",
		"mitems=four",
		"nested=1 five 1700000000 1",
		"sdkitem=six u c",
		"when=1700000000",
		"inferredCount=true",
		"inferredItems=inferred-set",
		"callInitVar=fresh",
	}, "|")
	if got != want {
		t.Fatalf("state verdict:\n got %q\nwant %q", got, want)
	}
}

// TestStatePersistenceLoadSeedsState mirrors the JavaScript load() flow: state
// written by the Load entry point is visible to every later call.
func TestStatePersistenceLoadSeedsState(t *testing.T) {
	const pkg = "statepersist2"
	bootStatePersistence(t, pkg)

	if _, err := callExtension(pkg, "Load"); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := latestVerb(t, pkg, 6); got != "token=from-load" {
		t.Fatalf("Load-seeded token = %q, want %q", got, "token=from-load")
	}
}

// TestStatePersistenceMutationAcrossCalls verifies the full recover -> use ->
// write -> recover cycle: a slice grown in one call keeps its growth in the
// next call.
func TestStatePersistenceMutationAcrossCalls(t *testing.T) {
	const pkg = "statepersist3"
	bootStatePersistence(t, pkg)

	if _, err := callExtension(pkg, "Latest", pkg, 1); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if got := latestVerb(t, pkg, 3); got != "len=3" {
		t.Fatalf("mutate = %q, want %q", got, "len=3")
	}
	if got := latestVerb(t, pkg, 4); got != "mutation=appended 2" {
		t.Fatalf("check mutation = %q, want %q", got, "mutation=appended 2")
	}
}

// TestStatePersistenceSurvivesFailedCall verifies that state written before a
// call fails still reaches the next call -- the Go counterpart of the JS
// runtime's capture-on-failure guarantee (the save is deferred, so it runs on
// the error path too).
func TestStatePersistenceSurvivesFailedCall(t *testing.T) {
	const pkg = "statepersist4"
	bootStatePersistence(t, pkg)

	_, err := callExtension(pkg, "Watch", pkg, "u")
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("Watch: expected the simulated failure, got %v", err)
	}
	if got := latestVerb(t, pkg, 5); got != "after-fail=pre-failure 1" {
		t.Fatalf("check after fail = %q, want %q", got, "after-fail=pre-failure 1")
	}
}

// TestStatePersistenceThroughSearch drives the state flow through the REAL
// endpoint dispatch (two Search calls, each a fresh compile): the second call
// recovers what the first one seeded and builds its result from it.
func TestStatePersistenceThroughSearch(t *testing.T) {
	const pkg = "statepersist5"
	bootStatePersistence(t, pkg)
	network.Init()

	items, err := Search(pkg, 1, "dandadan", nil)
	if err != nil {
		t.Fatalf("Search 1: %v", err)
	}
	if len(items) != 1 || items[0].Title != "fresh dandadan" {
		t.Fatalf("Search 1 items = %+v", items)
	}
	items, err = Search(pkg, 1, "dandadan", nil)
	if err != nil {
		t.Fatalf("Search 2: %v", err)
	}
	if len(items) != 1 || items[0].Title != "cached dandadan 8" {
		t.Fatalf("Search 2 items = %+v, want recovered state", items)
	}
}

// TestStatePersistenceInPlaceStructEdit verifies the capture sees PROPERTY
// level mutations of a struct package variable: the entry point edits the
// struct's fields in place (never reassigning the variable), the runtime
// captures the struct automatically after the call, and the next compile reads
// the edited fields back. The Level is bumped on two consecutive edit calls,
// proving the edit -> capture -> restore -> edit cycle works repeatedly. A
// package-level const coexists untouched.
func TestStatePersistenceInPlaceStructEdit(t *testing.T) {
	const pkg = "statepersist6"
	bootStatePersistence(t, pkg)

	if got := latestVerb(t, pkg, 7); got != "seeded" {
		t.Fatalf("seed = %q, want %q", got, "seeded")
	}
	// First edit round: properties mutated in place inside the entry block.
	if got := latestVerb(t, pkg, 8); got != "edited level=6" {
		t.Fatalf("edit 1 = %q, want %q", got, "edited level=6")
	}
	// Second edit round in ANOTHER fresh compile: the previous edits must have
	// been restored first, so Level continues from 6 to 11.
	if got := latestVerb(t, pkg, 8); got != "edited level=11" {
		t.Fatalf("edit 2 = %q, want %q", got, "edited level=11")
	}
	// Read back in yet another compile: every edited property persisted, the
	// struct round-tripped normally, and the const is intact.
	got := latestVerb(t, pkg, 9)
	want := "name=edited level=11 tags=[a b b] inner=true edited-via-local elem=elem-edited maxLevel=99"
	if got != want {
		t.Fatalf("read back:\n got %q\nwant %q", got, want)
	}
}

// TestInstrumentStateCaptureEdges pins the safety properties of the source
// instrumentation: sources that cannot be parsed, that declare __miru_
// identifiers, or that have no capturable package variables are returned
// unchanged, and a capturable source gets the entry bracket plus the typed
// bridge (referencing the persisted keys and copying through a local).
func TestInstrumentStateCaptureEdges(t *testing.T) {
	broken := "package x\nfunc Broken( {"
	if got := instrumentStateCapture("x", broken); got != broken {
		t.Fatal("unparseable source must be returned unchanged")
	}

	collide := "package x\nvar __miru_tmp int\nfunc Load() {}\n"
	if got := instrumentStateCapture("x", collide); got != collide {
		t.Fatal("sources declaring __miru_ identifiers must not be instrumented")
	}

	noVars := "package x\nfunc Load() {}\n"
	if got := instrumentStateCapture("x", noVars); got != noVars {
		t.Fatal("sources without package variables must not be instrumented")
	}

	withVars := "package x\n\nimport \"time\"\n\nvar token string = \"s\"\nvar when time.Time\nvar fn func() int\n\nfunc Load() {}\n"
	got := instrumentStateCapture("x", withVars)
	if !strings.Contains(got, "__miru_state_load_all()") ||
		!strings.Contains(got, "__miru_state_save_all()") {
		t.Fatalf("entry point not bracketed:\n%s", got)
	}
	if !strings.Contains(got, "func __miru_state_load_token()") ||
		!strings.Contains(got, `MiruStateGet("x", "__miru_var.token", &__miru_tmp)`) ||
		!strings.Contains(got, `MiruStatePut("x", "__miru_var.token", &__miru_tmp)`) {
		t.Fatalf("generated bridge incomplete:\n%s", got)
	}
	if !strings.Contains(got, "var __miru_tmp string = token") ||
		!strings.Contains(got, "var __miru_tmp time.Time = when") {
		t.Fatalf("bridge must copy through a local of the declared type:\n%s", got)
	}
	// Function-typed variables are excluded from the bridge entirely.
	if strings.Contains(got, "__miru_var.fn") {
		t.Fatalf("function-typed variable must not be captured:\n%s", got)
	}
	// The printed result must still be valid Go.
	if _, err := parser.ParseFile(token.NewFileSet(), "check.go", got, 0); err != nil {
		t.Fatalf("instrumented source does not parse: %v\n%s", err, got)
	}
}
