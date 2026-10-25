package a2ui

import (
	"time"
)

// Outbound is one renderer-to-agent envelope. Exactly one message field is
// set.
type Outbound struct {
	Version                  string            `json:"version"`
	Action                   *Action           `json:"action,omitempty"`
	RendererFunctionResponse *FunctionResponse `json:"rendererFunctionResponse,omitempty"`
	Error                    *Error            `json:"error,omitempty"`
}

// Action reports a user interaction with a component.
type Action struct {
	Name              string         `json:"name"`
	UserMessage       string         `json:"userMessage,omitempty"`
	SurfaceID         string         `json:"surfaceId"`
	SourceComponentID string         `json:"sourceComponentId"`
	Timestamp         string         `json:"timestamp"`
	Context           map[string]any `json:"context"`
}

// Error reports a renderer-side error to the agent.
type Error struct {
	Code           string `json:"code"`
	Message        string `json:"message"`
	SurfaceID      string `json:"surfaceId,omitempty"`
	Path           string `json:"path,omitempty"`
	FunctionCallID string `json:"functionCallId,omitempty"`
}

// RendererDataModel is the data model sync payload sent alongside outbound
// messages for surfaces created with sendDataModel.
type RendererDataModel struct {
	Version  string                    `json:"version"`
	Surfaces map[string]map[string]any `json:"surfaces"`
}

// EventSpec is a component's action.event definition.
type EventSpec struct {
	Name        string
	UserMessage any
	Context     map[string]any
}

// ActionOf reads a component's action property: an event to dispatch, or a
// local function call.
func ActionOf(c *Component) (event *EventSpec, call map[string]any) {
	raw, ok := c.Prop("action")
	if !ok {
		return nil, nil
	}
	action, ok := raw.(map[string]any)
	if !ok {
		return nil, nil
	}
	if ev, ok := action["event"].(map[string]any); ok {
		name, _ := ev["name"].(string)
		ctx, _ := ev["context"].(map[string]any)
		return &EventSpec{Name: name, UserMessage: ev["userMessage"], Context: ctx}, nil
	}
	if fc, ok := action["functionCall"].(map[string]any); ok {
		return nil, fc
	}
	if name, ok := action["name"].(string); ok {
		ctx, _ := action["context"].(map[string]any)
		return &EventSpec{Name: name, Context: ctx}, nil
	}
	return nil, nil
}

// BuildAction resolves an event's context in the node's scope and wraps it
// in an outbound envelope.
func (e *Evaluator) BuildAction(n Node, ev *EventSpec, now time.Time) Outbound {
	ctx := map[string]any{}
	for k, v := range ev.Context {
		r, err := e.Resolve(v, n.Scope)
		if err != nil {
			r = nil
		}
		ctx[k] = r
	}
	a := &Action{
		Name:              ev.Name,
		SurfaceID:         n.Surface.ID,
		SourceComponentID: n.ID(),
		Timestamp:         now.UTC().Format(time.RFC3339),
		Context:           ctx,
	}
	if ev.UserMessage != nil {
		a.UserMessage = e.String(ev.UserMessage, n.Scope)
	}
	return Outbound{Version: CurrentVersion, Action: a}
}

// ErrorMessage wraps an error for the agent.
func ErrorMessage(err error, surfaceID string) Outbound {
	out := Error{Code: "VALIDATION_FAILED", SurfaceID: surfaceID, Message: err.Error()}
	if pe, ok := err.(*ProtocolError); ok {
		out.Code, out.SurfaceID, out.Message = pe.Code, pe.SurfaceID, pe.Message
	}
	return Outbound{Version: CurrentVersion, Error: &out}
}

// HandleCall executes a callRendererFunction message in the surface's root
// scope (or an empty one) and returns the response envelope.
func (e *Evaluator) HandleCall(st *Store, call *CallRendererFunction) Outbound {
	scope := RootScope(NewDataModel())
	if call.SurfaceID != "" {
		if s, ok := st.Surface(call.SurfaceID); ok {
			scope = RootScope(s.Data)
		}
	}
	resp := &FunctionResponse{FunctionCallID: call.FunctionCallID}
	name, args, ok := IsCall(call.CallFunction)
	if !ok {
		resp.Error = &FunctionError{Code: "INVALID_FUNCTION_CALL", Message: "callFunction needs @call"}
		return Outbound{Version: CurrentVersion, RendererFunctionResponse: resp}
	}
	v, err := e.Call(name, args, scope)
	if err != nil {
		resp.Error = &FunctionError{Code: "FUNCTION_FAILED", Message: err.Error()}
	} else if v == nil {
		resp.Value = map[string]any{}
	} else {
		resp.Value = v
	}
	return Outbound{Version: CurrentVersion, RendererFunctionResponse: resp}
}
