package connwizardcmd

import (
	"fmt"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"io"
)

var choice = ""

type item string

func (i item) FilterValue() string {
	return string(i)
}

type itemDelegate struct{}

func (d itemDelegate) Height() int                               { return 1 }
func (d itemDelegate) Spacing() int                              { return 0 }
func (d itemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }

func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(item)
	if !ok {
		return
	}
	str := fmt.Sprintf("%d. %s", index+1, i)
	fn := noStyle.Render
	if index == m.Index() {
		fn = func(s string) string {
			return selectedItemStyle.Render(s)
		}
	}
	fmt.Fprintf(w, fn(str))
}

type ListModel struct {
	list     list.Model
	quitting bool
}

func InitializeListModel() ListModel {
	items := []list.Item{
		item("Hazelcast Viridian"),
		item("Local (Default)"),
		item("Standalone (Remote or Local)"),
	}
	l := list.New(items, itemDelegate{}, 20, 14)
	l.Title = "Where is your Hazelcast cluster (Press Ctrl+C to quit)?"
	l.Styles.Title = noStyle
	l.Styles.TitleBar = noStyle
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)
	return ListModel{
		l,
		false,
	}
}

func (m ListModel) Init() tea.Cmd {

	return nil
}

func (m ListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			choice = "e"
			m.quitting = true
			return m, tea.Quit
		case tea.KeyEnter:
			m.quitting = true
			value, _ := m.list.SelectedItem().(item)
			choice = value.FilterValue()
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m ListModel) View() string {
	if m.quitting {
		return fmt.Sprintf("%s", noStyle.Render(""))
	}
	return m.list.View()
}
