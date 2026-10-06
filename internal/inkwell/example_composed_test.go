package inkwell

import (
	"os"
	"strings"
	"testing"

	"github.com/grantlucas/inkwell/internal/inkwell/testutil"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

// composedScreen is the commented-out screen at the end of the example
// config that shows how a screen is composed from the day widgets.
const composedScreen = "today-and-tomorrow"

// uncommentScreen returns the example config with the commented-out
// screen named name uncommented and added to the end of the rotation, as
// someone following its instructions would. The block runs from its
// "# - name:" line to the end of the comment.
func uncommentScreen(t *testing.T, example, name string) string {
	t.Helper()
	const indent = "    "
	lines := strings.Split(example, "\n")
	start := -1
	for i, l := range lines {
		if l == indent+"# - name: "+name {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatalf("the example config has no commented-out %q screen", name)
	}
	var block []string
	for _, l := range lines[start:] {
		if !strings.HasPrefix(l, indent+"#") {
			break
		}
		block = append(block, indent+strings.TrimPrefix(strings.TrimPrefix(l, indent+"#"), " "))
	}
	return example + "\n" + strings.Join(block, "\n") + "\n"
}

// The example config's composed screen, uncommented as its instructions
// say, loads and draws in every widget: the clock band, and for today
// and tomorrow a day badge, a combined chart on the range the two share,
// and an event list.
func TestExampleConfig_ComposedScreen(t *testing.T) {
	raw, err := os.ReadFile(exampleConfigPath)
	if err != nil {
		t.Fatalf("read example config: %v", err)
	}
	cfg, err := LoadConfig(strings.NewReader(uncommentScreen(t, string(raw), composedScreen)))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	app, err := NewApp(cfg, WithHardware(&MockHardware{}), WithHTTPClient(exampleUpstream()),
		WithDeps(widget.Deps{Now: exampleNow}))
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	screens := app.dashboard.screens
	screen := screens[len(screens)-1]
	if screen.Name != composedScreen {
		t.Fatalf("last screen = %q, want %q", screen.Name, composedScreen)
	}
	frame, err := app.comp.Render(screen.Widgets())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for _, w := range screen.Widgets() {
		if !inked(frame, w.Bounds()) {
			t.Errorf("%T at %v drew nothing", w, w.Bounds())
		}
	}
	testutil.AssertGoldenPNG(t, frame)
}
