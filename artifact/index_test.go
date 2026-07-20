package artifact

import "testing"

func TestIndex_FindByKindRefs(t *testing.T) {
	ix := &Index{Artifacts: []Entry{
		{Ref: "uploads/envelope.json", Tags: map[string]string{"kind": "result"}},
		{Ref: "screenshots/1.png", Tags: map[string]string{"kind": "screenshot"}},
	}}
	if e, ok := ix.Find("uploads/envelope.json"); !ok || e.Tags["kind"] != "result" {
		t.Fatalf("Find result = %v,%v", e, ok)
	}
	if _, ok := ix.Find("nope"); ok {
		t.Fatal("Find nope should be false")
	}
	if got := ix.ByKind("screenshot"); len(got) != 1 || got[0].Ref != "screenshots/1.png" {
		t.Fatalf("ByKind = %v", got)
	}
	if got := ix.Refs(); len(got) != 2 {
		t.Fatalf("Refs = %v", got)
	}
}
