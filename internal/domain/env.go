package domain

import (
	"context"
	"path/filepath"
	"slices"
)

// Env descreve a máquina onde as regras são avaliadas.
//
// Existe para que nenhuma regra chame os.UserHomeDir() ou exec.LookPath()
// diretamente. Com tudo passando por aqui, um teste aponta Home para um
// t.TempDir() e o catálogo inteiro roda contra uma árvore falsa — nenhum teste
// jamais toca o home real.
type Env struct {
	// Home é o diretório do usuário. Em testes, um diretório temporário.
	Home string

	// Root é a raiz do sistema, "/" em produção. Separado de Home porque
	// algumas regras medem caminhos fora do home (/Library/Caches), e esses
	// também precisam ser redirecionáveis em teste.
	Root string

	// LookPath resolve um executável no PATH. Injetável para que Detect seja
	// testável sem depender do que está instalado na máquina de quem roda o teste.
	LookPath func(name string) (string, error)

	// Query executa um comando de leitura e devolve a saída padrão.
	//
	// Algumas regras não conseguem descobrir seus alvos só olhando caminhos:
	// saber quais simuladores do Xcode estão órfãos exige perguntar ao simctl, e
	// quanto o Docker tem de lixo exige perguntar ao daemon. Só comandos que não
	// alteram estado passam por aqui — a remoção é responsabilidade do service,
	// atrás do guard.
	Query func(ctx context.Context, name string, args ...string) ([]byte, error)
}

// HomePath monta um caminho dentro do home do usuário.
func (e Env) HomePath(parts ...string) string {
	return filepath.Join(append([]string{e.Home}, parts...)...)
}

// SystemPath monta um caminho a partir da raiz do sistema.
func (e Env) SystemPath(parts ...string) string {
	return filepath.Join(append([]string{e.Root}, parts...)...)
}

// LibraryPath monta um caminho dentro de ~/Library — o destino da maioria das
// regras, e por isso um atalho que se paga.
func (e Env) LibraryPath(parts ...string) string {
	return e.HomePath(append([]string{"Library"}, parts...)...)
}

// HasTool informa se um executável existe no PATH. É o Detect padrão da maioria
// das regras de desenvolvimento.
func (e Env) HasTool(name string) bool {
	if e.LookPath == nil {
		return false
	}
	_, err := e.LookPath(name)
	return err == nil
}

// HasAnyTool informa se ao menos um dos executáveis existe.
func (e Env) HasAnyTool(names ...string) bool {
	return slices.ContainsFunc(names, e.HasTool)
}
