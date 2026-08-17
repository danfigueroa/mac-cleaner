// Package guard é o portão único por onde passa todo caminho antes de ser
// removido.
//
// É o pacote mais importante do repositório e o mais deliberadamente pequeno:
// nenhuma dependência além da stdlib e do domain, nenhum estado, nenhuma
// configuração externa. A ideia é que dê para ler o arquivo inteiro numa
// sentada e concluir com segurança o que ele deixa passar — uma garantia que se
// perde no instante em que ele precisar de contexto de outros seis pacotes.
//
// A regra que organiza tudo: só é removível o que está estritamente dentro do
// home, não é um diretório que o usuário reconheceria como seu, e continua
// dentro do home depois de resolver symlinks.
package guard

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danfigueroa/mac-cleaner/internal/domain"
)

// secretTrees são os diretórios onde nada, sob nenhuma justificativa, pode ser
// removido.
//
// Guardam credenciais e a correspondência do usuário. Não há exceção aplicável
// aqui: uma regra que aponte para cá é bug, e a resposta certa a um bug é
// abortar, não interpretar a intenção.
var secretTrees = []string{
	".ssh",
	".gnupg",
	".aws",
	".kube",
	".config",
	".password-store",
	".docker",
	"Library/Keychains",
	"Library/Mail",
	"Library/Messages",
	"Library/Safari",
	"Library/Photos",
	"Library/CloudStorage",
	"Library/Mobile Documents", // iCloud Drive
	".Trash",
}

// projectTrees são os diretórios onde mora o trabalho do usuário.
//
// Proibidos por padrão, com uma única exceção: diretórios de build listados em
// buildDirNames. Sem essa exceção, a regra de node_modules abandonados seria
// impossível — eles vivem exatamente aqui, dentro dos projetos.
var projectTrees = []string{
	"Documents",
	"Desktop",
	"Downloads",
	"Development",
	"Projects",
	"Movies",
	"Music",
	"Pictures",
	"Public",
	"Applications",
	"Sites",
}

// buildDirNames são nomes de diretório cujo conteúdo é sempre resultado de
// instalação ou compilação, jamais trabalho original.
//
// A lista é curta de propósito. "dist", "build" e "target" ficaram de fora ainda
// que sejam comuns: cada um deles é, em algum projeto real, uma pasta com
// conteúdo escrito à mão. Os quatro abaixo não têm essa ambiguidade — são
// sempre reconstruíveis com um comando.
var buildDirNames = map[string]struct{}{
	"node_modules": {},
	".next":        {},
	".nuxt":        {},
	".turbo":       {},
}

// protectedRoots são diretórios que não podem ser removidos, mas cujo conteúdo
// pode.
//
// Apagar ~/Library/Caches inteiro tecnicamente libera espaço e tecnicamente não
// destrói nada insubstituível — mas quebra apps que assumem a existência do
// diretório, e é o tipo de coisa que uma regra faz por engano ao usar um glob
// que não casou com nada. O conteúdo, item a item, é permitido.
var protectedRoots = []string{
	"Library",
	"Library/Caches",
	"Library/Logs",
	"Library/Application Support",
	"Library/Containers",
	"Library/Preferences",
	"Library/Developer",
	"Library/Developer/Xcode",
	"go",
	"go/pkg",
	".nvm",
	".nvm/versions",
	".nvm/versions/node",
	".gradle",
	".m2",
	".npm",
	".cargo",
	".cocoapods",
	".pub-cache",
	".nuget",
}

// Guard valida caminhos. É imutável depois de construído.
type Guard struct {
	home string

	// resolvedHome é o home com os symlinks já resolvidos.
	//
	// Os dois divergem com mais frequência do que parece: no macOS /var é link
	// para /private/var, então qualquer home sob /var — o caso de qualquer
	// diretório temporário — resolve para outro caminho. Sem guardar os dois, a
	// verificação de symlink concluiria que o alvo saiu do home e rejeitaria
	// caminhos perfeitamente válidos.
	resolvedHome string

	secrets  map[string]struct{}
	projects map[string]struct{}
	roots    map[string]struct{}
}

// New monta um Guard para o home informado.
//
// O home é parâmetro, e não lido de os.UserHomeDir() aqui dentro, para que os
// testes possam apontá-lo a um diretório temporário. Um guard que só soubesse
// se validar contra o home real seria intestável exatamente nos casos que mais
// importam.
func New(home string) *Guard {
	cleaned := filepath.Clean(home)

	resolved, err := filepath.EvalSymlinks(cleaned)
	if err != nil {
		// Home inexistente ou ilegível. Manter os dois iguais faz a verificação
		// de symlink ser rigorosa em vez de permissiva, que é o lado certo do
		// erro quando não se sabe onde se está.
		resolved = cleaned
	}

	guard := &Guard{
		home:         cleaned,
		resolvedHome: resolved,
		secrets:      make(map[string]struct{}, len(secretTrees)),
		projects:     make(map[string]struct{}, len(projectTrees)),
		roots:        make(map[string]struct{}, len(protectedRoots)),
	}

	for _, rel := range secretTrees {
		guard.secrets[filepath.Join(guard.home, rel)] = struct{}{}
	}
	for _, rel := range projectTrees {
		guard.projects[filepath.Join(guard.home, rel)] = struct{}{}
	}
	for _, rel := range protectedRoots {
		guard.roots[filepath.Join(guard.home, rel)] = struct{}{}
	}
	// O próprio home é a raiz protegida mais óbvia.
	guard.roots[guard.home] = struct{}{}

	return guard
}

// Check devolve nil apenas se o caminho puder ser removido com segurança.
//
// Toda remoção passa por aqui. O erro sempre embrulha domain.ErrGuardViolation,
// e quem o recebe deve abortar a operação inteira: uma violação significa que o
// catálogo produziu um alvo que ninguém previu, e continuar limpando os outros
// itens do mesmo plano seria confiar num componente que acabou de se mostrar
// errado.
func (g *Guard) Check(path string) error {
	if err := g.checkLexical(path); err != nil {
		return err
	}
	return g.checkResolved(path)
}

// checkLexical valida o caminho como texto, sem tocar no disco.
func (g *Guard) checkLexical(path string) error {
	if strings.TrimSpace(path) == "" {
		return g.reject(path, "caminho vazio")
	}
	if !filepath.IsAbs(path) {
		return g.reject(path, "caminho relativo")
	}

	// Clean resolve "..", então comparar com o original detecta tanto travessia
	// quanto caminhos malformados vindos de um glob mal escrito.
	cleaned := filepath.Clean(path)
	if cleaned != path {
		return g.reject(path, "caminho não normalizado (esperava %q)", cleaned)
	}

	return g.checkLocation(cleaned)
}

// checkLocation aplica as listas de proteção a um caminho já normalizado.
func (g *Guard) checkLocation(path string) error {
	if !g.withinHome(path) {
		return g.reject(path, "fora do diretório do usuário (%s)", g.home)
	}
	if _, protected := g.roots[path]; protected {
		return g.reject(path, "é um diretório estrutural; só o conteúdo dele pode ser removido")
	}

	for tree := range g.secrets {
		if path == tree || isUnder(tree, path) {
			return g.reject(path, "está dentro de %s, que guarda credenciais", tree)
		}
	}

	for tree := range g.projects {
		if path != tree && !isUnder(tree, path) {
			continue
		}
		// A exceção é estreita e vale só pelo nome final do caminho: o alvo tem
		// que ser o próprio diretório de build, não algo dentro dele. Assim
		// "~/Development/app/node_modules" passa e
		// "~/Development/app/node_modules/../src" — que o Clean já teria
		// normalizado — não teria como passar.
		if _, isBuildDir := buildDirNames[filepath.Base(path)]; isBuildDir {
			return nil
		}
		return g.reject(path, "está dentro de %s, que é território do usuário", tree)
	}

	return nil
}

// checkResolved confere para onde o caminho aponta de verdade.
//
// Sem este passo, um symlink em ~/Library/Caches apontando para fora do home
// passaria por toda a validação textual e a remoção seguiria o link. É o único
// controle que exige tocar o disco, e por isso vem por último.
func (g *Guard) checkResolved(path string) error {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Não existe, logo não há o que remover. Os testes lexicais acima já
			// garantiram que o caminho era aceitável.
			return nil
		}
		return fmt.Errorf("%w: não foi possível resolver %s: %w", domain.ErrGuardViolation, path, err)
	}

	if resolved == path {
		return nil
	}

	// O destino do link passa pelas mesmas regras do caminho original, mas antes
	// precisa voltar ao espaço de nomes do home: se o próprio home está atrás de
	// um symlink, todo caminho resolvido parece estar fora dele.
	if err := g.checkLocation(g.rebase(resolved)); err != nil {
		return fmt.Errorf("%w (via link em %s)", err, path)
	}
	return nil
}

// rebase traz um caminho já resolvido de volta para o prefixo original do home.
// Caminhos que não estejam sob o home resolvido passam intactos — e serão
// rejeitados por checkLocation, que é o desfecho correto.
func (g *Guard) rebase(resolved string) string {
	if g.resolvedHome == g.home || !isUnder(g.resolvedHome, resolved) {
		return resolved
	}
	relative := strings.TrimPrefix(resolved, g.resolvedHome+string(filepath.Separator))
	return filepath.Join(g.home, relative)
}

// withinHome informa se o caminho está estritamente dentro do home. O próprio
// home não conta.
func (g *Guard) withinHome(path string) bool {
	return isUnder(g.home, path)
}

// isUnder informa se child está estritamente abaixo de parent.
//
// A comparação acrescenta o separador de propósito: sem ele, "/Users/ana-backup"
// passaria por estar dentro de "/Users/ana".
func isUnder(parent, child string) bool {
	return strings.HasPrefix(child, parent+string(filepath.Separator))
}

func (g *Guard) reject(path, reason string, args ...any) error {
	return fmt.Errorf("%w: %s — %s", domain.ErrGuardViolation, path, fmt.Sprintf(reason, args...))
}
