// Package tui é a interface interativa: a tela que aparece quando se digita
// `mac-cleaner` sem argumentos.
//
// Segue a arquitetura Elm do Bubble Tea, com cada peça em seu arquivo: o estado
// aqui, as transições em update.go, o desenho em view.go, as teclas em keys.go e
// as cores em styles.go.
package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/danfigueroa/mac-cleaner/internal/domain"
)

// phase é o passo do fluxo em que a tela está.
type phase int

const (
	phaseScanning phase = iota
	phaseChoosing
	phaseCleaning
	phaseDone
	phaseFailed
)

// row é uma linha da lista. Cabeçalhos de categoria e alvos convivem na mesma
// fatia para que a navegação com as setas seja uma simples soma de índice; o
// campo result nulo distingue um do outro.
type row struct {
	category domain.Category
	result   *domain.Result
	total    domain.Bytes
}

func (r row) isHeader() bool { return r.result == nil }

// Scanner é o que a TUI precisa para medir o disco.
type Scanner interface {
	Scan(ctx context.Context, env domain.Env, rules []domain.Rule) (domain.Report, error)
}

// Cleaner é o que a TUI precisa para executar o plano.
type Cleaner interface {
	Execute(ctx context.Context, plan domain.Plan) (domain.Summary, error)
}

// Trasher é o que a TUI precisa para esvaziar a Lixeira.
type Trasher interface {
	Empty(ctx context.Context) error
}

// Model é o estado da tela.
type Model struct {
	// ctx guardado em struct é normalmente um cheiro, e aqui é deliberado: a
	// arquitetura do Bubble Tea não passa contexto para Update nem para os
	// comandos, então este é o único lugar de onde as operações canceláveis
	// (varredura, limpeza) conseguem alcançá-lo. Sem isso, Ctrl-C fecharia a
	// tela e deixaria um `docker image prune` rodando órfão.
	ctx context.Context //nolint:containedctx // o Bubble Tea não propaga contexto

	scanner Scanner
	cleaner Cleaner
	trasher Trasher
	env     domain.Env
	rules   []domain.Rule
	minSize domain.Bytes

	phase  phase
	frame  int
	report domain.Report
	rows   []row

	cursor int
	// chosen é indexado por ID de regra, e não por posição, para sobreviver a
	// qualquer reordenação da lista.
	chosen map[string]bool

	summary      domain.Summary
	trashEmptied bool
	err          error

	width  int
	height int
}

// Config reúne as dependências da tela.
type Config struct {
	Ctx     context.Context //nolint:containedctx // repassado ao Model
	Scanner Scanner
	Cleaner Cleaner
	Trasher Trasher
	Env     domain.Env
	Rules   []domain.Rule
	MinSize domain.Bytes
}

// New monta o modelo inicial.
func New(cfg Config) Model {
	return Model{
		ctx:     cfg.Ctx,
		scanner: cfg.Scanner,
		cleaner: cfg.Cleaner,
		trasher: cfg.Trasher,
		env:     cfg.Env,
		rules:   cfg.Rules,
		minSize: cfg.MinSize,
		phase:   phaseScanning,
		chosen:  make(map[string]bool),
	}
}

// Init dispara a varredura assim que a tela abre.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.scan(), tick())
}

// Err devolve o erro que encerrou a tela, se houve algum.
//
// O Bubble Tea engole a distinção entre "saiu porque terminou" e "saiu porque
// falhou"; sem isto, a CLI sempre encerraria com código zero.
func (m Model) Err() error { return m.err }

// selectedResults devolve os alvos marcados, na ordem em que aparecem.
func (m Model) selectedResults() []domain.Result {
	var selected []domain.Result
	for _, r := range m.rows {
		if r.isHeader() || !m.chosen[r.result.Rule.ID] {
			continue
		}
		selected = append(selected, *r.result)
	}
	return selected
}

// selectedTotal soma o que está marcado agora.
func (m Model) selectedTotal() domain.Bytes {
	var total domain.Bytes
	for _, result := range m.selectedResults() {
		total += result.Finding.Size
	}
	return total
}

// buildRows monta a lista a partir do relatório, agrupada por categoria.
//
// A pré-seleção acontece aqui, e apenas para alvos de risco seguro. Marcar tudo
// por padrão transformaria o Enter em reflexo, e o dia em que isso apagasse algo
// importante seria o dia em que a ferramenta perderia a confiança do usuário.
func (m *Model) buildRows() {
	m.rows = nil
	for _, group := range m.report.GroupByCategory() {
		m.rows = append(m.rows, row{category: group.Category, total: group.Total})

		for _, result := range group.Results {
			m.rows = append(m.rows, row{result: &result})

			if result.Rule.Risk.PreSelected() && result.Rule.Removable() {
				m.chosen[result.Rule.ID] = true
			}
		}
	}
	m.cursor = m.firstToggleable()
}

// firstToggleable escolhe onde o cursor começa.
//
// Prefere uma linha que o usuário consiga marcar. Abrir com o cursor sobre um
// alvo que exige root faria a primeira tecla de espaço não produzir efeito
// nenhum, sem explicação — e a conclusão natural seria que a tela travou.
func (m Model) firstToggleable() int {
	primeiraLinha := -1

	for i, r := range m.rows {
		if r.isHeader() {
			continue
		}
		if primeiraLinha < 0 {
			primeiraLinha = i
		}
		if r.result.Rule.Removable() {
			return i
		}
	}

	if primeiraLinha < 0 {
		return 0
	}
	return primeiraLinha
}
