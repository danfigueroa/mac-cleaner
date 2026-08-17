package tui

import (
	"slices"

	tea "github.com/charmbracelet/bubbletea"
)

// binding é um atalho: as teclas que o acionam e como ele aparece na ajuda.
type binding struct {
	keys []string
	help string
}

// Atalhos da tela. As setas convivem com hjkl porque quem vive no terminal
// espera as duas coisas, e nenhuma delas custa nada.
var (
	keyUp         = binding{keys: []string{"up", "k"}, help: "↑/k mover"}
	keyDown       = binding{keys: []string{"down", "j"}, help: "↓/j mover"}
	keyToggle     = binding{keys: []string{" "}, help: "espaço marcar"}
	keySelectAll  = binding{keys: []string{"a"}, help: "a marcar tudo"}
	keySelectNone = binding{keys: []string{"n"}, help: "n desmarcar tudo"}
	keyConfirm    = binding{keys: []string{"enter"}, help: "enter limpar"}
	keyYes        = binding{keys: []string{"s", "y"}, help: "s sim"}
	keyQuit       = binding{keys: []string{"q", "esc", "ctrl+c"}, help: "q sair"}
)

// key agrupa a verificação de atalhos. É uma struct vazia com método em vez de
// uma função solta só para que o ponto de chamada leia como `key.matches(...)`.
var key keyMatcher

type keyMatcher struct{}

func (keyMatcher) matches(msg tea.KeyMsg, b binding) bool {
	return slices.Contains(b.keys, msg.String())
}
