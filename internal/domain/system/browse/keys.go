package browse

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Quit     key.Binding
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Home     key.Binding
	End      key.Binding
	Tab      key.Binding
	ShiftTab key.Binding
	Filter   key.Binding
	Enter    key.Binding
	Escape   key.Binding
	Left     key.Binding
	Right    key.Binding
}

type filterKeyMap struct {
	Left  key.Binding
	Right key.Binding
}

var keys = keyMap{
	Quit:     key.NewBinding(key.WithKeys("q", "Q", "ctrl+c")),
	Up:       key.NewBinding(key.WithKeys("up", "k")),
	Down:     key.NewBinding(key.WithKeys("down", "j")),
	PageUp:   key.NewBinding(key.WithKeys("pgup", "ctrl+u")),
	PageDown: key.NewBinding(key.WithKeys("pgdown", "ctrl+d")),
	Home:     key.NewBinding(key.WithKeys("home", "ctrl+home", "g")),
	End:      key.NewBinding(key.WithKeys("end", "ctrl+end", "G")),
	Tab:      key.NewBinding(key.WithKeys("tab")),
	ShiftTab: key.NewBinding(key.WithKeys("shift+tab")),
	Filter:   key.NewBinding(key.WithKeys("f", "F")),
	Enter:    key.NewBinding(key.WithKeys("enter")),
	Escape:   key.NewBinding(key.WithKeys("esc")),
	Left:     key.NewBinding(key.WithKeys("left", "h")),
	Right:    key.NewBinding(key.WithKeys("right", "l")),
}

var fkeys = filterKeyMap{
	Left:  key.NewBinding(key.WithKeys("left")),
	Right: key.NewBinding(key.WithKeys("right")),
}
