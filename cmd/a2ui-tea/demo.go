package main

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/GeorgePearse/a2ui-tea/a2ui"
	"github.com/GeorgePearse/a2ui-tea/catalog"
	"github.com/GeorgePearse/a2ui-tea/render"
)

// demoAgent plays a scripted agent: it creates a surface, then keeps
// reshaping it with updateComponents and updateDataModel messages while the
// user interacts, and answers the user's actions.
type demoAgent struct {
	p *tea.Program
}

func newDemoAgent(p *tea.Program) *demoAgent {
	return &demoAgent{p: p}
}

type step struct {
	after time.Duration
	json  string
}

var demoSteps = []step{
	{0, `{"version":"v1.0","createSurface":{"surfaceId":"demo","catalogId":"` + catalog.BasicCatalogID + `","sendDataModel":true,
	  "components":[
	    {"id":"root","component":"Card","child":"main"},
	    {"id":"main","component":"Column","children":["title","subtitle","greet_row","greeting"]},
	    {"id":"title","component":"Text","text":"# Agent dashboard"},
	    {"id":"subtitle","component":"Text","text":"Every part of this UI arrives as an A2UI message while the program runs.","variant":"caption"},
	    {"id":"greet_row","component":"Row","children":["name","greet"],"align":"end"},
	    {"id":"name","component":"TextField","label":"Your name","placeholder":"type here","value":{"@path":"/user/name"},"weight":1},
	    {"id":"greet","component":"Button","child":"greet_label","variant":"primary",
	      "checks":[{"condition":{"@call":"required","args":{"value":{"@path":"/user/name"}}},"message":"Enter a name first"}],
	      "action":{"event":{"name":"greet","context":{"name":{"@path":"/user/name"}}}}},
	    {"id":"greet_label","component":"Text","text":"Greet"},
	    {"id":"greeting","component":"Text","text":{"@path":"/greeting"}}
	  ],
	  "dataModel":{"user":{"name":""},"greeting":"","status":"idle"}}}`},
	{900 * time.Millisecond, `{"version":"v1.0","updateComponents":{"surfaceId":"demo","components":[
	    {"id":"main","component":"Column","children":["title","subtitle","greet_row","greeting","stats"]},
	    {"id":"stats","component":"Row","children":["stat_req","stat_lat","stat_err"]},
	    {"id":"stat_req","component":"Card","child":"stat_req_col","weight":1},
	    {"id":"stat_req_col","component":"Column","children":["stat_req_label","stat_req_value"],"align":"center"},
	    {"id":"stat_req_label","component":"Text","text":"requests","variant":"caption"},
	    {"id":"stat_req_value","component":"Text","text":{"@call":"formatNumber","args":{"value":{"@path":"/stats/requests"}}}},
	    {"id":"stat_lat","component":"Card","child":"stat_lat_col","weight":1},
	    {"id":"stat_lat_col","component":"Column","children":["stat_lat_label","stat_lat_value"],"align":"center"},
	    {"id":"stat_lat_label","component":"Text","text":"p95 latency","variant":"caption"},
	    {"id":"stat_lat_value","component":"Text","text":{"@call":"formatString","args":{"value":"${/stats/p95} ms"}}},
	    {"id":"stat_err","component":"Card","child":"stat_err_col","weight":1},
	    {"id":"stat_err_col","component":"Column","children":["stat_err_label","stat_err_value"],"align":"center"},
	    {"id":"stat_err_label","component":"Text","text":"errors","variant":"caption"},
	    {"id":"stat_err_value","component":"Text","text":{"@path":"/stats/errors"}}
	  ]}}`},
	{0, `{"version":"v1.0","updateDataModel":{"surfaceId":"demo","path":"/stats","value":{"requests":1204,"p95":212,"errors":3}}}`},
	{900 * time.Millisecond, `{"version":"v1.0","updateDataModel":{"surfaceId":"demo","path":"/stats","value":{"requests":1377,"p95":198,"errors":3}}}`},
	{900 * time.Millisecond, `{"version":"v1.0","updateDataModel":{"surfaceId":"demo","path":"/stats","value":{"requests":1593,"p95":171,"errors":4}}}`},
	{900 * time.Millisecond, `{"version":"v1.0","updateComponents":{"surfaceId":"demo","components":[
	    {"id":"main","component":"Column","children":["title","subtitle","greet_row","greeting","tabs"]},
	    {"id":"tabs","component":"Tabs","tabs":[{"title":"Overview","child":"stats"},{"title":"Tasks","child":"tasks"},{"title":"Settings","child":"settings"}]},
	    {"id":"tasks","component":"List","children":{"path":"/tasks","componentId":"task_row"}},
	    {"id":"task_row","component":"Row","children":["task_idx","task_title","task_state"]},
	    {"id":"task_idx","component":"Text","text":{"@call":"formatString","args":{"value":"${@index(offset: 1)}."}}},
	    {"id":"task_title","component":"Text","text":{"@path":"title"},"weight":1},
	    {"id":"task_state","component":"Text","text":{"@path":"state"},"variant":"caption"},
	    {"id":"settings","component":"Column","children":["threshold","notify","region"]},
	    {"id":"threshold","component":"Slider","label":"Alert threshold","min":0,"max":100,"steps":20,"value":{"@path":"/settings/threshold"}},
	    {"id":"notify","component":"CheckBox","label":"Notify on errors","value":{"@path":"/settings/notify"}},
	    {"id":"region","component":"ChoicePicker","label":"Region","displayStyle":"chips","options":[{"label":"eu-west","value":"eu"},{"label":"us-east","value":"us"},{"label":"ap-south","value":"ap"}],"value":{"@path":"/settings/region"}}
	  ]}}`},
	{0, `{"version":"v1.0","updateDataModel":{"surfaceId":"demo","path":"/tasks","value":[{"title":"Index documents","state":"done"},{"title":"Train classifier","state":"running"}]}}`},
	{0, `{"version":"v1.0","updateDataModel":{"surfaceId":"demo","path":"/settings","value":{"threshold":40,"notify":true,"region":["eu"]}}}`},
	{1200 * time.Millisecond, `{"version":"v1.0","updateDataModel":{"surfaceId":"demo","path":"/tasks/2","value":{"title":"Evaluate on holdout","state":"queued"}}}`},
	{900 * time.Millisecond, `{"version":"v1.0","updateDataModel":{"surfaceId":"demo","path":"/tasks/1/state","value":"done"}}`},
	{900 * time.Millisecond, `{"version":"v1.0","updateComponents":{"surfaceId":"demo","components":[
	    {"id":"main","component":"Column","children":["title","subtitle","greet_row","greeting","tabs","deploy_row"]},
	    {"id":"deploy_row","component":"Row","children":["deploy_modal","status"],"align":"center"},
	    {"id":"deploy_modal","component":"Modal","trigger":"deploy","content":"confirm"},
	    {"id":"deploy","component":"Button","child":"deploy_label","action":{"event":{"name":"open_deploy"}}},
	    {"id":"deploy_label","component":"Text","text":"Deploy…"},
	    {"id":"status","component":"Text","text":{"@call":"formatString","args":{"value":"status: ${/status}"}},"variant":"caption"},
	    {"id":"confirm","component":"Column","children":["confirm_text","confirm_row"]},
	    {"id":"confirm_text","component":"Text","text":"Deploy build **42** to production?"},
	    {"id":"confirm_row","component":"Row","children":["confirm_yes","confirm_no"],"justify":"end"},
	    {"id":"confirm_yes","component":"Button","child":"confirm_yes_label","variant":"primary","action":{"event":{"name":"deploy","context":{"build":42,"region":{"@path":"/settings/region"}}}}},
	    {"id":"confirm_yes_label","component":"Text","text":"Deploy"},
	    {"id":"confirm_no","component":"Button","child":"confirm_no_label","action":{"event":{"name":"cancel_deploy"}}},
	    {"id":"confirm_no_label","component":"Text","text":"Cancel"}
	  ]}}`},
}

func (d *demoAgent) run() {
	for _, s := range demoSteps {
		time.Sleep(s.after)
		d.send(s.json)
	}
}

func (d *demoAgent) send(src string) {
	msgs, err := a2ui.ParseMessages([]byte(src))
	if err != nil {
		d.p.Send(render.StatusMsg("demo: " + err.Error()))
		return
	}
	d.p.Send(render.MessagesMsg(msgs))
}

// handle answers user actions the way an agent would.
func (d *demoAgent) handle(out a2ui.Outbound) {
	a := out.Action
	switch a.Name {
	case "greet":
		name, _ := a.Context["name"].(string)
		d.send(fmt.Sprintf(`{"version":"v1.0","updateDataModel":{"surfaceId":"demo","path":"/greeting","value":%q}}`,
			"Hello, "+strings.TrimSpace(name)+"! The agent received your name as an A2UI action."))
	case "deploy":
		region := strings.Join(a2ui.ToStrings(a.Context["region"]), ",")
		d.send(`{"version":"v1.0","updateComponents":{"surfaceId":"demo","components":[
		    {"id":"deploy_row","component":"Row","children":["status"]},
		    {"id":"status","component":"Text","text":{"@call":"formatString","args":{"value":"**status:** ${/status}"}}}]}}`)
		d.send(fmt.Sprintf(`{"version":"v1.0","updateDataModel":{"surfaceId":"demo","path":"/status","value":%q}}`, "deploying build 42 to "+region+"…"))
		go func() {
			time.Sleep(1500 * time.Millisecond)
			d.send(`{"version":"v1.0","updateDataModel":{"surfaceId":"demo","path":"/status","value":"✓ build 42 live"}}`)
		}()
	case "cancel_deploy":
		d.send(`{"version":"v1.0","updateDataModel":{"surfaceId":"demo","path":"/status","value":"cancelled"}}`)
	}
}
