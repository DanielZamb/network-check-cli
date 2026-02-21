package events

import (
	"encoding/json"
	"io"
	"time"
)

type Event struct {
	EventType string         `json:"event_type"`
	Timestamp time.Time      `json:"timestamp"`
	RunID     string         `json:"run_id"`
	Sequence  int            `json:"sequence"`
	Payload   map[string]any `json:"payload,omitempty"`
}

type Writer struct {
	w   io.Writer
	seq int
	now func() time.Time
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w, now: func() time.Time { return time.Now().UTC() }}
}

func (wr *Writer) Emit(eventType, runID string, payload map[string]any) error {
	wr.seq++
	e := Event{EventType: eventType, Timestamp: wr.now(), RunID: runID, Sequence: wr.seq, Payload: payload}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = wr.w.Write(append(b, '\n'))
	return err
}

func (wr *Writer) SetNow(now func() time.Time) {
	if now != nil {
		wr.now = now
	}
}
