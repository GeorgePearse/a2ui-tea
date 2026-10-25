package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/GeorgePearse/a2ui-tea/a2ui"
	"github.com/GeorgePearse/a2ui-tea/render"
)

func readStdin(p *tea.Program, logger *log.Logger) {
	err := a2ui.ReadMessages(os.Stdin, func(m a2ui.Message) error {
		p.Send(render.MessagesMsg{m})
		return nil
	})
	if err != nil {
		logger.Printf("stdin: %v", err)
		p.Send(render.StatusMsg("stdin: " + err.Error()))
	}
	logger.Printf("stdin closed")
}

func watchFile(p *tea.Program, path string, logger *log.Logger) {
	var lastMod time.Time
	var lastSize int64
	for {
		info, err := os.Stat(path)
		if err != nil {
			p.Send(render.StatusMsg("file: " + err.Error()))
			time.Sleep(time.Second)
			continue
		}
		if !info.ModTime().Equal(lastMod) || info.Size() != lastSize {
			lastMod, lastSize = info.ModTime(), info.Size()
			data, err := os.ReadFile(path)
			if err != nil {
				p.Send(render.StatusMsg("file: " + err.Error()))
			} else if msgs, err := a2ui.ParseMessages(data); err != nil {
				p.Send(render.StatusMsg("file: " + err.Error()))
			} else {
				logger.Printf("file reloaded: %d messages", len(msgs))
				p.Send(render.ResetMsg{})
				p.Send(render.MessagesMsg(msgs))
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
}

func streamURL(p *tea.Program, url string, logger *log.Logger) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		p.Send(render.StatusMsg("url: " + err.Error()))
		return
	}
	req.Header.Set("Accept", "text/event-stream, application/x-ndjson, application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		p.Send(render.StatusMsg("url: " + err.Error()))
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		p.Send(render.StatusMsg(fmt.Sprintf("url: HTTP %d", resp.StatusCode)))
		return
	}
	deliver := func(payload []byte) {
		msgs, err := a2ui.ParseMessages(payload)
		if err != nil {
			logger.Printf("url: %v", err)
			p.Send(render.StatusMsg("url: " + err.Error()))
			return
		}
		if len(msgs) > 0 {
			p.Send(render.MessagesMsg(msgs))
		}
	}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)
	sse := strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream")
	var data []string
	for sc.Scan() {
		line := sc.Text()
		if !sse {
			if strings.TrimSpace(line) != "" {
				deliver([]byte(line))
			}
			continue
		}
		switch {
		case line == "":
			if len(data) > 0 {
				deliver([]byte(strings.Join(data, "\n")))
				data = nil
			}
		case strings.HasPrefix(line, "data:"):
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	if len(data) > 0 {
		deliver([]byte(strings.Join(data, "\n")))
	}
	if err := sc.Err(); err != nil {
		p.Send(render.StatusMsg("url: " + err.Error()))
	}
	logger.Printf("stream closed")
}

func postJSON(url string, body []byte, logger *log.Logger, status func(string)) {
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		logger.Printf("post: %v", err)
		if status != nil {
			status("post: " + err.Error())
		}
		return
	}
	defer func() { _ = resp.Body.Close() }()
	logger.Printf("post %s -> %d", url, resp.StatusCode)
}
