package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// confirm pergunta ao usuário e devolve true apenas para uma resposta afirmativa
// explícita.
//
// O padrão é sempre "não": Enter no vazio, EOF de uma entrada redirecionada ou
// qualquer coisa que não seja "s"/"sim"/"y"/"yes" cancela. Numa ferramenta que
// apaga arquivos, a resposta acidental precisa ser a inofensiva.
func confirm(in io.Reader, out io.Writer, question string) bool {
	fmt.Fprintf(out, "%s [s/N] ", question)

	reader := bufio.NewReader(in)
	answer, err := reader.ReadString('\n')
	if err != nil && answer == "" {
		return false
	}

	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "s", "sim", "y", "yes":
		return true
	default:
		return false
	}
}
