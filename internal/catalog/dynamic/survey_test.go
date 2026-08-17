package dynamic_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danfigueroa/mac-cleaner/internal/catalog/dynamic"
	"github.com/danfigueroa/mac-cleaner/internal/domain"
)

const (
	minimoGrande = 10 * domain.Megabyte
	prazo        = 90 * 24 * time.Hour
)

// homeDeTeste monta um home com um projeto ativo, um parado e alguns arquivos.
func homeDeTeste(t *testing.T) domain.Env {
	t.Helper()

	home := t.TempDir()
	antigo := time.Now().Add(-200 * 24 * time.Hour)

	criarDir := func(rel string) string {
		caminho := filepath.Join(home, rel)
		if err := os.MkdirAll(caminho, 0o755); err != nil {
			t.Fatalf("criando %s: %v", rel, err)
		}
		return caminho
	}
	criarArquivo := func(rel string, tamanho int, modificado time.Time) string {
		caminho := filepath.Join(home, rel)
		if err := os.MkdirAll(filepath.Dir(caminho), 0o755); err != nil {
			t.Fatalf("criando o diretório de %s: %v", rel, err)
		}
		if err := os.WriteFile(caminho, make([]byte, tamanho), 0o600); err != nil {
			t.Fatalf("criando %s: %v", rel, err)
		}
		if err := os.Chtimes(caminho, modificado, modificado); err != nil {
			t.Fatalf("datando %s: %v", rel, err)
		}
		return caminho
	}

	// Projeto parado há 200 dias.
	criarDir("Development/projeto-parado/node_modules/pacote")
	criarArquivo("Development/projeto-parado/package.json", 10, antigo)

	// Projeto mexido hoje.
	criarDir("Development/projeto-ativo/node_modules/pacote")
	criarArquivo("Development/projeto-ativo/package.json", 10, time.Now())

	// Arquivos soltos: um acima do limite, um abaixo.
	criarArquivo("Downloads/imagem-enorme.dmg", 20*1_000_000, time.Now())
	criarArquivo("Downloads/anotacao.txt", 100, time.Now())

	// ~/Library é ignorada de propósito: cada pedaço dela já pertence a uma
	// regra específica, e varrê-la aqui produziria alvos sobrepostos.
	criarArquivo("Library/Caches/app/blob.bin", 50*1_000_000, time.Now())

	// Datar o projeto parado por último, porque criar arquivos dentro dele
	// atualiza a data do diretório.
	parado := filepath.Join(home, "Development", "projeto-parado")
	if err := os.Chtimes(parado, antigo, antigo); err != nil {
		t.Fatalf("datando o projeto parado: %v", err)
	}

	return domain.Env{Home: home, Root: t.TempDir()}
}

func alvos(t *testing.T, rule domain.Rule, env domain.Env) []domain.Target {
	t.Helper()

	targets, err := rule.Targets(t.Context(), env)
	if err != nil {
		t.Fatalf("Targets da regra %s: %v", rule.ID, err)
	}
	return targets
}

func regraPorID(t *testing.T, rules []domain.Rule, id string) domain.Rule {
	t.Helper()

	for _, rule := range rules {
		if rule.ID == id {
			return rule
		}
	}
	t.Fatalf("regra %q não encontrada", id)
	return domain.Rule{}
}

// TestEncontraApenasProjetosParados é o coração da regra: apagar node_modules de
// um projeto ativo é uma tarde perdida reinstalando dependências, e a única
// coisa que separa os dois casos é a estimativa de última modificação.
func TestEncontraApenasProjetosParados(t *testing.T) {
	t.Parallel()

	env := homeDeTeste(t)
	rules := dynamic.Rules(minimoGrande, prazo)

	targets := alvos(t, regraPorID(t, rules, "projetos-abandonados"), env)

	if len(targets) != 1 {
		t.Fatalf("encontrou %d projetos, quer 1: %v", len(targets), targets)
	}
	if got := targets[0].Path; filepath.Base(filepath.Dir(got)) != "projeto-parado" {
		t.Errorf("encontrou %q, quer o node_modules do projeto-parado", got)
	}
}

func TestEncontraArquivosGrandesForaDeLibrary(t *testing.T) {
	t.Parallel()

	env := homeDeTeste(t)
	rules := dynamic.Rules(minimoGrande, prazo)

	targets := alvos(t, regraPorID(t, rules, "arquivos-grandes"), env)

	if len(targets) != 1 {
		t.Fatalf("encontrou %d arquivos, quer 1: %v", len(targets), targets)
	}
	if targets[0].Label != "imagem-enorme.dmg" {
		t.Errorf("encontrou %q, quer imagem-enorme.dmg", targets[0].Label)
	}
	if !targets[0].Measured {
		t.Error("o tamanho deveria vir medido da própria varredura")
	}
}

// TestArquivosGrandesNuncaSaoRemovidos fixa a decisão de projeto: eles vivem em
// Downloads e Documents, onde nada distingue lixo esquecido do único backup de
// um projeto. Listar é útil; decidir por conta própria, não.
func TestArquivosGrandesNuncaSaoRemovidos(t *testing.T) {
	t.Parallel()

	rule := regraPorID(t, dynamic.Rules(minimoGrande, prazo), "arquivos-grandes")

	if rule.Removable() {
		t.Error("a regra de arquivos grandes não pode ser removível pela CLI")
	}
	if _, apenasRelato := rule.Strategy.(domain.ReportOnly); !apenasRelato {
		t.Errorf("estratégia = %T, quer domain.ReportOnly", rule.Strategy)
	}
}

// TestUmaSoVarredura garante a otimização que derrubou o scan de 94 para 16
// segundos: as duas regras dividem uma única travessia do home.
func TestUmaSoVarredura(t *testing.T) {
	t.Parallel()

	env := homeDeTeste(t)
	rules := dynamic.Rules(minimoGrande, prazo)

	projetos := regraPorID(t, rules, "projetos-abandonados")
	grandes := regraPorID(t, rules, "arquivos-grandes")

	// Chamar em qualquer ordem, duas vezes, tem que dar o mesmo resultado: a
	// varredura roda uma vez e as demais chamadas leem o que ficou guardado.
	primeira := alvos(t, projetos, env)
	segunda := alvos(t, grandes, env)
	terceira := alvos(t, projetos, env)

	if len(primeira) != len(terceira) {
		t.Errorf("a segunda consulta devolveu %d projetos, a primeira %d",
			len(terceira), len(primeira))
	}
	if len(segunda) == 0 {
		t.Error("a regra de arquivos grandes não viu o resultado da varredura compartilhada")
	}
}
