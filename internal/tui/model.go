package tui

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/php-workx/epos/ticket"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const (
	narrowWidth = 90
	footerLines = 2
)

const (
	keyEnter = "enter"
	keyEsc   = "esc"
)

const detailFocusStatus = "detail focused; j/k scroll, esc returns to list"

type promptMode int

const (
	promptNone promptMode = iota
	promptNote
)

type tuiKeyMap struct {
	Up       key.Binding
	Down     key.Binding
	Group    key.Binding
	Prev     key.Binding
	Back     key.Binding
	Search   key.Binding
	Refresh  key.Binding
	Detail   key.Binding
	Children key.Binding
	Note     key.Binding
	Claim    key.Binding
	Release  key.Binding
	Close    key.Binding
	Reopen   key.Binding
	Help     key.Binding
	Quit     key.Binding
}

func (k *tuiKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Group, k.Detail, k.Children, k.Back, k.Search, k.Note, k.Claim, k.Close, k.Help, k.Quit}
}

func (k *tuiKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Group, k.Prev, k.Back},
		{k.Search, k.Refresh, k.Detail, k.Children, k.Note},
		{k.Claim, k.Release, k.Close, k.Reopen},
		{k.Help, k.Quit},
	}
}

var defaultKeys = tuiKeyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("k/up", "move"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("j/down", "move"),
	),
	Group: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "group"),
	),
	Prev: key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("shift+tab", "previous group"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc", "backspace"),
		key.WithHelp("esc", "back"),
	),
	Search: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "search"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	),
	Detail: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "detail"),
	),
	Children: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("l/right", "children"),
	),
	Note: key.NewBinding(
		key.WithKeys("n"),
		key.WithHelp("n", "note"),
	),
	Claim: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "claim"),
	),
	Release: key.NewBinding(
		key.WithKeys("u"),
		key.WithHelp("u", "release"),
	),
	Close: key.NewBinding(
		key.WithKeys("x"),
		key.WithHelp("x", "close"),
	),
	Reopen: key.NewBinding(
		key.WithKeys("o"),
		key.WithHelp("o", "reopen"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
}

// Config contains TUI runtime settings.
type Config struct {
	DataSource DataSource
	Parent     string
	Owner      string
	Refresh    time.Duration
}

// Model is the Bubble Tea application state.
type Model struct {
	ds      DataSource
	parent  string
	owner   string
	refresh time.Duration
	stack   []string

	snapshot Snapshot
	groupIdx int
	selected int

	width  int
	height int

	search *textinput.Model
	prompt *textinput.Model
	detail *viewport.Model
	list   *list.Model
	help   *help.Model
	spin   *spinner.Model
	keys   *tuiKeyMap

	searching bool
	helpOn    bool
	detailOn  bool
	prompting promptMode
	busy      bool
	status    string
	err       string
}

type snapshotMsg struct {
	snapshot Snapshot
	status   string
	err      error
}

type tickMsg time.Time

// NewModel constructs a TUI model.
func NewModel(cfg Config) *Model {
	search := textinput.New()
	search.Prompt = "/ "
	search.Placeholder = "filter tickets"
	search.SetWidth(40)

	prompt := textinput.New()
	prompt.Prompt = "> "
	prompt.SetWidth(60)

	detail := viewport.New()
	detail.SoftWrap = true

	tickets := list.New(nil, ticketDelegate{}, 40, 12)
	tickets.Title = ""
	tickets.SetShowTitle(false)
	tickets.SetShowStatusBar(false)
	tickets.SetShowHelp(false)
	tickets.SetFilteringEnabled(false)
	tickets.SetShowPagination(true)
	tickets.SetStatusBarItemName("ticket", "tickets")
	tickets.DisableQuitKeybindings()
	tickets.Styles = listStyles()

	helpModel := help.New()
	helpModel.Styles = help.DefaultStyles(true)
	helpModel.Styles.ShortKey = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)
	helpModel.Styles.ShortDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	helpModel.Styles.ShortSeparator = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	helpModel.Styles.FullKey = helpModel.Styles.ShortKey
	helpModel.Styles.FullDesc = helpModel.Styles.ShortDesc
	helpModel.Styles.FullSeparator = helpModel.Styles.ShortSeparator

	spin := spinner.New(
		spinner.WithSpinner(spinner.MiniDot),
		spinner.WithStyle(accentStyle),
	)

	return &Model{
		ds:       cfg.DataSource,
		parent:   cfg.Parent,
		owner:    cfg.Owner,
		refresh:  cfg.Refresh,
		snapshot: newSnapshot(),
		search:   &search,
		prompt:   &prompt,
		detail:   &detail,
		list:     &tickets,
		help:     &helpModel,
		spin:     &spin,
		keys:     &defaultKeys,
		width:    100,
		height:   30,
		busy:     true,
	}
}

// Init loads the first snapshot and starts polling if configured.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.loadSnapshot(), m.tick(), m.spin.Tick)
}

// Update applies Bubble Tea messages to the model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case snapshotMsg:
		m.busy = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.err = ""
		if msg.status != "" {
			m.status = msg.status
		}
		m.WithSnapshot(msg.snapshot)
		return m, nil
	case tickMsg:
		m.busy = true
		m.status = "refreshing"
		return m, tea.Batch(m.loadSnapshot(), m.tick(), m.spin.Tick)
	case spinner.TickMsg:
		if !m.busy {
			return m, nil
		}
		next, cmd := m.spin.Update(msg)
		m.spin = &next
		return m, cmd
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		return m, nil
	case tea.KeyPressMsg:
		return m.updateKey(msg)
	}
	return m, nil
}

// View renders the TUI.
func (m *Model) View() tea.View {
	body := m.render()
	v := tea.NewView(body)
	v.AltScreen = true
	return v
}

// WithSnapshot replaces the snapshot and preserves selection by ticket ID.
func (m *Model) WithSnapshot(snapshot Snapshot) *Model {
	if snapshot.Groups == nil {
		snapshot = newSnapshot()
	}
	snapshot = cloneSnapshot(snapshot)
	previousID := m.SelectedID()
	m.snapshot = snapshot
	if m.groupIdx >= len(Groups) {
		m.groupIdx = 0
	}
	m.refreshList(previousID)
	m.updateDetailContent()
	return m
}

func cloneSnapshot(snapshot Snapshot) Snapshot {
	out := Snapshot{
		Groups:  make(map[Group][]TicketRow, len(snapshot.Groups)),
		Counts:  make(map[Group]int, len(snapshot.Counts)),
		Rows:    append([]TicketRow(nil), snapshot.Rows...),
		Tickets: make(map[string]ticket.Ticket, len(snapshot.Tickets)),
		Runtime: make(map[string]*ticket.RuntimeState, len(snapshot.Runtime)),
	}
	for group, rows := range snapshot.Groups {
		out.Groups[group] = append([]TicketRow(nil), rows...)
	}
	for group, count := range snapshot.Counts {
		out.Counts[group] = count
	}
	for id := range snapshot.Tickets {
		out.Tickets[id] = snapshot.Tickets[id]
	}
	for id, state := range snapshot.Runtime {
		out.Runtime[id] = state
	}
	return out
}

// ActiveGroup returns the currently selected group.
func (m *Model) ActiveGroup() Group {
	if m.groupIdx < 0 || m.groupIdx >= len(Groups) {
		return GroupReady
	}
	return Groups[m.groupIdx]
}

// SelectedID returns the active row's ticket ID.
func (m *Model) SelectedID() string {
	row := m.selectedRow()
	if row == nil {
		return ""
	}
	return row.ID
}

// SearchFocused reports whether the search input is active.
func (m *Model) SearchFocused() bool { return m.searching }

// SearchValue returns the current search query.
func (m *Model) SearchValue() string { return m.search.Value() }

// HelpVisible reports whether the help overlay is active.
func (m *Model) HelpVisible() bool { return m.helpOn }

// DetailFocused reports whether the detail pane is active.
func (m *Model) DetailFocused() bool { return m.detailOn }

// ErrorMessage returns the current transient error.
func (m *Model) ErrorMessage() string { return m.err }

func (m *Model) updateKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.prompting != promptNone {
		return m.updatePrompt(msg)
	}
	if m.searching {
		return m.updateSearch(msg)
	}
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case msg.String() == keyEsc:
		if m.helpOn {
			m.helpOn = false
			m.help.ShowAll = false
			return m, nil
		}
		if m.detailOn {
			m.detailOn = false
			if m.status == detailFocusStatus {
				m.status = ""
			}
			return m, nil
		}
		if len(m.stack) > 0 {
			m.parent = m.stack[len(m.stack)-1]
			m.stack = m.stack[:len(m.stack)-1]
			m.search.SetValue("")
			m.searching = false
			m.busy = true
			m.status = "opening parent"
			return m, tea.Batch(m.loadSnapshot(), m.spin.Tick)
		}
	case key.Matches(msg, m.keys.Help):
		m.helpOn = !m.helpOn
		m.help.ShowAll = m.helpOn
	case key.Matches(msg, m.keys.Search):
		m.searching = true
		return m, m.search.Focus()
	default:
		return m.updateActiveKey(msg)
	}
	return m, nil
}

func (m *Model) updateActiveKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.detailOn {
		return m.updateDetailKey(msg)
	}

	switch {
	case key.Matches(msg, m.keys.Up, m.keys.Down):
		next, cmd := m.list.Update(msg)
		m.list = &next
		m.syncSelectedFromList()
		m.updateDetailContent()
		return m, cmd
	case key.Matches(msg, m.keys.Group):
		m.groupIdx = (m.groupIdx + 1) % len(Groups)
		m.refreshList("")
		m.updateDetailContent()
	case key.Matches(msg, m.keys.Prev):
		m.groupIdx = (m.groupIdx - 1 + len(Groups)) % len(Groups)
		m.refreshList("")
		m.updateDetailContent()
	case key.Matches(msg, m.keys.Detail):
		m.openDetail()
	case key.Matches(msg, m.keys.Children):
		return m.openChildren()
	case key.Matches(msg, m.keys.Refresh):
		m.busy = true
		m.status = "refreshing"
		cmd := m.loadSnapshot()
		return m, tea.Batch(cmd, m.spin.Tick)
	case key.Matches(msg, m.keys.Note):
		if m.SelectedID() != "" {
			m.prompting = promptNote
			m.prompt.Placeholder = "note text"
			m.prompt.SetValue("")
			return m, m.prompt.Focus()
		}
	case key.Matches(msg, m.keys.Claim):
		cmd := m.mutate("claim", func(id string) error {
			if m.owner == "" {
				return fmt.Errorf("owner is required to claim")
			}
			return m.ds.Claim(id, m.owner)
		})
		return m, cmd
	case key.Matches(msg, m.keys.Release):
		cmd := m.mutate("release", func(id string) error {
			if m.owner == "" {
				return fmt.Errorf("owner is required to release")
			}
			return m.ds.Release(id, m.owner)
		})
		return m, cmd
	case key.Matches(msg, m.keys.Close):
		cmd := m.mutate("close", func(id string) error {
			return m.ds.Close(id, "closed from tui")
		})
		return m, cmd
	case key.Matches(msg, m.keys.Reopen):
		cmd := m.mutate("reopen", func(id string) error {
			return m.ds.Reopen(id, "reopened from tui")
		})
		return m, cmd
	}
	return m, nil
}

func (m *Model) updateDetailKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Down):
		m.detail.ScrollDown(1)
		return m, nil
	case key.Matches(msg, m.keys.Up):
		m.detail.ScrollUp(1)
		return m, nil
	}
	next, cmd := m.detail.Update(msg)
	m.detail = &next
	return m, cmd
}

func (m *Model) openDetail() {
	row := m.selectedRow()
	if row != nil && row.ID != "" {
		m.detailOn = true
		m.status = detailFocusStatus
	}
}

func (m *Model) openChildren() (tea.Model, tea.Cmd) {
	row := m.selectedRow()
	if row == nil || row.ChildCount == 0 {
		return m, nil
	}
	m.stack = append(m.stack, m.parent)
	m.parent = row.ID
	m.search.SetValue("")
	m.searching = false
	m.busy = true
	m.status = "opening " + row.ID
	return m, tea.Batch(m.loadSnapshot(), m.spin.Tick)
}

func (m *Model) updateSearch(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case keyEsc:
		m.searching = false
		m.search.Blur()
		m.search.SetValue("")
		m.refreshList("")
		m.updateDetailContent()
		return m, nil
	case keyEnter:
		m.searching = false
		m.search.Blur()
		return m, nil
	}
	next, cmd := m.search.Update(msg)
	m.search = &next
	m.refreshList(m.SelectedID())
	m.updateDetailContent()
	return m, cmd
}

func (m *Model) updatePrompt(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case keyEsc:
		m.prompting = promptNone
		m.prompt.Blur()
		m.prompt.SetValue("")
		return m, nil
	case keyEnter:
		text := strings.TrimSpace(m.prompt.Value())
		m.prompting = promptNone
		m.prompt.Blur()
		m.prompt.SetValue("")
		if text == "" {
			return m, nil
		}
		cmd := m.mutate("note", func(id string) error {
			return m.ds.AddNote(id, text)
		})
		return m, cmd
	}
	next, cmd := m.prompt.Update(msg)
	m.prompt = &next
	return m, cmd
}

func (m *Model) mutate(name string, fn func(string) error) tea.Cmd {
	id := m.SelectedID()
	if id == "" {
		return nil
	}
	m.busy = true
	m.status = name + " " + id
	return func() tea.Msg {
		if err := fn(id); err != nil {
			return snapshotMsg{err: fmt.Errorf("%s %s: %w", name, id, err)}
		}
		snap, err := m.ds.Snapshot(m.parent)
		if err != nil {
			return snapshotMsg{err: err}
		}
		return snapshotMsg{
			snapshot: snap,
			status:   fmt.Sprintf("%s %s complete", name, id),
		}
	}
}

func (m *Model) loadSnapshot() tea.Cmd {
	if m.ds == nil {
		return nil
	}
	return func() tea.Msg {
		snap, err := m.ds.Snapshot(m.parent)
		return snapshotMsg{snapshot: snap, err: err}
	}
}

func (m *Model) tick() tea.Cmd {
	if m.refresh <= 0 {
		return nil
	}
	return tea.Tick(m.refresh, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m *Model) resize() {
	w := m.width
	h := m.height - 5 - footerLines
	if h < 6 {
		h = 6
	}
	if w < 40 {
		w = 40
	}
	m.search.SetWidth(w - 4)
	m.prompt.SetWidth(w - 4)
	m.help.SetWidth(w)
	if w >= narrowWidth {
		m.detail.SetWidth(w/2 - 4)
		m.list.SetSize(w/2-4, h)
	} else {
		m.detail.SetWidth(w - 4)
		m.list.SetSize(w-2, h)
	}
	m.detail.SetHeight(h)
}

func (m *Model) updateDetailContent() {
	row := m.selectedRow()
	if row == nil {
		m.detail.SetContent("No ticket selected")
		return
	}
	m.detail.SetContent(renderDetail(row))
	m.detail.GotoTop()
}

func (m *Model) selectedRow() *TicketRow {
	if item, ok := m.list.SelectedItem().(*ticketItem); ok {
		row := item.row
		return &row
	}
	rows := m.activeRows()
	if len(rows) == 0 || m.selected < 0 || m.selected >= len(rows) {
		return nil
	}
	return &rows[m.selected]
}

func (m *Model) activeRows() []TicketRow {
	return FilterRows(m.snapshot.Groups[m.ActiveGroup()], m.search.Value())
}

func (m *Model) refreshList(preferredID string) {
	rows := m.activeRows()
	items := make([]list.Item, 0, len(rows))
	selected := 0
	for i := range rows {
		if rows[i].ID == preferredID {
			selected = i
		}
		items = append(items, &ticketItem{row: rows[i]})
	}
	_ = m.list.SetItems(items)
	if len(items) == 0 {
		m.selected = 0
		return
	}
	m.selected = clamp(selected, len(items))
	m.list.Select(m.selected)
}

func (m *Model) syncSelectedFromList() {
	m.selected = clamp(m.list.Index(), len(m.activeRows()))
}

func (m *Model) render() string {
	m.resize()
	if m.helpOn {
		return m.frame(m.helpView())
	}
	if m.prompting != promptNone {
		return m.frame(m.renderList() + "\n\n" + m.prompt.View())
	}
	if m.detailOn && m.width < narrowWidth {
		return m.frame(m.detail.View())
	}
	listView := m.renderList()
	if m.width >= narrowWidth {
		panelHeight := m.height - 3 - footerLines
		if panelHeight < 6 {
			panelHeight = 6
		}
		detailPanelStyle := detailStyle
		if m.detailOn {
			detailPanelStyle = activeDetailStyle
		}
		detail := detailPanelStyle.Width(m.width/2 - 2).Height(panelHeight).Render(m.detail.View())
		listView = listStyle.Width(m.width/2 - 2).Height(panelHeight).Render(listView)
		return m.frame(lipgloss.JoinHorizontal(lipgloss.Top, listView, detail))
	}
	return m.frame(listView)
}

func (m *Model) frame(body string) string {
	parts := []string{m.renderTabs(), body, m.renderStatus()}
	if m.searching {
		parts = append([]string{m.search.View()}, parts...)
	}
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m *Model) renderTabs() string {
	tabs := make([]string, 0, len(Groups))
	for i, group := range Groups {
		label := fmt.Sprintf("%s %d", titleGroup(group), m.snapshot.Counts[group])
		style := tabStyle
		if i == m.groupIdx {
			style = activeTabStyle
		}
		tabs = append(tabs, style.Render(label))
	}
	crumb := ""
	if m.parent != "" {
		crumb = mutedStyle.Render("  in " + m.parent)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, tabs...) + crumb
}

func (m *Model) renderList() string {
	m.list.Title = titleGroup(m.ActiveGroup())
	if len(m.list.Items()) == 0 {
		return emptyStateStyle.Width(m.list.Width()).Render("No tickets in this view")
	}
	return m.list.View()
}

func (m *Model) renderStatus() string {
	helpView := m.help.View(m.keys)
	if m.err != "" {
		return errorStyle.Render(m.err) + "\n" + helpView
	}
	if m.busy {
		status := m.status
		if status == "" {
			status = "working"
		}
		return accentStyle.Render(m.spin.View()) + " " + mutedStyle.Render(status) + "\n" + helpView
	} else if m.status != "" {
		return successStyle.Render(m.status) + "\n" + helpView
	}
	return "\n" + helpView
}

func renderDetail(row *TicketRow) string {
	tk := row.Ticket
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", titleStyle.Render(tk.Title))
	fmt.Fprintf(&b, "%s\n\n", strings.Join([]string{
		idBadgeStyle.Render(displayTicketID(tk.ID)),
		statusBadge(tk.Status),
		typeBadgeStyle.Render(nonEmpty(tk.Type, "task")),
		priorityBadge(tk.Priority),
	}, " "))
	if row.ChildCount > 0 {
		fmt.Fprintf(&b, "%s\n\n", childBadgeStyle.Render(fmt.Sprintf("%d direct children", row.ChildCount)))
	}

	writeList(&b, "Frontmatter", frontmatterLines(&tk))
	writeRuntime(&b, row)
	writeSection(&b, "Description", tk.Description)
	writeSection(&b, "Intent", tk.Intent)
	writeList(&b, "Requirements", tk.RequirementIDs)
	writeList(&b, "Source Refs", tk.SourceRefs)
	writeList(&b, "Constraints", tk.Constraints)
	writeList(&b, "Warnings", tk.Warnings)
	writeList(&b, "Owned Paths", tk.Scope.OwnedPaths)
	writeList(&b, "Read-Only Paths", tk.Scope.ReadOnlyPaths)
	writeList(&b, "Shared Paths", tk.Scope.SharedPaths)
	writeList(&b, "Files Likely Touched", tk.FilesLikelyTouched)
	writeImplementationDetail(&b, tk.ImplementationDetail)
	writeFileChanges(&b, "Implementation Files", tk.ImplementationDetail.Files)
	writeLearningRefs(&b, tk.LearningContext)
	writeList(&b, "Children", row.ChildSummaries)
	writeList(&b, "Acceptance Criteria", tk.AcceptanceCriteria)
	writeList(&b, "Test Cases", tk.TestCases)
	writeList(&b, "Validation Commands", tk.ValidationCommands)
	writeValidationChecks(&b, tk.ValidationChecks)
	writeList(&b, "Required Evidence", tk.RequiredEvidence)
	writeSection(&b, "Reviewer Guidance", tk.ReviewerGuidance)
	writeList(&b, "Notes", tk.Notes)
	return strings.TrimSpace(b.String())
}

func frontmatterLines(tk *ticket.Ticket) []string {
	lines := []string{
		kv("id", tk.ID),
		kv("title", tk.Title),
		kv("type", nonEmpty(tk.Type, "task")),
		kv("status", string(tk.Status)),
		kv("priority", fmt.Sprintf("%d", tk.Priority)),
	}
	appendString := func(name, value string) {
		if strings.TrimSpace(value) != "" {
			lines = append(lines, kv(name, value))
		}
	}
	appendInt := func(name string, value int) {
		if value != 0 {
			lines = append(lines, kv(name, fmt.Sprintf("%d", value)))
		}
	}
	appendListValue := func(name string, values []string) {
		if len(values) > 0 {
			lines = append(lines, kv(name, strings.Join(values, ", ")))
		}
	}

	appendString("parent", tk.Parent)
	appendListValue("deps", tk.Deps)
	appendListValue("tags", tk.Tags)
	appendString("assignee", tk.Assignee)
	appendString("created", tk.Created)
	appendString("updated_at", tk.UpdatedAt)
	appendString("created_from", tk.CreatedFrom)
	appendInt("order", tk.Order)
	appendString("extended_status", tk.ExtendedStatus)
	appendString("status_reason", tk.StatusReason)
	appendString("runtime", tk.RuntimePreference)
	appendString("review_threshold", tk.ReviewThreshold)
	appendString("risk_level", tk.RiskLevel)
	appendString("lineage_id", tk.LineageID)
	appendString("grouping_reason", tk.GroupingReason)
	appendListValue("grouped_requirement_ids", tk.GroupedRequirementIDs)
	appendListValue("links", tk.Links)
	appendString("etag", tk.ETag)

	if len(tk.Extra) > 0 {
		keys := make([]string, 0, len(tk.Extra))
		for key := range tk.Extra {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			lines = append(lines, kv(key, fmt.Sprint(tk.Extra[key])))
		}
	}
	return lines
}

func kv(name, value string) string {
	return fmt.Sprintf("%s: %s", name, value)
}

func writeRuntime(b *strings.Builder, row *TicketRow) {
	if row.RuntimeSummary == "" && (row.Runtime == nil || row.Runtime.Phase == "") {
		return
	}
	fmt.Fprintf(b, "\n%s\n", sectionStyle.Render("Runtime"))
	if row.RuntimeSummary != "" {
		fmt.Fprintf(b, "- %s\n", row.RuntimeSummary)
	}
	if row.Runtime != nil && row.Runtime.Phase != "" {
		fmt.Fprintf(b, "- phase: %s attempt %d\n", row.Runtime.Phase, row.Runtime.Attempt)
	}
}

func writeSection(b *strings.Builder, title, body string) {
	if strings.TrimSpace(body) == "" {
		return
	}
	fmt.Fprintf(b, "\n%s\n%s\n", sectionStyle.Render(title), body)
}

func writeList(b *strings.Builder, title string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "\n%s\n", sectionStyle.Render(title))
	for _, item := range items {
		fmt.Fprintf(b, "- %s\n", item)
	}
}

func writeImplementationDetail(b *strings.Builder, detail ticket.ImplementationDetail) {
	writeSection(b, "Implementation Approach", detail.Approach)
	writeSection(b, "Implementation Notes", detail.Notes)
}

func writeFileChanges(b *strings.Builder, title string, files []ticket.FileChange) {
	if len(files) == 0 {
		return
	}
	fmt.Fprintf(b, "\n%s\n", sectionStyle.Render(title))
	for _, file := range files {
		line := file.Path
		if file.Change != "" {
			line += " - " + file.Change
		}
		if file.Reason != "" {
			line += " (" + file.Reason + ")"
		}
		fmt.Fprintf(b, "- %s\n", line)
	}
}

func writeLearningRefs(b *strings.Builder, refs []ticket.LearningRef) {
	if len(refs) == 0 {
		return
	}
	fmt.Fprintf(b, "\n%s\n", sectionStyle.Render("Learning Context"))
	for _, ref := range refs {
		line := ref.ID
		if ref.Type != "" {
			line += " [" + ref.Type + "]"
		}
		if ref.Title != "" {
			line += " " + ref.Title
		}
		fmt.Fprintf(b, "- %s\n", line)
	}
}

func writeValidationChecks(b *strings.Builder, checks []ticket.ValidationCheck) {
	if len(checks) == 0 {
		return
	}
	fmt.Fprintf(b, "\n%s\n", sectionStyle.Render("Validation Checks"))
	for _, check := range checks {
		line := check.Command
		if check.Expected != "" {
			line += " -> " + check.Expected
		}
		if check.Description != "" {
			line += " (" + check.Description + ")"
		}
		fmt.Fprintf(b, "- %s\n", line)
	}
}

func (m *Model) helpView() string {
	body := strings.Join([]string{
		titleStyle.Render("epos tui"),
		"",
		m.help.FullHelpView(m.keys.FullHelp()),
		"",
		mutedStyle.Render("esc closes help"),
	}, "\n")
	return helpPanelStyle.Width(m.width - 4).Render(body)
}

type ticketItem struct {
	row TicketRow
}

func (i *ticketItem) FilterValue() string {
	row := i.row
	return strings.Join([]string{
		row.ID,
		row.Title,
		string(row.Status),
		row.Type,
		strings.Join(row.Tags, " "),
		row.Assignee,
	}, " ")
}

type ticketDelegate struct{}

func (ticketDelegate) Height() int  { return 3 }
func (ticketDelegate) Spacing() int { return 1 }

func (ticketDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (ticketDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) { //nolint:gocritic // list.ItemDelegate requires this signature.
	row, ok := item.(*ticketItem)
	if !ok {
		return
	}
	tk := row.row
	width := m.Width() - 4
	if width < 28 {
		width = 28
	}
	title := ticketTitleStyle.Width(width).MaxWidth(width).Render(tk.Title)
	meta := renderTicketMeta(&tk, width)
	card := ticketCardStyle.Width(width).Render(title + "\n" + meta)
	if index == m.Index() {
		card = selectedTicketCardStyle.Width(width).Render(title + "\n" + meta)
	}
	fmt.Fprint(w, card)
}

func renderTicketMeta(row *TicketRow, width int) string {
	parts := []string{
		idBadgeStyle.Render(displayTicketID(row.ID)),
		statusBadge(row.Status),
		typeBadgeStyle.Render(nonEmpty(row.Type, "task")),
		priorityBadge(row.Priority),
	}
	if row.ChildCount > 0 {
		parts = append(parts, childBadgeStyle.Render(fmt.Sprintf("%d child", row.ChildCount)))
	}
	if row.DepCount > 0 {
		parts = append(parts, mutedBadgeStyle.Render(fmt.Sprintf("%d deps", row.DepCount)))
	}
	if row.Assignee != "" {
		parts = append(parts, assigneeBadgeStyle.Render(row.Assignee))
	}
	if row.ClaimOwner != "" {
		parts = append(parts, claimBadgeStyle.Render(row.ClaimOwner))
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(strings.Join(parts, " "))
}

func statusBadge(status ticket.Status) string {
	style := statusOpenStyle
	switch status {
	case ticket.StatusClosed, ticket.StatusDone:
		style = statusDoneStyle
	case ticket.StatusFailed:
		style = statusFailedStyle
	case ticket.StatusBlocked, ticket.StatusHeld:
		style = statusBlockedStyle
	case ticket.StatusClaimed, ticket.StatusInProgress, ticket.StatusImplementing, ticket.StatusVerifying, ticket.StatusUnderReview:
		style = statusActiveStyle
	case ticket.StatusPending, ticket.StatusReady, ticket.StatusRepairPending:
		style = statusReadyStyle
	}
	return style.Render(nonEmpty(string(status), "open"))
}

func priorityBadge(priority int) string {
	if priority <= 0 {
		return mutedBadgeStyle.Render("p0")
	}
	return priorityBadgeStyle.Render(fmt.Sprintf("p%d", priority))
}

func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func titleGroup(group Group) string {
	switch group {
	case GroupReady:
		return "Ready"
	case GroupBlocked:
		return "Blocked"
	case GroupClaimed:
		return "Claimed"
	case GroupOpen:
		return "Open"
	case GroupClosed:
		return "Closed"
	case GroupAll:
		return "All"
	default:
		return string(group)
	}
}

func listStyles() list.Styles {
	styles := list.DefaultStyles(true)
	styles.PaginationStyle = mutedStyle
	styles.NoItems = emptyStateStyle
	styles.HelpStyle = mutedStyle
	styles.ActivePaginationDot = accentStyle
	styles.InactivePaginationDot = mutedStyle
	return styles
}

func clamp(i, n int) int {
	if n <= 0 {
		return 0
	}
	if i < 0 {
		return 0
	}
	if i >= n {
		return n - 1
	}
	return i
}

var (
	titleStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	sectionStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	mutedStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	accentStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true)
	successStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	errorStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	tabStyle          = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("8"))
	activeTabStyle    = lipgloss.NewStyle().Padding(0, 1).Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("14"))
	listStyle         = lipgloss.NewStyle().PaddingRight(1)
	detailStyle       = lipgloss.NewStyle().Padding(1, 2).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("8"))
	activeDetailStyle = detailStyle.BorderForeground(lipgloss.Color("14"))
	helpPanelStyle    = lipgloss.NewStyle().Padding(1, 2).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("14"))
	emptyStateStyle   = lipgloss.NewStyle().Padding(2, 3).Foreground(lipgloss.Color("8")).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("8"))
	ticketTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))

	ticketCardStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(lipgloss.Color("8"))
	selectedTicketCardStyle = ticketCardStyle.
				BorderForeground(lipgloss.Color("14")).
				Background(lipgloss.Color("0"))

	badgeBaseStyle     = lipgloss.NewStyle().Padding(0, 1).Bold(true)
	idBadgeStyle       = badgeBaseStyle.Foreground(lipgloss.Color("15")).Background(lipgloss.Color("8"))
	typeBadgeStyle     = badgeBaseStyle.Foreground(lipgloss.Color("15")).Background(lipgloss.Color("5"))
	priorityBadgeStyle = badgeBaseStyle.Foreground(lipgloss.Color("0")).Background(lipgloss.Color("11"))
	mutedBadgeStyle    = badgeBaseStyle.Foreground(lipgloss.Color("15")).Background(lipgloss.Color("8"))
	childBadgeStyle    = badgeBaseStyle.Foreground(lipgloss.Color("0")).Background(lipgloss.Color("12"))
	assigneeBadgeStyle = badgeBaseStyle.Foreground(lipgloss.Color("0")).Background(lipgloss.Color("14"))
	claimBadgeStyle    = badgeBaseStyle.Foreground(lipgloss.Color("0")).Background(lipgloss.Color("13"))
	statusOpenStyle    = badgeBaseStyle.Foreground(lipgloss.Color("15")).Background(lipgloss.Color("4"))
	statusReadyStyle   = badgeBaseStyle.Foreground(lipgloss.Color("0")).Background(lipgloss.Color("10"))
	statusActiveStyle  = badgeBaseStyle.Foreground(lipgloss.Color("0")).Background(lipgloss.Color("14"))
	statusBlockedStyle = badgeBaseStyle.Foreground(lipgloss.Color("0")).Background(lipgloss.Color("11"))
	statusDoneStyle    = badgeBaseStyle.Foreground(lipgloss.Color("0")).Background(lipgloss.Color("10"))
	statusFailedStyle  = badgeBaseStyle.Foreground(lipgloss.Color("15")).Background(lipgloss.Color("9"))
)

var _ tea.Model = (*Model)(nil)
