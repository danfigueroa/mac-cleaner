//go:build darwin && integration

// Teste de integração contra o disco real. Fica atrás da tag `integration` e
// fora do CI porque depende do conteúdo da máquina de quem roda.
//
//	go test -tags integration ./internal/adapter/osfs/ -v
package osfs_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/danfigueroa/mac-cleaner/internal/adapter/osfs"
	"github.com/danfigueroa/mac-cleaner/internal/domain"
	"github.com/danfigueroa/mac-cleaner/internal/service"
)

// TestMedicaoBateComDu é o teste que mantém a ferramenta honesta.
//
// O usuário vai conferir o relatório com `du -sh` mais cedo ou mais tarde. Se os
// números divergirem, a conclusão dele será que a CLI inventa dados — e ele
// estará certo em desconfiar. A tolerância é apertada de propósito: as duas
// medições percorrem a mesma árvore com as mesmas regras (blocos alocados,
// hardlink contado uma vez, sem cruzar volume), então só a corrida com processos
// escrevendo em disco durante o teste justifica diferença.
func TestMedicaoBateComDu(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("descobrindo o home: %v", err)
	}

	candidatos := []string{
		filepath.Join(home, ".npm"),
		filepath.Join(home, "go", "pkg", "mod"),
		filepath.Join(home, ".cache"),
		filepath.Join(home, "Library", "Caches"),
	}

	fs := osfs.New()
	scanner := service.NewScanner(fs, fs)

	medidos := 0
	for _, dir := range candidatos {
		if _, err := os.Stat(dir); err != nil {
			t.Logf("pulando %s (não existe nesta máquina)", dir)
			continue
		}

		t.Run(filepath.Base(dir), func(t *testing.T) {
			esperado, err := duBytes(t, dir)
			if err != nil {
				t.Skipf("du falhou em %s: %v", dir, err)
			}

			obtido, negados, err := scanner.MeasurePath(t.Context(), dir)
			if err != nil {
				t.Fatalf("MeasurePath(%s): %v", dir, err)
			}
			if len(negados) > 0 {
				t.Skipf("%d caminhos ilegíveis em %s — sem Acesso Total ao Disco, "+
					"a comparação com du não é válida", len(negados), dir)
			}

			divergencia := diferencaRelativa(obtido, esperado)
			t.Logf("%s: mac-cleaner=%s du=%s divergência=%.2f%%",
				dir, obtido, esperado, divergencia*100)

			if divergencia > 0.05 {
				t.Errorf("divergência de %.2f%% em %s (mac-cleaner=%d, du=%d): "+
					"suspeite de bug na contagem de blocos ou na deduplicação de hardlink",
					divergencia*100, dir, int64(obtido), int64(esperado))
			}
		})
		medidos++
	}

	if medidos == 0 {
		t.Skip("nenhum dos diretórios candidatos existe nesta máquina")
	}
}

// duBytes roda `du -skx` e converte para bytes.
//
// -s resume, -k reporta em blocos de 1024, -x não cruza fronteira de volume —
// as mesmas três decisões que o nosso walker toma.
func duBytes(t *testing.T, dir string) (domain.Bytes, error) {
	t.Helper()

	out, err := exec.CommandContext(t.Context(), "du", "-skx", dir).Output()
	if err != nil {
		return 0, err
	}

	campos := strings.Fields(string(out))
	if len(campos) == 0 {
		return 0, os.ErrInvalid
	}

	kb, err := strconv.ParseInt(campos[0], 10, 64)
	if err != nil {
		return 0, err
	}
	return domain.Bytes(kb) * 1024, nil
}

func diferencaRelativa(a, b domain.Bytes) float64 {
	if b == 0 {
		if a == 0 {
			return 0
		}
		return 1
	}
	diff := float64(a - b)
	if diff < 0 {
		diff = -diff
	}
	return diff / float64(b)
}
