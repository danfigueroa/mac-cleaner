package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/danfigueroa/mac-cleaner/internal/domain"
	"github.com/danfigueroa/mac-cleaner/internal/service"
)

// Mensagens que chegam das operações assíncronas.
type (
	scanDoneMsg  struct{ report domain.Report }
	cleanDoneMsg struct{ summary domain.Summary }
	trashDoneMsg struct{}
	failureMsg   struct{ err error }
	tickMsg      struct{}
)

// spinnerInterval é o ritmo da animação enquanto o disco é medido.
const spinnerInterval = 120 * time.Millisecond

func tick() tea.Cmd {
	return tea.Tick(spinnerInterval, func(time.Time) tea.Msg { return tickMsg{} })
}

// scan mede o disco fora da goroutine da interface.
func (m Model) scan() tea.Cmd {
	return func() tea.Msg {
		report, err := m.scanner.Scan(m.ctx, m.env, m.rules)
		if err != nil {
			return failureMsg{err: err}
		}
		report.FilterBySize(m.minSize)
		return scanDoneMsg{report: report}
	}
}

// clean executa o plano aprovado.
func (m Model) clean() tea.Cmd {
	plan := service.BuildPlan(m.selectedResults())

	return func() tea.Msg {
		summary, err := m.cleaner.Execute(m.ctx, plan)
		if err != nil {
			return failureMsg{err: err}
		}
		return cleanDoneMsg{summary: summary}
	}
}

func (m Model) emptyTrash() tea.Cmd {
	return func() tea.Msg {
		if err := m.trasher.Empty(m.ctx); err != nil {
			return failureMsg{err: err}
		}
		return trashDoneMsg{}
	}
}

// Update é a única função que altera o estado da tela.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tickMsg:
		m.frame++
		if m.phase == phaseScanning || m.phase == phaseCleaning {
			return m, tick()
		}
		return m, nil

	case scanDoneMsg:
		m.report = msg.report
		m.buildRows()
		m.phase = phaseChoosing
		if len(m.rows) == 0 {
			m.phase = phaseDone
			return m, tea.Quit
		}
		return m, nil

	case cleanDoneMsg:
		m.summary = msg.summary
		m.phase = phaseDone
		return m, nil

	case trashDoneMsg:
		m.trashEmptied = true
		return m, tea.Quit

	case failureMsg:
		m.err = msg.err
		m.phase = phaseFailed
		return m, tea.Quit

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Sair sempre funciona, em qualquer fase. Numa ferramenta que apaga
	// arquivos, uma tela da qual não se consegue escapar é inaceitável.
	if key.matches(msg, keyQuit) {
		return m, tea.Quit
	}

	switch m.phase {
	case phaseChoosing:
		return m.handleChoosingKey(msg)
	case phaseDone:
		return m.handleDoneKey(msg)
	case phaseScanning, phaseCleaning, phaseFailed:
		return m, nil
	}
	return m, nil
}

func (m Model) handleChoosingKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.matches(msg, keyUp):
		m.cursor = m.move(-1)

	case key.matches(msg, keyDown):
		m.cursor = m.move(1)

	case key.matches(msg, keyToggle):
		if atual := m.rows[m.cursor]; !atual.isHeader() && atual.result.Rule.Removable() {
			m.chosen[atual.result.Rule.ID] = !m.chosen[atual.result.Rule.ID]
		}

	case key.matches(msg, keySelectAll):
		m.setAll(true)

	case key.matches(msg, keySelectNone):
		m.setAll(false)

	case key.matches(msg, keyConfirm):
		if m.selectedTotal() == 0 {
			return m, nil
		}
		m.phase = phaseCleaning
		return m, tea.Batch(m.clean(), tick())
	}

	return m, nil
}

func (m Model) handleDoneKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// A pergunta sobre esvaziar a Lixeira só existe se algo foi para lá.
	if m.summary.Trashed() == 0 || m.trashEmptied {
		return m, tea.Quit
	}

	if key.matches(msg, keyYes) {
		return m, m.emptyTrash()
	}
	return m, tea.Quit
}

// move desloca o cursor pulando os cabeçalhos de categoria.
func (m Model) move(direction int) int {
	for i := m.cursor + direction; i >= 0 && i < len(m.rows); i += direction {
		if !m.rows[i].isHeader() {
			return i
		}
	}
	return m.cursor
}

// setAll marca ou desmarca tudo que é selecionável.
func (m *Model) setAll(value bool) {
	for _, r := range m.rows {
		if r.isHeader() || !r.result.Rule.Removable() {
			continue
		}
		m.chosen[r.result.Rule.ID] = value
	}
}
