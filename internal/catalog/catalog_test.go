package catalog_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/danfigueroa/mac-cleaner/internal/catalog"
	"github.com/danfigueroa/mac-cleaner/internal/domain"
)

var idPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// TestRegrasSaoBemFormadas é o contrato que separa esta CLI de um script de rm
// com nome bonito: nenhuma regra chega ao usuário sem dizer o que é, o que ele
// perde e como aquilo volta. Uma regra sem essas três frases não é acionável, e
// o único jeito de decidir sobre ela seria confiar cegamente na ferramenta.
func TestRegrasSaoBemFormadas(t *testing.T) {
	t.Parallel()

	for _, rule := range catalog.All() {
		t.Run(rule.ID, func(t *testing.T) {
			t.Parallel()

			if !idPattern.MatchString(rule.ID) {
				t.Errorf("ID %q não está em kebab-case — ele aparece em `mac-cleaner clean <id>`", rule.ID)
			}
			if rule.Name == "" {
				t.Error("Name vazio")
			}
			if !rule.Category.Valid() {
				t.Errorf("categoria %q é desconhecida", rule.Category)
			}

			for campo, valor := range map[string]string{
				"What":  rule.What,
				"Lose":  rule.Lose,
				"Regen": rule.Regen,
			} {
				if strings.TrimSpace(valor) == "" {
					t.Errorf("%s vazio: o usuário não teria como decidir sobre esta regra", campo)
				}
			}

			if rule.Targets == nil {
				t.Error("Targets nulo: a regra não sabe dizer o que mediria")
			}
			if rule.Strategy == nil {
				t.Error("Strategy nula: a regra não sabe como se limpa")
			}
		})
	}
}

func TestIDsSaoUnicos(t *testing.T) {
	t.Parallel()

	vistos := make(map[string]struct{})
	for _, rule := range catalog.All() {
		if _, duplicado := vistos[rule.ID]; duplicado {
			t.Errorf("ID duplicado: %q", rule.ID)
		}
		vistos[rule.ID] = struct{}{}
	}
}

// TestAlvosNaoSeSobrepoem garante que duas regras nunca reivindicam o mesmo
// espaço em disco.
//
// Uma sobreposição não causa erro visível — causa algo pior: o total prometido
// na tela fica maior que o espaço que existe para liberar, o usuário aprova a
// limpeza esperando 20 GB, recebe 12, e conclui que a ferramenta exagera. Como
// as regras de varredura ampla usam listas de exclusão para ceder o que pertence
// às regras específicas, este teste é o que impede que uma dessas listas fique
// desatualizada quando alguém adicionar a próxima regra.
func TestAlvosNaoSeSobrepoem(t *testing.T) {
	t.Parallel()

	env := homeFalso(t)

	type alvo struct {
		regra string
		path  string
	}
	var todos []alvo

	for _, rule := range catalog.All() {
		targets, err := rule.Targets(t.Context(), env)
		if err != nil {
			t.Fatalf("regra %s: Targets devolveu erro: %v", rule.ID, err)
		}
		for _, target := range targets {
			if target.Path == "" {
				continue // alvo lógico (Docker), sem caminho em disco
			}
			todos = append(todos, alvo{regra: rule.ID, path: filepath.Clean(target.Path)})
		}
	}

	for i, a := range todos {
		for _, b := range todos[i+1:] {
			if a.regra == b.regra {
				continue
			}
			if contémOuIgual(a.path, b.path) {
				t.Errorf("regras %q e %q reivindicam o mesmo espaço:\n  %s\n  %s",
					a.regra, b.regra, a.path, b.path)
			}
		}
	}
}

// contémOuIgual informa se um dos caminhos é igual ao outro ou ancestral dele.
func contémOuIgual(a, b string) bool {
	if a == b {
		return true
	}
	return strings.HasPrefix(b, a+string(filepath.Separator)) ||
		strings.HasPrefix(a, b+string(filepath.Separator))
}

func TestByIDsRejeitaDesconhecido(t *testing.T) {
	t.Parallel()

	if _, err := catalog.ByIDs([]string{"npm-cache", "regra-que-nao-existe"}); !errors.Is(err, domain.ErrRuleNotFound) {
		t.Errorf("erro = %v, quer ErrRuleNotFound", err)
	}
}

func TestByIDsPreservaOrdem(t *testing.T) {
	t.Parallel()

	rules, err := catalog.ByIDs([]string{"user-logs", "npm-cache"})
	if err != nil {
		t.Fatalf("ByIDs: %v", err)
	}
	if len(rules) != 2 || rules[0].ID != "user-logs" || rules[1].ID != "npm-cache" {
		t.Errorf("ordem não preservada: %v", idsDe(rules))
	}
}

func TestParseCategories(t *testing.T) {
	t.Parallel()

	if _, err := catalog.ParseCategories([]string{"dev", "apps"}); err != nil {
		t.Errorf("categorias válidas devolveram erro: %v", err)
	}
	if _, err := catalog.ParseCategories([]string{"inexistente"}); !errors.Is(err, domain.ErrInvalidCategory) {
		t.Errorf("erro = %v, quer ErrInvalidCategory", err)
	}
}

func TestFilterByCategories(t *testing.T) {
	t.Parallel()

	todas := catalog.All()
	if got := catalog.FilterByCategories(todas, nil); len(got) != len(todas) {
		t.Errorf("lista vazia deveria não filtrar: %d de %d", len(got), len(todas))
	}

	apenasApps := catalog.FilterByCategories(todas, []domain.Category{domain.CategoryApps})
	if len(apenasApps) == 0 {
		t.Fatal("nenhuma regra na categoria apps")
	}
	for _, rule := range apenasApps {
		if rule.Category != domain.CategoryApps {
			t.Errorf("regra %s tem categoria %s", rule.ID, rule.Category)
		}
	}
}

// homeFalso monta uma árvore com a estrutura que as regras esperam encontrar,
// para que os globs tenham o que casar. Fica num t.TempDir(): nenhum teste do
// catálogo toca o home real.
func homeFalso(t *testing.T) domain.Env {
	t.Helper()

	home := t.TempDir()
	dirs := []string{
		"Library/Caches/go-build",
		"Library/Caches/Homebrew",
		"Library/Caches/Yarn",
		"Library/Caches/pip",
		"Library/Caches/CocoaPods",
		"Library/Caches/org.swift.swiftpm",
		"Library/Caches/JetBrains/IntelliJIdea2024.1",
		"Library/Caches/Google/Chrome",
		"Library/Caches/BraveSoftware",
		"Library/Caches/com.apple.appstoreagent",
		"Library/Caches/algum-app-qualquer",
		"Library/Logs/JetBrains/IntelliJIdea2024.1",
		"Library/Logs/Discord",
		"Library/Updates",
		"Library/Application Support/Claude/vm_bundles",
		"Library/Application Support/Claude/Cache",
		"Library/Application Support/Cursor/CachedData",
		"Library/Application Support/Code/Cache",
		"Library/Application Support/MobileSync/Backup/00008110-000",
		"Library/Developer/Xcode/DerivedData",
		"Library/Developer/Xcode/UserData/Previews",
		"Library/Developer/Xcode/Archives/2026-01-01",
		"Library/Developer/Xcode/iOS DeviceSupport/iPhone15,2 18.6.2",
		"Library/Developer/CoreSimulator/Caches",
		".nvm/versions/node/v22.21.0",
		".nvm/versions/node/v18.20.5",
		".npm/_cacache",
		".gradle/caches",
		".m2/repository",
		"go/pkg/mod",
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(home, dir), 0o755); err != nil {
			t.Fatalf("montando o home falso: %v", err)
		}
	}

	return domain.Env{
		Home: home,
		Root: t.TempDir(),
		// Toda ferramenta "existe", para que nenhuma regra seja pulada por
		// Detect e o teste cubra o catálogo inteiro.
		LookPath: func(name string) (string, error) { return "/usr/bin/" + name, nil },
		// Nenhum comando responde: as regras que dependem de consultar o Docker
		// ou o simctl devem lidar com isso sem quebrar.
		Query: func(context.Context, string, ...string) ([]byte, error) {
			return nil, errors.New("sem comandos externos em teste")
		},
	}
}

func idsDe(rules []domain.Rule) []string {
	ids := make([]string, 0, len(rules))
	for _, rule := range rules {
		ids = append(ids, rule.ID)
	}
	return ids
}
