// Package cli monta a interface de linha de comando e é o composition root da
// aplicação: o único lugar que sabe qual implementação concreta satisfaz cada
// interface. Nenhum outro pacote importa cli.
package cli

import (
	"context"
	"errors"

	"github.com/danfigueroa/mac-cleaner/internal/domain"
)

// Códigos de saída, documentados no --help e estáveis para quem usa a CLI em
// script.
const (
	exitOK          = 0
	exitError       = 1
	exitUsage       = 2
	exitNothingToDo = 3
	exitUserAborted = 130 // convenção de shell para SIGINT
)

// Execute monta e roda o comando raiz.
//
// O contexto entra pelo ExecuteContext e chega a cada subcomando por
// cmd.Context(), que é o mecanismo do Cobra para isso — o contextcheck não
// enxerga esse caminho porque ele passa pela struct do comando, e não pela
// assinatura das funções.
//
//nolint:contextcheck // o ctx é propagado via ExecuteContext/cmd.Context()
func Execute(ctx context.Context, args []string) error {
	root := newRootCommand()
	root.SetArgs(args)
	return root.ExecuteContext(ctx)
}

// ExitCode traduz um erro no código de saída correspondente.
func ExitCode(err error) int {
	switch {
	case err == nil:
		return exitOK
	case errors.Is(err, context.Canceled):
		return exitUserAborted
	case errors.Is(err, domain.ErrNothingToClean):
		return exitNothingToDo
	case errors.Is(err, errUsage):
		return exitUsage
	default:
		return exitError
	}
}

// UserMessage devolve o que imprimir em stderr, ou string vazia quando o erro
// não merece mensagem.
//
// Dois casos são silenciosos de propósito: cancelar com Ctrl-C é uma escolha do
// usuário, não uma falha, e "nada a limpar" já foi comunicado pela saída normal
// do comando.
func UserMessage(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, context.Canceled):
		return ""
	case errors.Is(err, domain.ErrNothingToClean):
		return ""
	default:
		return "erro: " + err.Error()
	}
}

// errUsage marca erros de uso da CLI, que saem com código 2 em vez de 1.
var errUsage = errors.New("uso inválido")
