package terminal

import (
	"strings"
	"time"
)

const (
	DefaultMaxConsoleEvents = 150
	F12KeyName              = "F12"
)

type ConsoleEvent struct {
	Timestamp time.Time
	Kind      string
	Source    string
	Message   string
}

type EventConsole struct {
	isVisible bool
	maxItems  int
	items     []ConsoleEvent
}

func NewEventConsole(maxItems int) *EventConsole {
	if maxItems <= 0 {
		maxItems = DefaultMaxConsoleEvents
	}
	return &EventConsole{
		maxItems: maxItems,
		items:    make([]ConsoleEvent, 0, maxItems),
	}
}

func (c *EventConsole) Toggle() bool {
	if c == nil {
		return false
	}
	c.isVisible = !c.isVisible
	return c.isVisible
}

func (c *EventConsole) SetVisible(visible bool) {
	if c == nil {
		return
	}
	c.isVisible = visible
}

func (c *EventConsole) IsVisible() bool {
	if c == nil {
		return false
	}
	return c.isVisible
}

func (c *EventConsole) Append(kind, source, message string) {
	c.AppendEvent(ConsoleEvent{
		Timestamp: time.Now(),
		Kind:      kind,
		Source:    source,
		Message:   message,
	})
}

func (c *EventConsole) AppendEvent(item ConsoleEvent) {
	if c == nil {
		return
	}
	if c.maxItems <= 0 {
		return
	}

	if item.Timestamp.IsZero() {
		item.Timestamp = time.Now()
	}
	if len(c.items) >= c.maxItems {
		copy(c.items, c.items[1:])
		c.items = c.items[:c.maxItems-1]
	}
	c.items = append(c.items, item)
}

func (c *EventConsole) Items() []ConsoleEvent {
	if c == nil {
		return nil
	}
	cloned := make([]ConsoleEvent, len(c.items))
	copy(cloned, c.items)
	return cloned
}

func (c *EventConsole) HandleShortcut(keyName string) bool {
	switch strings.ToUpper(strings.TrimSpace(keyName)) {
	case F12KeyName, "VK_F12":
		c.Toggle()
		return true
	default:
		return false
	}
}
