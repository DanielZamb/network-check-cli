package main

import (
	"fmt"
	"io"
	"netcheck/internal/runner"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type uiOptions struct {
	Verbose bool
	Color   bool
	Animate bool
	MaxHead int
}

type verboseUI struct {
	w        io.Writer
	mu       sync.Mutex
	done     int
	total    int
	current  string
	group    string
	verbose  bool
	color    bool
	animate  bool
	maxHead  int
	termW    int
	frame    int
	stopCh   chan struct{}
	spinDone chan struct{}
	stopOnce sync.Once
}

func newVerboseUI(w io.Writer, opts uiOptions) *verboseUI {
	ui := &verboseUI{
		w:        w,
		verbose:  opts.Verbose,
		color:    opts.Color,
		animate:  opts.Animate,
		maxHead:  opts.MaxHead,
		termW:    detectTerminalWidth(),
		stopCh:   make(chan struct{}),
		spinDone: make(chan struct{}),
	}
	if ui.maxHead <= 0 {
		ui.maxHead = 120
	}
	if ui.animate {
		go ui.spin()
	}
	return ui
}

func (u *verboseUI) spin() {
	defer close(u.spinDone)
	frames := []string{"|", "/", "-", "\\"}
	ticker := time.NewTicker(120 * time.Millisecond)
	defer ticker.Stop()
	i := 0
	for {
		select {
		case <-u.stopCh:
			u.mu.Lock()
			fmt.Fprint(u.w, "\r\x1b[2K")
			u.mu.Unlock()
			return
		case <-ticker.C:
			u.mu.Lock()
			u.renderLocked(frames[i%len(frames)])
			u.mu.Unlock()
			i++
		}
	}
}

func (u *verboseUI) Stop() {
	u.stopOnce.Do(func() {
		close(u.stopCh)
		if u.animate {
			<-u.spinDone
			return
		}
		u.mu.Lock()
		fmt.Fprint(u.w, "\r\x1b[2K")
		u.mu.Unlock()
	})
}

func (u *verboseUI) CompleteMessage(msg string) {
	u.Stop()
	msg = normalizeHeader(msg, u.maxHead)
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.color {
		fmt.Fprintf(u.w, "\x1b[32m%s\x1b[0m\n", msg)
		return
	}
	fmt.Fprintln(u.w, msg)
}

func (u *verboseUI) OnProgress(ev runner.ProgressEvent) {
	u.mu.Lock()
	if ev.Total > 0 {
		u.total = ev.Total
	}
	switch ev.Phase {
	case "start":
		u.group = strings.ToUpper(ev.Group)
		u.current = normalizeHeader(fmt.Sprintf("starting check %s (%d/%d)", ev.Check, ev.Index, ev.Total), u.maxHead)
		if u.verbose {
			fmt.Fprintf(u.w, "\n%s op : %s\n", formatGroupTag(u.group, u.color), u.current)
		}
	case "end":
		if ev.Index > u.done {
			u.done = ev.Index
		}
		u.group = strings.ToUpper(ev.Group)
		u.current = normalizeHeader(fmt.Sprintf("completed status=%s check=%s", ev.Status, ev.Check), u.maxHead)
		if u.verbose {
			fmt.Fprintf(u.w, "\n%s op : %s\n", formatGroupTag(u.group, u.color), u.current)
		}
	}
	if !u.animate {
		u.frame++
		frames := []string{"|", "/", "-", "\\"}
		u.renderLocked(frames[u.frame%len(frames)])
	}
	u.mu.Unlock()
}

func (u *verboseUI) OnExecLog(group, op, msg string) {
	u.mu.Lock()
	u.group = strings.ToUpper(group)
	u.current = normalizeHeader(msg, u.maxHead)
	if u.verbose {
		fmt.Fprintf(u.w, "\n%s %s : %s\n", formatGroupTag(u.group, u.color), op, msg)
	}
	if !u.animate && !u.verbose {
		u.frame++
		frames := []string{"|", "/", "-", "\\"}
		u.renderLocked(frames[u.frame%len(frames)])
	}
	u.mu.Unlock()
}

func (u *verboseUI) renderLocked(frame string) {
	if frame == "" {
		frame = "|"
	}
	groupTag := ""
	if u.group != "" {
		groupTag = formatGroupTag(u.group, u.color)
	}
	barWidth := 24
	if u.termW > 0 && u.termW < 90 {
		barWidth = 16
	}
	if u.termW > 0 && u.termW < 70 {
		barWidth = 10
	}
	bar := progressBar(u.done, u.total, barWidth, u.color)
	headerText := u.current
	if headerText == "" {
		headerText = "running checks"
	}
	maxHead := u.maxHead
	prefix := fmt.Sprintf("[%s] %s %d/%d ", frame, bar, u.done, u.total)
	staticLen := visibleLen(prefix)
	if groupTag != "" {
		staticLen += visibleLen(groupTag) + 1
	}
	if u.termW > 0 {
		avail := u.termW - staticLen
		if avail < 12 {
			avail = 12
		}
		if maxHead > avail {
			maxHead = avail
		}
	}
	headerText = normalizeHeader(headerText, maxHead)
	header := headerText
	if groupTag != "" {
		header = groupTag + " " + headerText
	}
	fmt.Fprint(u.w, "\r\x1b[2K")
	fmt.Fprintf(u.w, "[%s] %s %d/%d %s", frame, bar, u.done, u.total, header)
}

func normalizeHeader(s string, max int) string {
	if max <= 0 {
		max = 120
	}
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 3 {
		return string(r[:max])
	}
	return string(r[:max-3]) + "..."
}

func detectTerminalWidth() int {
	raw := strings.TrimSpace(os.Getenv("COLUMNS"))
	if raw == "" {
		return 80
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 80
	}
	return n
}

func visibleLen(s string) int {
	return len([]rune(stripANSI(s)))
}

func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if !inEsc {
			if ch == 0x1b {
				inEsc = true
				continue
			}
			b.WriteByte(ch)
			continue
		}
		if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') {
			inEsc = false
		}
	}
	return b.String()
}

func progressBar(done, total, width int, color bool) string {
	if width <= 0 {
		width = 10
	}
	if total <= 0 {
		return "[" + strings.Repeat(".", width) + "]"
	}
	if done < 0 {
		done = 0
	}
	if done > total {
		done = total
	}
	filled := int(float64(done) / float64(total) * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	fill := strings.Repeat("=", filled)
	if color && filled > 0 {
		pct := float64(done) / float64(total)
		fill = colorForCompletion(pct) + fill + "\x1b[0m"
	}
	return "[" + fill + strings.Repeat(".", width-filled) + "]"
}

func colorForCompletion(pct float64) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	red := 196.0
	green := 46.0
	code := int(red + (green-red)*pct)
	return fmt.Sprintf("\x1b[38;5;%dm", code)
}

func formatGroupTag(group string, color bool) string {
	tag := "[" + strings.ToUpper(group) + "]"
	if !color {
		return tag
	}
	return groupColor(strings.ToLower(group)) + tag + "\x1b[0m"
}

func groupColor(group string) string {
	switch group {
	case "local":
		return "\x1b[38;5;39m"
	case "bandwidth":
		return "\x1b[38;5;214m"
	case "dns":
		return "\x1b[38;5;33m"
	case "http":
		return "\x1b[38;5;75m"
	case "path":
		return "\x1b[38;5;208m"
	case "reachability":
		return "\x1b[38;5;45m"
	case "bufferbloat":
		return "\x1b[38;5;171m"
	default:
		return "\x1b[38;5;250m"
	}
}
