package render

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/GeorgePearse/a2ui-tea/a2ui"
)

var update = flag.Bool("update", false, "rewrite golden files")

var goldenWidths = []int{60, 80, 120}

func loadExample(t *testing.T, path string) *Model {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	msgs, err := a2ui.ParseMessages(data)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	m := New(Options{HideHelp: true})
	if err := m.ApplyAll(msgs); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return m
}

func TestGoldenExamples(t *testing.T) {
	files, err := filepath.Glob("../testdata/examples/*.json")
	if err != nil || len(files) == 0 {
		t.Fatalf("no examples found: %v", err)
	}
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".json")
		for _, width := range goldenWidths {
			t.Run(name+"_"+itoa(width), func(t *testing.T) {
				m := loadExample(t, file)
				m.SetSize(width, 200)
				var out []string
				for _, s := range m.store.Surfaces() {
					out = append(out, ansi.Strip(m.RenderSurface(s.ID, width)))
				}
				got := strings.Join(out, "\n\n") + "\n"
				golden := filepath.Join("../testdata/golden", name+"_"+itoa(width)+".txt")
				if *update {
					if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
						t.Fatal(err)
					}
					return
				}
				want, err := os.ReadFile(golden)
				if err != nil {
					t.Fatalf("missing golden %s (run with -update): %v", golden, err)
				}
				if string(want) != got {
					t.Errorf("render differs from %s\n--- got ---\n%s\n--- want ---\n%s", golden, got, want)
				}
				for _, line := range strings.Split(got, "\n") {
					if w := ansi.StringWidth(line); w > width {
						t.Errorf("line wider than %d (%d): %q", width, w, line)
					}
				}
			})
		}
	}
}

func itoa(i int) string {
	return strings.TrimSpace(strings.Repeat(" ", 0) + fmtInt(i))
}

func fmtInt(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}
