package browser

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hazelcast/hazelcast-go-client"
	"github.com/muesli/termenv"

	"github.com/hazelcast/hazelcast-commandline-client/internal/browser/layout/vertical"
	"github.com/hazelcast/hazelcast-commandline-client/internal/browser/multiline"
	"github.com/hazelcast/hazelcast-commandline-client/internal/termdbms/tuiutil"
	"github.com/hazelcast/hazelcast-commandline-client/internal/termdbms/viewer"
)

type StringResultMsg string
type TableResultMsg *sql.Rows

type controller struct {
	tea.Model
	client *hazelcast.Client
}

type table struct {
	termdbmsTable viewer.TuiModel
	keyboardFocus bool
	lastIteration *SqlIterator
}

func (t *table) Init() tea.Cmd {
	tuiutil.Faint = true
	if lipgloss.ColorProfile() == termenv.Ascii {
		tuiutil.Ascii = true
		lipgloss.SetColorProfile(termenv.Ascii)
	}
	viewer.GlobalCommands["j"] = viewer.GlobalCommands["s"]
	viewer.GlobalCommands["k"] = viewer.GlobalCommands["w"]
	viewer.GlobalCommands["a"] = viewer.GlobalCommands["left"]
	viewer.GlobalCommands["d"] = viewer.GlobalCommands["right"]
	viewer.GlobalCommands["down"] = viewer.GlobalCommands["s"]
	viewer.GlobalCommands["up"] = viewer.GlobalCommands["w"]
	t.termdbmsTable = viewer.GetNewModel("", nil)
	t.termdbmsTable.UI.BorderToggle = true
	viewer.HeaderStyle.Bold(true)
	return t.termdbmsTable.Init()
}

type FetchMoreRowsMsg struct{}

func (t *table) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case StringResultMsg:
		// update table
		t.termdbmsTable.UI.RenderSelection = true
		t.termdbmsTable.Data().EditTextBuffer = string(m)
		return t, nil
	case TableResultMsg:
		t.termdbmsTable.UI.RenderSelection = false
		t.termdbmsTable.Data().EditTextBuffer = ""
		t.termdbmsTable.QueryData = &viewer.UIData{
			TableHeaders:      make(map[string][]string),
			TableHeadersSlice: []string{},
			TableSlices:       make(map[string][]interface{}),
			TableIndexMap:     make(map[int]string),
		}
		t.termdbmsTable.QueryResult = &viewer.TableState{ // perform query
			Database: t.termdbmsTable.Table().Database,
			Data:     make(map[string]interface{}),
		}
		t.termdbmsTable.MouseData = tea.MouseEvent{}
		t.lastIteration = NewSqlIterator(50, m)
		return t, t.lastIteration.IterateCmd(50 * time.Millisecond)
	case NewRowsMessage:
		t.PopulateDataForResult(m)
		t.termdbmsTable.UI.CurrentTable = 1
		_ = t.termdbmsTable.NumHeaders() // to set maxHeaders global var, for side effect
		t.termdbmsTable.SetViewSlices()
		var cmd tea.Cmd
		if !t.lastIteration.rowsFinished {
			cmd = t.lastIteration.IterateCmd(50 * time.Millisecond)
		}
		return t, cmd
	case FetchMoreRowsMsg:
		if t.lastIteration.rowsFinished {
			return t, nil
		}
		if t.lastIteration.Iterating {
			return t, nil
		}
		go t.lastIteration.Iterate(50)
		return t, t.lastIteration.IterateCmd(50 * time.Millisecond)
	case tea.KeyMsg:
		switch m.Type {
		case tea.KeyTab:
			t.keyboardFocus = !t.keyboardFocus
			tuiutil.Faint = !tuiutil.Faint
			return t, nil
		case tea.KeyCtrlC:
			if t.lastIteration != nil {
				t.lastIteration.rows.Close()
			}
			return t, nil
		}
		if !t.keyboardFocus {
			return t, nil
		}
	case tea.MouseMsg:
		// disable all mouse events
		return t, nil
	case tea.WindowSizeMsg:
		if m.Height > 0 {
			m.Height += -2 // footer, header height offset
		}
		msg = m
	}
	oldYOffset := t.termdbmsTable.Viewport.YOffset + t.termdbmsTable.GetRow()
	tmp, cmd := t.termdbmsTable.Update(msg)
	t.termdbmsTable = tmp.(viewer.TuiModel)
	newYOffset := t.termdbmsTable.Viewport.YOffset + t.termdbmsTable.GetRow()
	if t.lastIteration != nil {
		userOnLastPage := newYOffset > t.lastIteration.totalProcessedLines-t.termdbmsTable.Viewport.Height
		if newYOffset > oldYOffset {
			if userOnLastPage {
				cmd = tea.Batch(cmd, func() tea.Msg {
					return FetchMoreRowsMsg{}
				})
			}
		}

	}
	return t, cmd
}

type SqlIterator struct {
	rows                *sql.Rows
	resultPipe          chan []interface{}
	rowsFinished        bool
	totalProcessedLines int
	columnNames         []string // cannot access these after rows.Close(), hence save them
	Iterating           bool
}

func NewSqlIterator(maxIterationCount int, rows *sql.Rows) *SqlIterator {
	var si SqlIterator
	si.rows = rows
	si.columnNames, _ = rows.Columns()
	si.resultPipe = make(chan []interface{}, maxIterationCount+1)
	go si.Iterate(maxIterationCount)
	return &si
}

func (si *SqlIterator) Iterate(maxIterationCount int) {
	si.Iterating = true
	var i int
	for i = 0; si.rows.Next() && i < maxIterationCount; i++ { // each row of the table
		// golang wizardry
		columns := make([]interface{}, len(si.columnNames))
		columnPointers := make([]interface{}, len(si.columnNames))
		// init interface array
		for i := range columns {
			columnPointers[i] = &columns[i]
		}
		if err := si.rows.Scan(columnPointers...); err != nil {
			break
		}
		si.resultPipe <- columnPointers
	}
	if i != maxIterationCount { // means query finished and there will be no more results
		close(si.resultPipe)
	}
	si.Iterating = false
}

type NewRowsMessage [][]interface{}

func (si *SqlIterator) IterateCmd(timeline time.Duration) func() tea.Msg {
	return func() tea.Msg {
		var newRows [][]interface{}
		var timeout bool
		for {
			select {
			case row := <-si.resultPipe:
				if row == nil {
					si.rowsFinished = true
					return NewRowsMessage(newRows)
				}
				si.totalProcessedLines++
				newRows = append(newRows, row)
			case <-time.After(timeline):
				timeout = true
				break
			}
			if timeout {
				break
			}
		}
		return NewRowsMessage(newRows)
	}
}

func (m *table) PopulateDataForResult(rows [][]interface{}) {
	columnNames := m.lastIteration.columnNames
	columnValues := make(map[string][]interface{})
	if m.termdbmsTable.QueryResult != nil && m.termdbmsTable.QueryData != nil {
		if m.termdbmsTable.QueryResult.Data["0"] != nil {
			columnValues = m.termdbmsTable.QueryResult.Data["0"].(map[string][]interface{})
		}
	}

	for _, row := range rows {
		for i, colName := range columnNames {
			val := row[i].(*interface{})
			columnValues[colName] = append(columnValues[colName], *val)
		}
	}

	// onto the next schema
	if m.termdbmsTable.QueryResult != nil && m.termdbmsTable.QueryData != nil {
		m.termdbmsTable.QueryResult.Data["0"] = columnValues
		m.termdbmsTable.QueryData.TableHeaders["0"] = columnNames // headers for the schema, for later reference
		m.termdbmsTable.QueryData.TableIndexMap[1] = "0"
		return
	}
	m.termdbmsTable.Table().Data["0"] = columnValues       // data for schema, organized by column
	m.termdbmsTable.Data().TableHeaders["0"] = columnNames // headers for the schema, for later reference
	// mapping between schema and an int ( since maps aren't deterministic), for later reference
	m.termdbmsTable.Data().TableIndexMap[1] = "0"
}

func (t *table) View() string {
	done := make(chan bool, 2)
	defer close(done) // close
	var header, content string
	// body
	go func(c *string) {
		*c = viewer.AssembleTable(&t.termdbmsTable)
		done <- true
	}(&content)
	// header
	go viewer.HeaderAssembly(&t.termdbmsTable, &header, &done)
	<-done
	<-done
	if content == "" {
		content = strings.Repeat("\n", t.termdbmsTable.Viewport.Height)
	}
	return fmt.Sprintf("%s\n%s", header, content)
}

type Shortcut struct {
	key         string
	description string
}

type Help struct {
	width  int
	values []Shortcut
	align  lipgloss.Position
}

func (h Help) Init() tea.Cmd {
	return nil
}

func (h Help) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.WindowSizeMsg); ok {
		h.width = msg.Width - 2
	}
	return h, nil
}

func (h Help) View() string {
	base := lipgloss.NewStyle()
	sh := base.Copy().Background(lipgloss.Color(tuiutil.Highlight())).Foreground(lipgloss.Color("#000000"))
	def := base.Copy()
	var b strings.Builder
	for _, v := range h.values {
		b.WriteString(sh.Render(fmt.Sprintf(" %s ", v.key)))
		b.WriteString(" - ")
		b.WriteString(def.Render(v.description))
		b.WriteString("      ")
	}
	return b.String()
}

type Separator int

func (s Separator) Init() tea.Cmd {
	return nil
}

func (s Separator) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.WindowSizeMsg); ok {
		s = Separator(msg.Width - 2)
	}
	return s, nil
}

func (s Separator) View() string {
	return strings.Repeat("─", max(0, int(s)))
}

func (c controller) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case multiline.SubmitMsg:
		return c, func() tea.Msg {
			lt := strings.TrimSpace(string(m))
			rows, err := execSQL(c.client, lt)
			if err != nil {
				return StringResultMsg(err.Error())
			}
			return TableResultMsg(rows)
		}
	}
	var cmd tea.Cmd
	c.Model, cmd = c.Model.Update(msg)
	return c, cmd
}

func InitSQLBrowser(client *hazelcast.Client) *tea.Program {
	var s Separator
	textArea := multiline.InitTextArea()
	keys := make(map[string]string)
	keys["^-x"] = "execute"
	keys["^-c"] = "quit"
	keys["tab"] = "toggle focus"
	c := &controller{vertical.InitialModel([]tea.Model{
		&table{},
		s,
		textArea,
		Help{
			values: []Shortcut{
				{
					"^E",
					"execute",
				},
				{
					"^Q",
					"quit",
				},
				{
					"Tab",
					"toggle focus",
				},
				{
					"^V",
					"paste",
				},
				{
					"^C",
					"cancel query",
				},
				{
					"^U",
					"clear query",
				},
			},
			align: lipgloss.Left,
		},
	}, []int{3, -1, 1, -1}), client}
	p := tea.NewProgram(
		c,
		tea.WithMouseCellMotion(), // turn on mouse support, so we can track the mouse wheel
	)
	return p
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}