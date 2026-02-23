package engine

import (
	"time"

	"rfguard/internal/model"
)

type EventEntry struct {
	Timestamp time.Time
	UID       string
	Result    model.Result
}

type WindowState struct {
	duration  time.Duration
	events    []EventEntry
	head      int
	attempts  int
	failures  int
	uidCounts map[string]int
}

func NewWindowState(duration time.Duration) *WindowState {
	return &WindowState{
		duration:  duration,
		events:    make([]EventEntry, 0, 128),
		uidCounts: make(map[string]int),
	}
}

func (w *WindowState) Add(ev EventEntry) {
	w.events = append(w.events, ev)
	w.attempts++
	if ev.Result == model.ResultFailure {
		w.failures++
	}
	if ev.UID != "" {
		w.uidCounts[ev.UID]++
	}
}

func (w *WindowState) Evict(cutoff time.Time) {
	for w.head < len(w.events) {
		ev := w.events[w.head]
		if !ev.Timestamp.Before(cutoff) {
			break
		}
		w.attempts--
		if ev.Result == model.ResultFailure {
			w.failures--
		}
		if ev.UID != "" {
			if count := w.uidCounts[ev.UID]; count <= 1 {
				delete(w.uidCounts, ev.UID)
			} else {
				w.uidCounts[ev.UID] = count - 1
			}
		}
		w.head++
	}
	if w.head > 0 && w.head*2 >= len(w.events) {
		w.events = append([]EventEntry{}, w.events[w.head:]...)
		w.head = 0
	}
}

func (w *WindowState) Metrics() model.WindowMetrics {
	attempts := w.attempts
	failures := w.failures
	windowSec := int(w.duration.Seconds())
	aps := 0.0
	fr := 0.0
	uds := 0.0
	if attempts > 0 {
		aps = float64(attempts) / w.duration.Seconds()
		fr = float64(failures) / float64(attempts)
		uds = float64(len(w.uidCounts)) / float64(attempts)
	}
	tv := varianceDelta(w.events, w.head)
	return model.WindowMetrics{
		WindowSec: windowSec,
		Attempts:  attempts,
		Failures:  failures,
		APS:       aps,
		FR:        fr,
		UDS:       uds,
		TV:        tv,
	}
}

func varianceDelta(events []EventEntry, start int) float64 {
	if len(events)-start <= 1 {
		return 0
	}
	var n int
	var mean float64
	var m2 float64
	prev := events[start].Timestamp
	for i := start + 1; i < len(events); i++ {
		delta := events[i].Timestamp.Sub(prev).Seconds()
		if delta < 0 {
			delta = 0
		}
		n++
		diff := delta - mean
		mean += diff / float64(n)
		m2 += diff * (delta - mean)
		prev = events[i].Timestamp
	}
	if n == 0 {
		return 0
	}
	return m2 / float64(n)
}
