package guard_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/danfigueroa/mac-cleaner/internal/domain"
	"github.com/danfigueroa/mac-cleaner/internal/guard"
)

const home = "/Users/ana"

// TestRejeita é a tabela hostil: cada linha é um caminho que, se algum dia
// passar, significa que a ferramenta pode destruir algo insubstituível.
//
// Ela é escrita de fora para dentro — não a partir do que o catálogo produz
// hoje, mas do que jamais deve ser aceito venha de onde vier. É essa inversão
// que faz o teste continuar valendo quando alguém adicionar uma regra nova
// daqui a um ano.
func TestRejeita(t *testing.T) {
	t.Parallel()

	casos := []struct {
		nome string
		path string
	}{
		{"vazio", ""},
		{"só espaços", "   "},
		{"relativo", "Library/Caches/algo"},
		{"raiz do sistema", "/"},
		{"fora do home", "/etc/passwd"},
		{"biblioteca do sistema", "/System/Library/Caches"},
		{"pasta de aplicativos", "/Applications/Xcode.app"},
		{"o próprio home", home},
		{"o pai do home", "/Users"},

		{"Documents", home + "/Documents"},
		{"arquivo em Documents", home + "/Documents/tese.pdf"},
		{"subpasta de Documents", home + "/Documents/2026/fotos"},
		{"Desktop", home + "/Desktop"},
		{"Downloads", home + "/Downloads/instalador.dmg"},
		{"Development", home + "/Development/projeto-do-cliente"},
		{"Pictures", home + "/Pictures/Fotos.photoslibrary"},
		{"Movies", home + "/Movies"},
		{"iCloud Drive", home + "/Library/Mobile Documents/algo"},
		{"Lixeira", home + "/.Trash"},

		{"chaves SSH", home + "/.ssh"},
		{"chave privada", home + "/.ssh/id_ed25519"},
		{"GnuPG", home + "/.gnupg"},
		{"credenciais AWS", home + "/.aws/credentials"},
		{"kubeconfig", home + "/.kube/config"},
		{"configuração", home + "/.config/gh/hosts.yml"},
		{"chaveiro", home + "/Library/Keychains/login.keychain-db"},
		{"Mail", home + "/Library/Mail/V10"},
		{"Mensagens", home + "/Library/Messages/chat.db"},

		{"Library nua", home + "/Library"},
		{"Caches nu", home + "/Library/Caches"},
		{"Logs nu", home + "/Library/Logs"},
		{"Application Support nu", home + "/Library/Application Support"},
		{"Containers nu", home + "/Library/Containers"},
		{"Developer nu", home + "/Library/Developer"},
		{"go nu", home + "/go"},
		{"go/pkg nu", home + "/go/pkg"},
		{"nvm nu", home + "/.nvm"},
		{"versões do nvm nuas", home + "/.nvm/versions/node"},
		{"npm nu", home + "/.npm"},
		{"gradle nu", home + "/.gradle"},

		{"travessia com ..", home + "/Library/Caches/../../Documents"},
		{"travessia até a raiz", home + "/Library/Caches/../../../.."},
		{"barra dupla", home + "//Library//Caches//algo"},
		{"barra final", home + "/Library/Caches/algo/"},

		// O clássico: comparação por prefixo sem o separador aceitaria isto
		// como se estivesse dentro de /Users/ana.
		{"home de nome parecido", "/Users/ana-backup/Library/Caches/algo"},
		{"sufixo colado no home", "/Users/anabela/Documents"},
	}

	guardian := guard.New(home)

	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			t.Parallel()

			err := guardian.Check(caso.path)
			if err == nil {
				t.Fatalf("guard ACEITOU %q — este caminho jamais pode ser removido", caso.path)
			}
			if !errors.Is(err, domain.ErrGuardViolation) {
				t.Errorf("erro = %v, quer que embrulhe ErrGuardViolation", err)
			}
		})
	}
}

// TestAceita cobre o outro lado: um guard que rejeitasse tudo seria trivialmente
// seguro e completamente inútil.
func TestAceita(t *testing.T) {
	t.Parallel()

	casos := []struct {
		nome string
		path string
	}{
		{"cache do Go", home + "/Library/Caches/go-build"},
		{"cache de app qualquer", home + "/Library/Caches/com.exemplo.app"},
		{"cache dentro de fabricante", home + "/Library/Caches/Google/Chrome"},
		{"VMs do Claude", home + "/Library/Application Support/Claude/vm_bundles"},
		{"cache de app Electron", home + "/Library/Application Support/Cursor/CachedData"},
		{"logs de um app", home + "/Library/Logs/Discord"},
		{"DerivedData", home + "/Library/Developer/Xcode/DerivedData"},
		{"símbolos de dispositivo", home + "/Library/Developer/Xcode/iOS DeviceSupport/iPhone15,2 18.6.2"},
		{"módulos do Go", home + "/go/pkg/mod"},
		{"versão do nvm", home + "/.nvm/versions/node/v18.20.5"},
		{"cache do npm", home + "/.npm/_cacache"},
		{"caches do gradle", home + "/.gradle/caches"},
		{"repositório do maven", home + "/.m2/repository"},
	}

	guardian := guard.New(home)

	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			t.Parallel()

			if err := guardian.Check(caso.path); err != nil {
				t.Errorf("guard REJEITOU um alvo legítimo %q: %v", caso.path, err)
			}
		})
	}
}

// TestRejeitaSymlinkQueEscapa cobre o ataque que nenhuma validação textual pega:
// um caminho impecável dentro de ~/Library/Caches que, na verdade, aponta para
// fora do home. Sem resolver o link, a remoção seguiria para o destino.
func TestRejeitaSymlinkQueEscapa(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	caches := filepath.Join(base, "Library", "Caches")
	if err := os.MkdirAll(caches, 0o755); err != nil {
		t.Fatalf("montando o home: %v", err)
	}

	// O destino é um diretório fora do home, como ~/Documents seria.
	foraDoHome := t.TempDir()

	armadilha := filepath.Join(caches, "cache-inocente")
	if err := os.Symlink(foraDoHome, armadilha); err != nil {
		t.Fatalf("criando o symlink: %v", err)
	}

	err := guard.New(base).Check(armadilha)
	if err == nil {
		t.Fatal("guard ACEITOU um symlink que aponta para fora do home")
	}
	if !errors.Is(err, domain.ErrGuardViolation) {
		t.Errorf("erro = %v, quer que embrulhe ErrGuardViolation", err)
	}
}

// TestAceitaSymlinkInterno garante que a proteção acima não é rígida demais:
// links dentro do próprio home são comuns e legítimos.
func TestAceitaSymlinkInterno(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	caches := filepath.Join(base, "Library", "Caches")
	destino := filepath.Join(caches, "dados-reais")
	if err := os.MkdirAll(destino, 0o755); err != nil {
		t.Fatalf("montando o home: %v", err)
	}

	link := filepath.Join(caches, "atalho")
	if err := os.Symlink(destino, link); err != nil {
		t.Fatalf("criando o symlink: %v", err)
	}

	if err := guard.New(base).Check(link); err != nil {
		t.Errorf("guard rejeitou um link interno legítimo: %v", err)
	}
}

// TestHomeAtrasDeSymlink cobre o caso que quebraria a ferramenta em silêncio:
// no macOS /var é link para /private/var, então qualquer home sob /var faz todo
// caminho resolvido parecer estar fora dele.
func TestHomeAtrasDeSymlink(t *testing.T) {
	t.Parallel()

	base := t.TempDir() // no macOS, tipicamente /var/folders/...
	alvo := filepath.Join(base, "Library", "Caches", "algum-cache")
	if err := os.MkdirAll(alvo, 0o755); err != nil {
		t.Fatalf("montando o home: %v", err)
	}

	if err := guard.New(base).Check(alvo); err != nil {
		t.Errorf("guard rejeitou um alvo válido num home atrás de symlink: %v", err)
	}
}

// TestAceitaDiretoriosDeBuildEmProjetos cobre a exceção estreita que torna a
// regra de node_modules abandonados possível: dentro de território do usuário,
// só passam diretórios cujo conteúdo é sempre reconstruível por um comando.
func TestAceitaDiretoriosDeBuildEmProjetos(t *testing.T) {
	t.Parallel()

	guardian := guard.New(home)

	permitidos := []string{
		home + "/Development/app/node_modules",
		home + "/Documents/site/.next",
		home + "/Desktop/experimento/node_modules",
		home + "/Development/mono/pacotes/api/.turbo",
	}
	for _, path := range permitidos {
		if err := guardian.Check(path); err != nil {
			t.Errorf("guard rejeitou um diretório de build legítimo %q: %v", path, err)
		}
	}

	// A exceção não pode vazar para o conteúdo do projeto nem para credenciais.
	proibidos := []string{
		home + "/Development/app/src",
		home + "/Development/app/node_modules/pacote/README.md",
		home + "/Development/app/dist",
		home + "/Development/app/build",
		home + "/Development/app/target",
		home + "/.ssh/node_modules",
		home + "/.aws/node_modules",
		home + "/Library/Keychains/node_modules",
	}
	for _, path := range proibidos {
		if err := guardian.Check(path); err == nil {
			t.Errorf("guard ACEITOU %q — a exceção de build vazou", path)
		}
	}
}
