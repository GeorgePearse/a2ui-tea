package a2ui

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// CurrentVersion is the protocol version this package implements and emits.
const CurrentVersion = "v1.0"

// AcceptedVersions lists the envelope versions ParseMessage accepts. Messages
// of these versions share the same shape; v0.9 bindings written as
// {"path": ...} are understood as well as the v1.0 {"@path": ...}.
var AcceptedVersions = []string{"v1.0", "v0.9.1", "v0.9"}

// Message is one agent-to-renderer envelope. Exactly one of the message
// fields is set.
type Message struct {
	Version               string                `json:"version,omitempty"`
	CreateSurface         *CreateSurface        `json:"createSurface,omitempty"`
	UpdateComponents      *UpdateComponents     `json:"updateComponents,omitempty"`
	UpdateDataModel       *UpdateDataModel      `json:"updateDataModel,omitempty"`
	DeleteSurface         *DeleteSurface        `json:"deleteSurface,omitempty"`
	CallRendererFunction  *CallRendererFunction `json:"callRendererFunction,omitempty"`
	AgentFunctionResponse *FunctionResponse     `json:"agentFunctionResponse,omitempty"`
}

// CreateSurface creates a surface, optionally with its first components and
// data model.
type CreateSurface struct {
	SurfaceID     string         `json:"surfaceId"`
	CatalogID     string         `json:"catalogId,omitempty"`
	SendDataModel bool           `json:"sendDataModel,omitempty"`
	Components    []Component    `json:"components,omitempty"`
	DataModel     map[string]any `json:"dataModel,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	Theme         map[string]any `json:"theme,omitempty"`
}

// UpdateComponents adds or replaces components in a surface.
type UpdateComponents struct {
	SurfaceID  string      `json:"surfaceId"`
	Components []Component `json:"components"`
}

// UpdateDataModel replaces the value at Path; a nil Value deletes it.
type UpdateDataModel struct {
	SurfaceID string `json:"surfaceId"`
	Path      string `json:"path,omitempty"`
	Value     any    `json:"value"`
}

// DeleteSurface removes a surface and everything in it.
type DeleteSurface struct {
	SurfaceID string `json:"surfaceId"`
}

// CallRendererFunction asks the renderer to run a catalog function.
type CallRendererFunction struct {
	SurfaceID      string         `json:"surfaceId,omitempty"`
	FunctionCallID string         `json:"functionCallId"`
	CallFunction   map[string]any `json:"callFunction"`
}

// FunctionResponse answers a function call in either direction.
type FunctionResponse struct {
	FunctionCallID string         `json:"functionCallId"`
	Value          any            `json:"value,omitempty"`
	Error          *FunctionError `json:"error,omitempty"`
}

// FunctionError reports a failed function execution.
type FunctionError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Kind returns the envelope key of the message, such as "createSurface".
func (m Message) Kind() string {
	switch {
	case m.CreateSurface != nil:
		return "createSurface"
	case m.UpdateComponents != nil:
		return "updateComponents"
	case m.UpdateDataModel != nil:
		return "updateDataModel"
	case m.DeleteSurface != nil:
		return "deleteSurface"
	case m.CallRendererFunction != nil:
		return "callRendererFunction"
	case m.AgentFunctionResponse != nil:
		return "agentFunctionResponse"
	}
	return ""
}

// SurfaceID returns the surface the message targets, if any.
func (m Message) SurfaceID() string {
	switch {
	case m.CreateSurface != nil:
		return m.CreateSurface.SurfaceID
	case m.UpdateComponents != nil:
		return m.UpdateComponents.SurfaceID
	case m.UpdateDataModel != nil:
		return m.UpdateDataModel.SurfaceID
	case m.DeleteSurface != nil:
		return m.DeleteSurface.SurfaceID
	case m.CallRendererFunction != nil:
		return m.CallRendererFunction.SurfaceID
	}
	return ""
}

// Validate checks the envelope carries exactly one message of an accepted
// version with its required fields.
func (m Message) Validate() error {
	n := 0
	for _, set := range []bool{
		m.CreateSurface != nil, m.UpdateComponents != nil, m.UpdateDataModel != nil,
		m.DeleteSurface != nil, m.CallRendererFunction != nil, m.AgentFunctionResponse != nil,
	} {
		if set {
			n++
		}
	}
	if n != 1 {
		return fmt.Errorf("a2ui: message must contain exactly one of createSurface, updateComponents, updateDataModel, deleteSurface, callRendererFunction or agentFunctionResponse (got %d)", n)
	}
	if m.Version != "" && !versionAccepted(m.Version) {
		return fmt.Errorf("a2ui: unsupported version %q", m.Version)
	}
	kind := m.Kind()
	if kind != "agentFunctionResponse" && kind != "callRendererFunction" && m.SurfaceID() == "" {
		return fmt.Errorf("a2ui: %s requires surfaceId", kind)
	}
	switch {
	case m.UpdateComponents != nil && len(m.UpdateComponents.Components) == 0:
		return errors.New("a2ui: updateComponents requires at least one component")
	case m.CallRendererFunction != nil && m.CallRendererFunction.FunctionCallID == "":
		return errors.New("a2ui: callRendererFunction requires functionCallId")
	case m.AgentFunctionResponse != nil && m.AgentFunctionResponse.FunctionCallID == "":
		return errors.New("a2ui: agentFunctionResponse requires functionCallId")
	}
	for _, c := range m.components() {
		if err := c.validate(); err != nil {
			return err
		}
	}
	return nil
}

func (m Message) components() []Component {
	switch {
	case m.CreateSurface != nil:
		return m.CreateSurface.Components
	case m.UpdateComponents != nil:
		return m.UpdateComponents.Components
	}
	return nil
}

func versionAccepted(v string) bool {
	for _, a := range AcceptedVersions {
		if a == v {
			return true
		}
	}
	return false
}

// ParseMessage decodes a single envelope and validates it.
func ParseMessage(data []byte) (Message, error) {
	var m Message
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return Message{}, fmt.Errorf("a2ui: decode message: %w", err)
	}
	if err := m.Validate(); err != nil {
		return Message{}, err
	}
	return m, nil
}

// ParseMessages decodes one envelope, a JSON array of envelopes, or an object
// with a "messages" array (the layout of the spec's example files).
func ParseMessages(data []byte) ([]Message, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, nil
	}
	if trimmed[0] == '[' {
		var raws []json.RawMessage
		if err := json.Unmarshal(trimmed, &raws); err != nil {
			return nil, fmt.Errorf("a2ui: decode message array: %w", err)
		}
		return parseEach(raws)
	}
	var probe struct {
		Messages []json.RawMessage `json:"messages"`
	}
	if err := json.Unmarshal(trimmed, &probe); err == nil && probe.Messages != nil {
		return parseEach(probe.Messages)
	}
	m, err := ParseMessage(trimmed)
	if err != nil {
		return nil, err
	}
	return []Message{m}, nil
}

func parseEach(raws []json.RawMessage) ([]Message, error) {
	out := make([]Message, 0, len(raws))
	for i, raw := range raws {
		m, err := ParseMessage(raw)
		if err != nil {
			return out, fmt.Errorf("message %d: %w", i, err)
		}
		out = append(out, m)
	}
	return out, nil
}

// ReadMessages reads JSON Lines from r and calls fn for every envelope. A
// line may hold one envelope or an array of them; blank lines are skipped.
// Reading stops at the first error returned by fn.
func ReadMessages(r io.Reader, fn func(Message) error) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		msgs, err := ParseMessages([]byte(line))
		if err != nil {
			return err
		}
		for _, m := range msgs {
			if err := fn(m); err != nil {
				return err
			}
		}
	}
	return sc.Err()
}
