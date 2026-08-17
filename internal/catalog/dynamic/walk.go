// Package dynamic contém as regras que precisam varrer o disco para descobrir
// seus alvos, em vez de apontar para caminhos conhecidos.
//
// Elas são construídas por função, e não declaradas como valores, porque
// dependem de parâmetros que o usuário controla: o tamanho mínimo de um arquivo
// grande e a idade a partir da qual um projeto conta como abandonado.
package dynamic

import (
	"io/fs"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/danfigueroa/mac-cleaner/internal/domain"
)

// maxDepth limita quão fundo a varredura desce a partir do home.
//
// Sem limite, um único diretório com dezenas de milhares de projetos faria a
// abertura da ferramenta demorar minutos. Seis níveis alcançam
// ~/código/cliente/projeto/pacotes/api/node_modules, que já é mais fundo do que
// a esmagadora maioria das árvores reais.
const maxDepth = 6

// skipDirNames são diretórios em que nenhuma regra dinâmica precisa entrar.
//
// ~/Library fica de fora porque cada pedaço relevante dela já pertence a uma
// regra específica, e varrê-la de novo produziria alvos sobrepostos. Os demais
// são grandes, ruidosos e nunca contêm o que estas regras procuram.
// node_modules e os demais diretórios de build NÃO entram aqui: a regra de
// projetos parados existe justamente para encontrá-los, e ela é quem decide não
// descer. Confundir os dois papéis foi um bug real — a lista abaixo é sobre o
// que ninguém quer nem ver.
var skipDirNames = map[string]struct{}{
	"Library": {},
	".Trash":  {},
	".git":    {},
	".venv":   {},
	"venv":    {},
	"Pods":    {},
	".cache":  {},
}

// walkHome percorre o home aplicando os limites acima e chamando visit em cada
// entrada que sobreviver aos filtros.
//
// visit devolve true quando a subárvore não deve ser percorrida — é assim que a
// regra de node_modules evita descer nos milhares de arquivos de um diretório
// que ela já decidiu incluir inteiro.
func walkHome(root string, visit func(path string, entry fs.DirEntry) (skip bool)) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			// Diretório ilegível: seguir adiante. As regras dinâmicas são um
			// complemento, e derrubar a varredura inteira por causa de um
			// caminho sem permissão seria desproporcional.
			return nil //nolint:nilerr // um caminho ilegível não invalida a varredura
		}
		if path == root {
			return nil
		}

		if entry.IsDir() {
			if shouldSkipDir(root, path, entry) {
				return filepath.SkipDir
			}
		}

		if visit(path, entry) {
			return filepath.SkipDir
		}
		return nil
	})
}

func shouldSkipDir(root, path string, entry fs.DirEntry) bool {
	// Diretórios de build precisam ser vistos antes de pular: quem decide não
	// descer neles é a regra que os procura, no visit.
	if isBuildDirName(entry.Name()) {
		return false
	}
	if _, skip := skipDirNames[entry.Name()]; skip {
		return true
	}
	if depth(root, path) > maxDepth {
		return true
	}
	// Diretórios ocultos que não são alvo de nenhuma regra dinâmica: quase
	// sempre configuração de ferramenta, e nunca projetos ou arquivos soltos.
	return strings.HasPrefix(entry.Name(), ".")
}

func depth(root, path string) int {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return maxDepth + 1
	}
	return strings.Count(relative, string(filepath.Separator)) + 1
}

// diskSize devolve o espaço realmente alocado, e não o tamanho aparente.
//
// A distinção importa justamente aqui: imagens de disco e arquivos esparsos são
// exatamente o tipo de coisa que aparece numa lista de arquivos grandes, e um
// .dmg esparso de 50 GB pode ocupar poucos megabytes.
func diskSize(info fs.FileInfo) domain.Bytes {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return domain.Bytes(stat.Blocks) * 512
	}
	return domain.Bytes(info.Size())
}

func isBuildDirName(name string) bool {
	_, ok := buildDirNames[name]
	return ok
}
