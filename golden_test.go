package sqlx_test

import (
	"flag"
	"os"
	"os/exec"
	"testing"
)

var updateGolden = flag.Bool("update", false, "rewrite testdata/golden.txt from the current output")

// TestGolden pins the exact output of BindNamed, In and Rebind for the corpus
// in golden_corpus_test.go. Run `go test -run TestGolden -update` after an
// intended change and review the diff of testdata/golden.txt.
// testdata/golden-v1.4.0.txt is the same corpus rendered by upstream v1.4.0;
// diffing the two files lists every output the fork changed on purpose.
func TestGolden(t *testing.T) {
	const path = "testdata/golden.txt"
	got := goldenRender()
	if *updateGolden {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(want) == got {
		return
	}
	tmp := t.TempDir() + "/golden.txt"
	if err := os.WriteFile(tmp, []byte(got), 0o644); err != nil {
		t.Fatal(err)
	}
	diff, _ := exec.Command("diff", "-u", path, tmp).CombinedOutput()
	t.Errorf("golden output changed; run `go test -run TestGolden -update` if intended:\n%s", diff)
}
