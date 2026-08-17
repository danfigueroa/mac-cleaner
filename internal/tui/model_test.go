package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/danfigueroa/mac-cleaner/internal/domain"
)

// O teste vive no pacote tui, e não em tui_test, porque o que importa aqui é a
// máquina de estados — quais alvos ficam marcados, o que o Enter monta — e não a
// superfície pública, que é só New/Init/Update/View.

type cleanerSpy struct {
	recebido domain.Plan
	summary  domain.Summary
}

func (c *cleanerSpy) Execute(_ context.Context, plan domain.Plan) (domain.Summary, error) {
	c.recebido = plan
	return c.summary, nil
}

type trasherFake struct{}

func (trasherFake) Empty(context.Context) error { return nil }

func regra(id string, risk domain.Risk, size domain.Bytes, strategy domain.Strategy) domain.Result {
	return domain.Result{
		Rule: domain.Rule{
			ID:       id,
			Name:     id,
			Category: domain.CategoryDev,
			Risk:     risk,
			Strategy: strategy,
		},
		Finding: domain.Finding{
			RuleID:  id,
			Size:    size,
			Targets: []domain.Target{{Path: "/home/ana/Library/Caches/" + id, Size: size}},
		},
	}
}

func relatorioDeTeste() domain.Report {
	return domain.Report{
		Volume: domain.Volume{Path: "/", Total: 100 * domain.Gigabyte, Free: 10 * domain.Gigabyte, Used: 90 * domain.Gigabyte},
		Results: []domain.Result{
			regra("cache-seguro", domain.RiskSafe, 5*domain.Gigabyte, domain.TrashTargets{}),
			regra("cache-regeneravel", domain.RiskRegenerable, 3*domain.Gigabyte, domain.TrashTargets{}),
			regra("dados-sensiveis", domain.RiskReview, 7*domain.Gigabyte, domain.TrashTargets{}),
			regra("precisa-root", domain.RiskSafe, 9*domain.Gigabyte,
				domain.ManualOnly{Command: "sudo rm -rf /Library/Caches/*"}),
		},
	}
}

func telaComRelatorio(t *testing.T, cleaner Cleaner) Model {
	t.Helper()

	m := New(Config{Ctx: t.Context(), Cleaner: cleaner, Trasher: trasherFake{}})
	return aplicar(t, m, scanDoneMsg{report: relatorioDeTeste()})
}

func aplicar(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()

	next, _ := m.Update(msg)
	updated, ok := next.(Model)
	if !ok {
		t.Fatalf("Update devolveu %T, quer tui.Model", next)
	}
	return updated
}

// executar roda um tea.Cmd como o runtime do Bubble Tea faria.
//
// tea.Batch não executa nada: devolve uma BatchMsg com os comandos dentro, que o
// runtime então dispara. Chamar o batch e parar por aí, como é tentador, faria o
// teste passar sem nunca exercitar a limpeza.
func executar(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	if batch, ok := cmd().(tea.BatchMsg); ok {
		for _, sub := range batch {
			executar(sub)
		}
	}
}

func tecla(t *testing.T, m Model, key string) Model {
	t.Helper()

	if key == " " {
		return aplicar(t, m, tea.KeyMsg{Type: tea.KeySpace})
	}
	if key == "enter" {
		return aplicar(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	}
	return aplicar(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
}

// TestPreSelecionaApenasOSeguro é a regra que substitui o julgamento do LLM: o
// que vem marcado por padrão é só o que não custa nada perder. Marcar tudo
// transformaria o Enter em reflexo, e o dia em que isso apagasse algo importante
// seria o dia em que a ferramenta perderia a confiança do usuário.
func TestPreSelecionaApenasOSeguro(t *testing.T) {
	t.Parallel()

	m := telaComRelatorio(t, &cleanerSpy{})

	quer := map[string]bool{
		"cache-seguro":      true,
		"cache-regeneravel": false,
		"dados-sensiveis":   false,
		"precisa-root":      false, // seguro, mas a CLI nunca escala privilégio
	}
	for id, esperado := range quer {
		if m.chosen[id] != esperado {
			t.Errorf("regra %q marcada = %v, quer %v", id, m.chosen[id], esperado)
		}
	}

	if m.selectedTotal() != 5*domain.Gigabyte {
		t.Errorf("total selecionado = %s, quer 5.0 GB", m.selectedTotal())
	}
}

// TestCursorComecaNumaLinhaMarcavel cobre o detalhe que faria a tela parecer
// travada: o maior alvo da lista pode ser um que exige root, e abrir com o
// cursor sobre ele faria a primeira tecla de espaço não fazer nada.
func TestCursorComecaNumaLinhaMarcavel(t *testing.T) {
	t.Parallel()

	m := telaComRelatorio(t, &cleanerSpy{})

	linha := m.rows[m.cursor]
	if linha.isHeader() {
		t.Fatal("o cursor começou sobre um cabeçalho de categoria")
	}
	if linha.result.Rule.NeedsRoot() {
		t.Errorf("o cursor começou em %q, que exige root e não pode ser marcado",
			linha.result.Rule.ID)
	}
}

func TestEspacoAlternaALinhaSobOCursor(t *testing.T) {
	t.Parallel()

	m := telaComRelatorio(t, &cleanerSpy{})

	sobOCursor := m.rows[m.cursor].result.Rule.ID
	antes := m.chosen[sobOCursor]

	m = tecla(t, m, " ")
	if m.chosen[sobOCursor] == antes {
		t.Errorf("espaço não alternou a regra %q", sobOCursor)
	}
}

// TestEspacoNaoMarcaAlvoQueExigeRoot leva o cursor até a linha de root e confirma
// que ela permanece intocável.
func TestEspacoNaoMarcaAlvoQueExigeRoot(t *testing.T) {
	t.Parallel()

	m := telaComRelatorio(t, &cleanerSpy{})

	// Posicionamos o cursor direto na linha de root em vez de navegar até ela:
	// depender da ordenação faria o teste virar um t.Skip silencioso no dia em
	// que os tamanhos do relatório de teste mudassem.
	indice := -1
	for i, r := range m.rows {
		if !r.isHeader() && r.result.Rule.NeedsRoot() {
			indice = i
			break
		}
	}
	if indice < 0 {
		t.Fatal("o relatório de teste precisa conter um alvo que exija root")
	}
	m.cursor = indice

	m = tecla(t, m, " ")
	if m.chosen[m.rows[indice].result.Rule.ID] {
		t.Error("espaço marcou um alvo que exige root")
	}
}

// TestMarcarTudoIgnoraOQueExigeRoot garante que o atalho de conveniência não
// contorna a regra de nunca escalar privilégio.
func TestMarcarTudoIgnoraOQueExigeRoot(t *testing.T) {
	t.Parallel()

	m := tecla(t, telaComRelatorio(t, &cleanerSpy{}), "a")

	if !m.chosen["dados-sensiveis"] || !m.chosen["cache-regeneravel"] {
		t.Error("'a' deveria marcar todos os alvos removíveis")
	}
	if m.chosen["precisa-root"] {
		t.Error("'a' marcou um alvo que exige root")
	}
}

func TestDesmarcarTudo(t *testing.T) {
	t.Parallel()

	m := tecla(t, tecla(t, telaComRelatorio(t, &cleanerSpy{}), "a"), "n")

	if m.selectedTotal() != 0 {
		t.Errorf("total = %s, quer 0 depois de 'n'", m.selectedTotal())
	}
}

// TestEnterMontaOPlanoComOQueEstaMarcado fecha o ciclo: o que a tela mostra como
// selecionado é exatamente o que chega ao executor.
func TestEnterMontaOPlanoComOQueEstaMarcado(t *testing.T) {
	t.Parallel()

	spy := &cleanerSpy{}
	m := telaComRelatorio(t, spy)

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter não disparou a limpeza")
	}
	if updated, ok := next.(Model); !ok || updated.phase != phaseCleaning {
		t.Errorf("fase = %v, quer phaseCleaning", updated.phase)
	}

	executar(cmd)

	if len(spy.recebido.Items) != 1 {
		t.Fatalf("plano tem %d itens, quer 1", len(spy.recebido.Items))
	}
	if id := spy.recebido.Items[0].Rule.ID; id != "cache-seguro" {
		t.Errorf("plano contém %q, quer cache-seguro", id)
	}
}

func TestEnterSemSelecaoNaoFazNada(t *testing.T) {
	t.Parallel()

	m := tecla(t, telaComRelatorio(t, &cleanerSpy{}), "n")

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("Enter sem nada marcado disparou a limpeza")
	}
	if updated, ok := next.(Model); !ok || updated.phase != phaseChoosing {
		t.Error("a tela saiu da fase de escolha sem nada selecionado")
	}
}

// TestAvisaQueALixeiraNaoLiberaEspaco cobre o aviso mais importante da tela.
// Sem ele o usuário roda `df`, vê o mesmo número e conclui que nada aconteceu.
func TestAvisaQueALixeiraNaoLiberaEspaco(t *testing.T) {
	t.Parallel()

	m := telaComRelatorio(t, &cleanerSpy{})
	m = aplicar(t, m, cleanDoneMsg{summary: domain.Summary{
		Outcomes: []domain.Outcome{{
			Rule:      domain.Rule{Name: "cache-seguro"},
			Reclaimed: 5 * domain.Gigabyte,
			Trashed:   true,
		}},
	}})

	view := m.View()
	if !strings.Contains(view, "não libera disco") {
		t.Errorf("a tela final não avisa que a Lixeira ainda ocupa espaço:\n%s", view)
	}
	if !strings.Contains(view, "esvaziar a Lixeira agora") {
		t.Errorf("a tela final não oferece esvaziar a Lixeira:\n%s", view)
	}
}

func TestViewDaEscolhaMostraOsDados(t *testing.T) {
	t.Parallel()

	view := telaComRelatorio(t, &cleanerSpy{}).View()

	for _, esperado := range []string{"mac-cleaner", "dados-sensiveis", "revisar", "requer sudo"} {
		if !strings.Contains(view, esperado) {
			t.Errorf("a tela não contém %q:\n%s", esperado, view)
		}
	}
}
