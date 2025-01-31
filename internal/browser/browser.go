package browser

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hazelcast/hazelcast-go-client"
	"github.com/hazelcast/hazelcast-go-client/sql"
	"github.com/muesli/termenv"

	"github.com/hazelcast/hazelcast-commandline-client/internal/browser/layout/vertical"
	"github.com/hazelcast/hazelcast-commandline-client/internal/browser/multiline"
	"github.com/hazelcast/hazelcast-commandline-client/internal/termdbms/viewer"
	"github.com/hazelcast/hazelcast-commandline-client/internal/tuiutil"
)

type StringResultMsg string
type TableResultMsg sql.Result

type controller struct {
	tea.Model
	client *hazelcast.Client
}

type table struct {
	termdbmsTable viewer.TuiModel
	keyboardFocus bool
	lastIteration *SQLIterator
}

func (t *table) Init() tea.Cmd {
	tuiutil.Faint = true
	if lipgloss.ColorProfile() == termenv.Ascii || tuiutil.SelectedTheme == tuiutil.NoColor {
		tuiutil.Ascii = true
		tuiutil.SelectedTheme = tuiutil.NoColor
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
		t.termdbmsTable.QueryResult = &viewer.TableState{
			Database: t.termdbmsTable.Table().Database,
			Data:     make(map[string]interface{}),
		}
		t.termdbmsTable.MouseData = tea.MouseEvent{}
		var err error
		if t.lastIteration, err = NewSqlIterator(50, m); err != nil {
			return t, nil
		}
		return t, t.lastIteration.ConsumeRowsCmd(50 * time.Millisecond)
	case NewRowsMessage:
		t.PopulateDataForResult(m)
		t.termdbmsTable.UI.CurrentTable = 1
		_ = t.termdbmsTable.NumHeaders() // to set maxHeaders global var, for side effect
		t.termdbmsTable.SetViewSlices()
		var cmd tea.Cmd
		if !t.lastIteration.rowsFinished {
			cmd = t.lastIteration.ConsumeRowsCmd(50 * time.Millisecond)
		}
		return t, cmd
	case FetchMoreRowsMsg:
		if t.lastIteration.rowsFinished || atomic.LoadInt32(&t.lastIteration.iterating) == set {
			return t, nil
		}
		go t.lastIteration.Iterate(50)
		return t, t.lastIteration.ConsumeRowsCmd(50 * time.Millisecond)
	case tea.KeyMsg:
		switch m.Type {
		case tea.KeyTab:
			t.keyboardFocus = !t.keyboardFocus
			tuiutil.Faint = !tuiutil.Faint
			return t, nil
		case tea.KeyCtrlC:
			if atomic.CompareAndSwapInt32(&t.lastIteration.queryStatus, unset, closed) {
				// go client halts at the Close call if there is no member to connect
				// this is a hacky work around
				go func() {
					t.lastIteration.result.Close()
				}()
				changeProgress(HideProgress)
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
		msg = m
	}
	var cmd tea.Cmd
	oldYOffset := t.termdbmsTable.Viewport.YOffset + t.termdbmsTable.GetRow()
	t.termdbmsTable, cmd = t.termdbmsTable.Update(msg)
	newYOffset := t.termdbmsTable.Viewport.YOffset + t.termdbmsTable.GetRow()
	if t.lastIteration != nil {
		userOnLastPage := newYOffset > t.lastIteration.totalProcessedLines-t.termdbmsTable.Viewport.Height
		if newYOffset > oldYOffset && userOnLastPage {
			cmd = tea.Batch(cmd, func() tea.Msg {
				return FetchMoreRowsMsg{}
			})
		}
	}
	return t, cmd
}

type NewRowsMessage [][]interface{}

const (
	unset  = 0
	set    = 1
	closed = 2
)

type SQLIterator struct {
	it                  sql.RowsIterator
	result              sql.Result
	resultPipe          chan []interface{}
	rowsFinished        bool
	totalProcessedLines int
	columnNames         []string // cannot access these after it.Close(), hence save them
	iterating           int32
	consumingRows       int32
	queryStatus         int32
}

func NewSqlIterator(maxIterationCount int, result sql.Result) (*SQLIterator, error) {
	var si SQLIterator
	var err error
	if si.it, err = result.Iterator(); err != nil {
		return nil, err
	}
	si.result = result
	mt, err := result.RowMetadata()
	if err != nil {
		return nil, err
	}
	for _, c := range mt.Columns() {
		si.columnNames = append(si.columnNames, c.Name())
	}
	si.resultPipe = make(chan []interface{}, maxIterationCount+1)
	go si.Iterate(maxIterationCount)
	return &si, nil
}

func (si *SQLIterator) Iterate(maxIterationCount int) {
	atomic.StoreInt32(&si.iterating, set)
	defer atomic.StoreInt32(&si.iterating, unset)
	var i int
	for i = 0; si.it.HasNext() && i < maxIterationCount; i++ {
		rows, err := si.it.Next()
		if err != nil {
			changeProgress(HideProgress)
			break
		}
		var values []interface{}
		for i := 0; i < rows.Metadata().ColumnCount(); i++ {
			r, err := rows.Get(i)
			if err != nil {
				// this should never happen
				panic("can not access row value")
			}
			values = append(values, r)
		}
		si.resultPipe <- values
	}
	if i < maxIterationCount {
		changeProgress(HideProgress)
		// means query finished and there will be no more results
		close(si.resultPipe)
	}
}

func (si *SQLIterator) ConsumeRowsCmd(deadline time.Duration) func() tea.Msg {
	return func() tea.Msg {
		if !atomic.CompareAndSwapInt32(&si.consumingRows, 0, 1) {
			// already iterating
			return nil
		}
		defer atomic.CompareAndSwapInt32(&si.consumingRows, 1, 0)
		timer := time.NewTimer(deadline)
		defer timer.Stop()
		var newRows [][]interface{}
	loop:
		for {
			select {
			case row, ok := <-si.resultPipe:
				if !ok {
					si.rowsFinished = true
					return NewRowsMessage(newRows)
				}
				si.totalProcessedLines++
				newRows = append(newRows, row)
			case <-timer.C:
				break loop
			}
		}
		return NewRowsMessage(newRows)
	}
}

func (m *table) PopulateDataForResult(rows [][]interface{}) {
	columnNames := m.lastIteration.columnNames
	columnValues := make(map[string][]interface{})
	if m.termdbmsTable.QueryResult != nil && m.termdbmsTable.QueryData != nil && m.termdbmsTable.QueryResult.Data["0"] != nil {
		columnValues = m.termdbmsTable.QueryResult.Data["0"].(map[string][]interface{})
	}
	for _, row := range rows {
		for i, colName := range columnNames {
			columnValues[colName] = append(columnValues[colName], row[i])
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
	var wg sync.WaitGroup
	wg.Add(2)
	var header, content string
	// body
	go func(c *string) {
		*c = viewer.AssembleTable(&t.termdbmsTable)
		wg.Done()
	}(&content)
	// header
	go func() {
		viewer.HeaderAssembly(&t.termdbmsTable, &header)
		wg.Done()
	}()
	wg.Wait()
	if content == "" && t.termdbmsTable.Viewport.Height > 0 {
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
		h.width = msg.Width
	}
	return h, nil
}

func (h Help) View() string {
	base := lipgloss.NewStyle().Foreground(tuiutil.FooterForeground())
	reversed := base.Copy().Reverse(true)
	var b strings.Builder
	for _, v := range h.values {
		b.WriteString(reversed.Render(fmt.Sprintf(" %s ", v.key)))
		b.WriteString(" - ")
		b.WriteString(base.Render(v.description))
		b.WriteString("      ")
	}
	return b.String()
}

func (c controller) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case multiline.SubmitMsg:
		return c, func() tea.Msg {
			q := strings.TrimSpace(string(m))
			if q == "" {
				return nil
			}
			if strings.HasPrefix(q, "select") || strings.HasPrefix(q, "show") {
				result, err := c.client.SQL().Execute(context.TODO(), q)
				if err != nil {
					return StringResultMsg(err.Error())
				}
				changeProgress(ShowProgress)
				return TableResultMsg(result)
			}
			result, err := c.client.SQL().Execute(context.TODO(), q)
			if err != nil {
				return StringResultMsg(err.Error())
			}
			return StringResultMsg(fmt.Sprintf("Affected Rows: %d", result.UpdateCount()))
		}
	}
	var cmd tea.Cmd
	c.Model, cmd = c.Model.Update(msg)
	return c, cmd
}

func InitSQLBrowser(client *hazelcast.Client, in io.Reader, out io.Writer) *tea.Program {
	var s SeparatorWithProgress
	textArea := multiline.InitTextArea()
	table := &table{}
	c := &controller{vertical.InitialModel([]tea.Model{
		table,
		&s,
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
		tea.WithOutput(out),
		tea.WithInput(in),
	)
	return p
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
