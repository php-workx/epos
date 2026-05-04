package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/php-workx/epos/ticket"

	tea "charm.land/bubbletea/v2"
)

func TestModelKeyHandlingAndSelectionPreservation(t *testing.T) {
	ds := &fakeDataSource{snap: Snapshot{
		Groups: map[Group][]TicketRow{
			GroupReady: {
				{ID: "epo-one", Title: "one"},
				{ID: "epo-two", Title: "two"},
			},
			GroupBlocked: {
				{ID: "epo-blocked", Title: "blocked"},
			},
		},
		Counts: map[Group]int{GroupReady: 2, GroupBlocked: 1, GroupAll: 3},
	}}
	m := NewModel(Config{DataSource: ds, Refresh: 0})
	m = m.WithSnapshot(ds.snap)

	m = updateModel(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	if got := m.SelectedID(); got != "epo-two" {
		t.Fatalf("SelectedID after down = %q, want epo-two", got)
	}

	m = updateModel(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	if m.ActiveGroup() != GroupBlocked {
		t.Fatalf("ActiveGroup = %s, want blocked", m.ActiveGroup())
	}
	if got := m.SelectedID(); got != "epo-blocked" {
		t.Fatalf("SelectedID after tab = %q, want epo-blocked", got)
	}

	m = updateModel(t, m, tea.KeyPressMsg{Code: '/', Text: "/"})
	if !m.SearchFocused() {
		t.Fatal("search should be focused")
	}
	m = updateModel(t, m, tea.KeyPressMsg{Code: 'b', Text: "b"})
	if m.SearchValue() != "b" {
		t.Fatalf("SearchValue = %q, want b", m.SearchValue())
	}
	m = updateModel(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.SearchFocused() || m.SearchValue() != "" {
		t.Fatalf("search focused=%v value=%q, want cleared", m.SearchFocused(), m.SearchValue())
	}

	m = updateModel(t, m, tea.KeyPressMsg{Code: '?'})
	if !m.HelpVisible() {
		t.Fatal("help should be visible")
	}
	m = updateModel(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.HelpVisible() {
		t.Fatal("help should close on escape")
	}

	m = updateModel(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.DetailFocused() {
		t.Fatal("detail should be focused after enter")
	}
	m = updateModel(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.DetailFocused() {
		t.Fatal("detail should close on escape")
	}

	next := ds.snap
	next.Groups[GroupBlocked] = []TicketRow{
		{ID: "epo-new", Title: "new"},
		{ID: "epo-blocked", Title: "blocked updated"},
	}
	m = m.WithSnapshot(next)
	if got := m.SelectedID(); got != "epo-blocked" {
		t.Fatalf("SelectedID after refresh = %q, want preserved epo-blocked", got)
	}
}

func TestModelEmptyAndErrorViewsDoNotCrash(t *testing.T) {
	m := NewModel(Config{DataSource: &fakeDataSource{}, Refresh: 0}).WithSnapshot(Snapshot{})
	if got := m.View().Content; got == "" {
		t.Fatal("empty view should render content")
	}

	previous := Snapshot{
		Groups: map[Group][]TicketRow{GroupReady: {{ID: "epo-one", Title: "one"}}},
		Counts: map[Group]int{GroupReady: 1},
	}
	m = m.WithSnapshot(previous)
	m = updateModel(t, m, snapshotMsg{err: errors.New("load failed")})
	if m.ErrorMessage() == "" {
		t.Fatal("expected error message")
	}
	if got := m.SelectedID(); got != "epo-one" {
		t.Fatalf("previous snapshot not preserved; selected = %q", got)
	}
	if got := m.View().Content; got == "" {
		t.Fatal("error view should render content")
	}
}

func TestMutationKeysCallDataSource(t *testing.T) {
	tests := []struct {
		name   string
		key    tea.KeyPressMsg
		owner  string
		wantOp string
	}{
		{name: "claim", key: tea.KeyPressMsg{Code: 'c'}, owner: "alice", wantOp: "claim:epo-one:alice"},
		{name: "release", key: tea.KeyPressMsg{Code: 'u'}, owner: "alice", wantOp: "release:epo-one:alice"},
		{name: "close", key: tea.KeyPressMsg{Code: 'x'}, wantOp: "close:epo-one:closed from tui"},
		{name: "reopen", key: tea.KeyPressMsg{Code: 'o'}, wantOp: "reopen:epo-one:reopened from tui"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds := &fakeDataSource{snap: singleTicketSnapshot()}
			m := NewModel(Config{DataSource: ds, Owner: tt.owner, Refresh: 0}).WithSnapshot(ds.snap)

			cmd := updateModelCmd(t, m, tt.key)
			if cmd == nil {
				t.Fatal("expected mutation command")
			}
			updateModel(t, m, cmd())

			if got := strings.Join(ds.ops, ","); got != tt.wantOp {
				t.Fatalf("datasource ops = %q, want %q", got, tt.wantOp)
			}
		})
	}
}

func TestNotePromptAddsNote(t *testing.T) {
	ds := &fakeDataSource{snap: singleTicketSnapshot()}
	m := NewModel(Config{DataSource: ds, Refresh: 0}).WithSnapshot(ds.snap)

	m = updateModel(t, m, tea.KeyPressMsg{Code: 'n'})
	m.prompt.SetValue("needs validation")

	cmd := updateModelCmd(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected note command")
	}
	updateModel(t, m, cmd())

	if got, want := strings.Join(ds.ops, ","), "note:epo-one:needs validation"; got != want {
		t.Fatalf("datasource ops = %q, want %q", got, want)
	}
}

func TestOwnerRequiredErrorsAreShown(t *testing.T) {
	tests := []struct {
		name string
		key  tea.KeyPressMsg
		want string
	}{
		{name: "claim", key: tea.KeyPressMsg{Code: 'c'}, want: "owner is required to claim"},
		{name: "release", key: tea.KeyPressMsg{Code: 'u'}, want: "owner is required to release"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds := &fakeDataSource{snap: singleTicketSnapshot()}
			m := NewModel(Config{DataSource: ds, Refresh: 0}).WithSnapshot(ds.snap)

			cmd := updateModelCmd(t, m, tt.key)
			if cmd == nil {
				t.Fatal("expected mutation command")
			}
			m = updateModel(t, m, cmd())

			if !strings.Contains(m.ErrorMessage(), tt.want) {
				t.Fatalf("ErrorMessage = %q, want containing %q", m.ErrorMessage(), tt.want)
			}
			if len(ds.ops) != 0 {
				t.Fatalf("datasource ops = %v, want none", ds.ops)
			}
		})
	}
}

func TestMutationSuccessSetsStatusMessage(t *testing.T) {
	ds := &fakeDataSource{snap: singleTicketSnapshot()}
	m := NewModel(Config{DataSource: ds, Owner: "alice", Refresh: 0}).WithSnapshot(ds.snap)

	cmd := updateModelCmd(t, m, tea.KeyPressMsg{Code: 'c'})
	if cmd == nil {
		t.Fatal("expected mutation command")
	}
	m = updateModel(t, m, cmd())

	if m.status == "" {
		t.Fatal("expected success status message")
	}
	if !strings.Contains(m.status, "claim") || !strings.Contains(m.status, "epo-one") {
		t.Fatalf("status = %q, want mutation name and ticket id", m.status)
	}
}

func TestEnterOpensDetailAndRightDrillsIntoChildren(t *testing.T) {
	root := singleTicketSnapshot()
	root.Groups[GroupReady][0].ChildCount = 1
	root.Rows[0].ChildCount = 1
	ds := &fakeDataSource{snap: root}
	m := NewModel(Config{DataSource: ds, Refresh: 0}).WithSnapshot(root)

	cmd := updateModelCmd(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("enter should not load children")
	}
	if !m.DetailFocused() {
		t.Fatal("enter should focus detail")
	}
	m.detailOn = false

	cmd = updateModelCmd(t, m, tea.KeyPressMsg{Code: tea.KeyRight})
	if cmd == nil {
		t.Fatal("expected drill-down load command")
	}
	updateModel(t, m, cmd())
	if m.parent != "epo-one" {
		t.Fatalf("parent = %q, want epo-one", m.parent)
	}

	cmd = updateModelCmd(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected parent load command")
	}
	updateModel(t, m, cmd())
	if m.parent != "" {
		t.Fatalf("parent after escape = %q, want root", m.parent)
	}
}

func TestDetailFocusScrollsInsteadOfMovingList(t *testing.T) {
	row := TicketRow{
		ID:    "epo-one",
		Title: "one",
		Ticket: ticket.Ticket{
			ID:     "epo-one",
			Title:  "one",
			Type:   "task",
			Status: ticket.StatusOpen,
			Notes: []string{
				"line 1",
				"line 2",
				"line 3",
				"line 4",
				"line 5",
				"line 6",
			},
		},
	}
	snap := Snapshot{
		Groups: map[Group][]TicketRow{
			GroupReady: {
				row,
				{ID: "epo-two", Title: "two"},
			},
		},
		Counts: map[Group]int{GroupReady: 2},
		Rows:   []TicketRow{row},
	}
	m := NewModel(Config{DataSource: &fakeDataSource{snap: snap}, Refresh: 0}).WithSnapshot(snap)
	m.detail.SetHeight(4)

	m = updateModel(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.DetailFocused() {
		t.Fatal("detail should be focused")
	}
	if !strings.Contains(m.status, "detail focused") {
		t.Fatalf("status = %q, want detail focus hint", m.status)
	}

	m = updateModel(t, m, tea.KeyPressMsg{Code: 'j', Text: "j"})
	if got := m.SelectedID(); got != "epo-one" {
		t.Fatalf("SelectedID = %q, want list selection unchanged", got)
	}
	if got := m.detail.YOffset(); got == 0 {
		t.Fatal("detail viewport should scroll on j")
	}

	m = updateModel(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.DetailFocused() {
		t.Fatal("escape should return focus to list")
	}
	if m.status == detailFocusStatus {
		t.Fatal("detail focus status should clear on escape")
	}
}

func TestRenderStatusKeepsActivitySeparateFromHelpLine(t *testing.T) {
	m := NewModel(Config{DataSource: &fakeDataSource{}, Refresh: 0})
	m.status = "reopen epo-fabrikk-integration-replace-inte-qq44 complete"

	lines := strings.Split(m.renderStatus(), "\n")
	if len(lines) != 2 {
		t.Fatalf("renderStatus lines = %d, want 2: %q", len(lines), m.renderStatus())
	}
	if !strings.Contains(lines[0], "reopen epo-fabrikk-integration-replace-inte-qq44 complete") {
		t.Fatalf("activity line = %q, want status text", lines[0])
	}
	if strings.Contains(lines[1], "reopen epo-fabrikk-integration") {
		t.Fatalf("help line includes activity text: %q", lines[1])
	}
	if !strings.Contains(lines[1], "k/up") {
		t.Fatalf("help line = %q, want fixed help text", lines[1])
	}
}

func TestRenderDetailIncludesFrontmatterBodyAndChildren(t *testing.T) {
	row := TicketRow{
		ID:             "epo-parent-ticket-with-long-title-abcd",
		ChildCount:     1,
		ChildSummaries: []string{"#wxyz  open  p2  child task"},
		Ticket: ticket.Ticket{
			ID:                 "epo-parent-ticket-with-long-title-abcd",
			Title:              "Parent epic",
			Type:               "epic",
			Status:             ticket.StatusOpen,
			Priority:           2,
			Parent:             "epo-root",
			Deps:               []string{"epo-dep"},
			Tags:               []string{"ui"},
			Assignee:           "riley",
			Description:        "Full markdown body description.",
			AcceptanceCriteria: []string{"Details are visible"},
			ValidationCommands: []string{"go test ./..."},
		},
	}

	view := renderDetail(&row)
	for _, want := range []string{
		"#abcd",
		"Frontmatter",
		"id: epo-parent-ticket-with-long-title-abcd",
		"parent: epo-root",
		"Description",
		"Full markdown body description.",
		"Children",
		"#wxyz",
		"Acceptance Criteria",
		"Details are visible",
		"Validation Commands",
		"go test ./...",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("renderDetail missing %q:\n%s", want, view)
		}
	}
}

func TestDisplayTicketIDUsesSuffixForLongIDs(t *testing.T) {
	if got, want := displayTicketID("epo-agent-skill-vakt-read-only-integ-ur1i"), "#ur1i"; got != want {
		t.Fatalf("displayTicketID long = %q, want %q", got, want)
	}
	if got, want := displayTicketID("epo-one"), "epo-one"; got != want {
		t.Fatalf("displayTicketID short = %q, want %q", got, want)
	}
}

func updateModel(t *testing.T, m *Model, msg tea.Msg) *Model {
	t.Helper()
	next, _ := m.Update(msg)
	cast, ok := next.(*Model)
	if !ok {
		t.Fatalf("Update returned %T, want *tui.Model", next)
	}
	return cast
}

func updateModelCmd(t *testing.T, m *Model, msg tea.Msg) tea.Cmd {
	t.Helper()
	next, cmd := m.Update(msg)
	cast, ok := next.(*Model)
	if !ok {
		t.Fatalf("Update returned %T, want *tui.Model", next)
	}
	*m = *cast
	return cmd
}

func singleTicketSnapshot() Snapshot {
	row := TicketRow{
		ID:    "epo-one",
		Title: "one",
		Ticket: ticket.Ticket{
			ID:     "epo-one",
			Title:  "one",
			Type:   "task",
			Status: ticket.StatusOpen,
		},
	}
	return Snapshot{
		Groups: map[Group][]TicketRow{
			GroupReady: {row},
		},
		Counts: map[Group]int{GroupReady: 1, GroupAll: 1},
		Rows:   []TicketRow{row},
	}
}

type fakeDataSource struct {
	snap Snapshot
	err  error
	ops  []string
}

func (f *fakeDataSource) Snapshot(string) (Snapshot, error) {
	return f.snap, f.err
}

func (f *fakeDataSource) AddNote(id, text string) error {
	f.ops = append(f.ops, "note:"+id+":"+text)
	return nil
}

func (f *fakeDataSource) Claim(id, owner string) error {
	f.ops = append(f.ops, "claim:"+id+":"+owner)
	return nil
}

func (f *fakeDataSource) Release(id, owner string) error {
	f.ops = append(f.ops, "release:"+id+":"+owner)
	return nil
}

func (f *fakeDataSource) Close(id, reason string) error {
	f.ops = append(f.ops, "close:"+id+":"+reason)
	return nil
}

func (f *fakeDataSource) Reopen(id, reason string) error {
	f.ops = append(f.ops, "reopen:"+id+":"+reason)
	return nil
}

var _ DataSource = (*fakeDataSource)(nil)
