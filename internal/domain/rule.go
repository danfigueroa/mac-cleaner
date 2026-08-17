package domain

import "context"

// Rule descreve um alvo de limpeza: o que ele é, quanto custa remover e como
// removê-lo.
//
// Os campos What/Lose/Regen são obrigatórios e existem por um motivo específico:
// eles são o que separa esta CLI de um script de `rm -rf` com nome bonito. O
// usuário precisa poder decidir, e para decidir precisa saber o que perde. Um
// teste no pacote catalog falha se algum deles ficar vazio.
type Rule struct {
	ID       string // identificador estável, usado em `mac-cleaner clean <id>`
	Name     string // rótulo exibido na TUI
	Category Category
	Risk     Risk

	What  string // o que este diretório é
	Lose  string // o que você perde ao removê-lo
	Regen string // como e a que custo ele volta

	// Detect informa se a regra faz sentido nesta máquina. Uma regra do Gradle
	// não deve aparecer no relatório de quem nunca instalou o Gradle — ruído em
	// ferramenta de limpeza vira cliques distraídos.
	Detect func(Env) bool

	// Targets lista os caminhos concretos que a regra ocupa. Regras estáticas
	// devolvem caminhos fixos; regras dinâmicas (arquivos grandes, node_modules
	// abandonados) varrem o disco e devolvem um Target por item encontrado.
	Targets func(context.Context, Env) ([]Target, error)

	// Strategy é como a remoção acontece. Ver as implementações abaixo.
	Strategy Strategy
}

// NeedsRoot informa se a regra só pode ser executada como root. A CLI nunca
// escala privilégio: nesses casos ela mede, exibe e imprime o comando pronto
// para o usuário colar, mas não executa.
func (r Rule) NeedsRoot() bool {
	_, manual := r.Strategy.(ManualOnly)
	return manual
}

// Removable informa se a CLI pode executar a remoção por conta própria.
//
// É falso para os alvos que exigem root e para os que são apenas informativos.
// A interface usa isto para decidir o que é marcável: uma caixa de seleção que
// não faz nada ao ser marcada é pior do que caixa nenhuma.
func (r Rule) Removable() bool {
	switch r.Strategy.(type) {
	case ManualOnly, ReportOnly:
		return false
	default:
		return true
	}
}

// Target é um caminho candidato à remoção, produzido por Rule.Targets.
type Target struct {
	Path  string `json:"path"`
	Label string `json:"label,omitempty"` // rótulo por item, usado por regras dinâmicas
	Size  Bytes  `json:"size"`

	// Measured indica que Size já é definitivo e o Scanner não precisa medir.
	// Regras dinâmicas descobrem o tamanho enquanto varrem; remedi-lo seria
	// percorrer o disco duas vezes.
	Measured bool `json:"measured"`
}

// Strategy descreve como uma regra libera espaço.
//
// É um conjunto fechado de tipos (a interface tem um método não exportado), e
// não um func, porque o plano de limpeza precisa ser inspecionado e exibido
// antes de rodar — o usuário confirma o comando exato, não uma caixa-preta.
type Strategy interface {
	isStrategy()
}

// RunCommand delega a limpeza ao comando oficial da própria ferramenta.
//
// É sempre preferível a apagar arquivos: `docker system prune` sabe atualizar o
// estado interno do Docker, enquanto remover diretórios por baixo dele corrompe
// o daemon.
type RunCommand struct {
	Name string
	Args []string
}

func (RunCommand) isStrategy() {}

// TrashTargets move para a Lixeira os caminhos descobertos por Rule.Targets.
//
// Usada quando não existe comando oficial. Move em vez de apagar porque é
// reversível — mas note que mover para a Lixeira NÃO libera espaço até esvaziá-la,
// e a interface é obrigada a deixar isso explícito.
type TrashTargets struct{}

func (TrashTargets) isStrategy() {}

// ManualOnly marca um alvo que exige root. A CLI mede e mostra Command, e para
// por aí.
type ManualOnly struct {
	Command string
}

func (ManualOnly) isStrategy() {}

// ReportOnly marca um alvo que a CLI encontra e exibe, mas nunca remove.
//
// É a estratégia dos arquivos grandes soltos. Eles quase sempre estão em
// Downloads, Desktop ou Documents — território do usuário, onde o guard barra
// qualquer remoção, e com razão: um .zip de 4 GB pode ser lixo esquecido ou o
// único backup de um projeto, e nada no arquivo permite à ferramenta distinguir
// os dois casos. Listar é útil; decidir por conta própria, não.
type ReportOnly struct {
	// Hint explica ao usuário o que fazer com o que foi listado.
	Hint string
}

func (ReportOnly) isStrategy() {}
