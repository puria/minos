package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Quit      key.Binding
	FocusNext key.Binding
	FocusPrev key.Binding
	Up        key.Binding
	Down      key.Binding
	Refresh   key.Binding
	Sort      key.Binding
	Filter    key.Binding
	ToggleSel key.Binding
	Delete    key.Binding
	Force     key.Binding
	Help      key.Binding
	Summary   key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		FocusNext: key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next pane")),
		FocusPrev: key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev pane")),
		Up:        key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("k/up", "up")),
		Down:      key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("j/down", "down")),
		Refresh:   key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		Sort:      key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sort")),
		Filter:    key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		ToggleSel: key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "toggle select")),
		Delete:    key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete/remove")),
		Force:     key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "force action")),
		Help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Summary:   key.NewBinding(key.WithKeys("a", "e", "."), key.WithHelp("e/.", "explain")),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Quit, k.FocusNext, k.Up, k.Down, k.Delete, k.Refresh, k.Help}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Quit, k.FocusNext, k.FocusPrev, k.Up, k.Down},
		{k.Filter, k.Sort, k.ToggleSel, k.Delete, k.Force, k.Summary, k.Help},
	}
}
