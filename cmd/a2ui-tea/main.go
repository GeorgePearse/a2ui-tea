// Command a2ui-tea renders a stream of A2UI messages as a Bubble Tea
// program and writes user actions back out.
//
//	a2ui-tea --stdin            read JSON lines from stdin
//	a2ui-tea --file ui.json     render a file and re-render when it changes
//	a2ui-tea --url http://...   consume SSE or JSON lines, POST actions back
//	a2ui-tea demo               a scripted agent that reshapes the UI live
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"

	"github.com/GeorgePearse/a2ui-tea/a2ui"
	"github.com/GeorgePearse/a2ui-tea/render"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "a2ui-tea:", err)
		os.Exit(1)
	}
}

type config struct {
	stdin   bool
	file    string
	url     string
	post    string
	logPath string
	noAlt   bool
	noHelp  bool
	stay    bool
	demo    bool
}

func parseFlags(args []string) (config, error) {
	var c config
	fs := flag.NewFlagSet("a2ui-tea", flag.ContinueOnError)
	fs.BoolVar(&c.stdin, "stdin", false, "read A2UI messages as JSON lines from stdin")
	fs.StringVar(&c.file, "file", "", "render the messages in this file and watch it for changes")
	fs.StringVar(&c.url, "url", "", "stream messages from this HTTP(S) URL (SSE or JSON lines)")
	fs.StringVar(&c.post, "post", "", "POST user actions to this URL (default: --url)")
	fs.StringVar(&c.logPath, "log", "", "append a debug trace to this file")
	fs.BoolVar(&c.noAlt, "no-alt-screen", false, "render inline instead of in the alternate screen")
	fs.BoolVar(&c.noHelp, "no-help", false, "hide the key hints line")
	fs.BoolVar(&c.stay, "stay", false, "keep running after the last surface is deleted")
	fs.Usage = func() {
		_, _ = fmt.Fprintf(fs.Output(), "usage: a2ui-tea (--stdin | --file PATH | --url URL | demo) [flags]\n\n")
		fs.PrintDefaults()
	}
	var rest []string
	for _, a := range args {
		if a == "demo" {
			c.demo = true
			continue
		}
		rest = append(rest, a)
	}
	if err := fs.Parse(rest); err != nil {
		return c, err
	}
	modes := 0
	for _, on := range []bool{c.stdin, c.file != "", c.url != "", c.demo} {
		if on {
			modes++
		}
	}
	if modes != 1 {
		fs.Usage()
		return c, fmt.Errorf("choose exactly one of --stdin, --file, --url or demo")
	}
	return c, nil
}

func run(args []string) error {
	cfg, err := parseFlags(args)
	if err != nil {
		return err
	}
	logger := log.New(io.Discard, "", 0)
	if cfg.logPath != "" {
		f, err := os.OpenFile(cfg.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		logger = log.New(f, "", log.Ltime|log.Lmicroseconds)
	}

	ttyIn, ttyOut, err := tea.OpenTTY()
	if err != nil {
		return fmt.Errorf("a terminal is required: %w", err)
	}
	defer func() { _ = ttyIn.Close() }()
	defer func() { _ = ttyOut.Close() }()

	sink := newSink(os.Stdout, cfg, logger)
	var program *tea.Program
	opts := render.Options{
		Outbound:    sink.outbound,
		DataModel:   sink.dataModel,
		Log:         logger,
		AltScreen:   !cfg.noAlt,
		HideHelp:    cfg.noHelp,
		QuitOnEmpty: !cfg.stay && !cfg.demo,
	}
	model := render.New(opts)
	program = tea.NewProgram(model, tea.WithInput(ttyIn), tea.WithOutput(ttyOut))
	sink.status = func(s string) { go program.Send(render.StatusMsg(s)) }

	switch {
	case cfg.stdin:
		go readStdin(program, logger)
	case cfg.file != "":
		go watchFile(program, cfg.file, logger)
	case cfg.url != "":
		go streamURL(program, cfg.url, logger)
	case cfg.demo:
		agent := newDemoAgent(program)
		sink.demo = agent
		go agent.run()
	}
	_, err = program.Run()
	return err
}

// sink writes renderer-to-agent messages: JSON lines on stdout when stdout
// is not a terminal, a status line otherwise, and a POST in url mode.
type sink struct {
	mu     sync.Mutex
	w      io.Writer
	isTTY  bool
	post   string
	log    *log.Logger
	status func(string)
	demo   *demoAgent
}

func newSink(w *os.File, cfg config, logger *log.Logger) *sink {
	post := cfg.post
	if post == "" && cfg.url != "" {
		post = cfg.url
	}
	return &sink{w: w, isTTY: term.IsTerminal(w.Fd()), post: post, log: logger}
}

func (s *sink) outbound(out a2ui.Outbound) {
	data, err := json.Marshal(out)
	if err != nil {
		return
	}
	s.write(data)
	if s.demo != nil && out.Action != nil {
		go s.demo.handle(out)
	}
	if s.post != "" {
		go postJSON(s.post, data, s.log, s.status)
	}
	if s.isTTY && s.status != nil && out.Action != nil {
		ctx, _ := json.Marshal(out.Action.Context)
		s.status("→ " + out.Action.Name + " " + string(ctx))
	}
}

func (s *sink) dataModel(dm a2ui.RendererDataModel) {
	data, err := json.Marshal(map[string]any{"rendererDataModel": dm})
	if err != nil {
		return
	}
	s.write(data)
}

func (s *sink) write(line []byte) {
	s.log.Printf("out %s", line)
	if s.isTTY {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = s.w.Write(append(line, '\n'))
}
