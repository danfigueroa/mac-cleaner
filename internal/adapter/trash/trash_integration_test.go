//go:build darwin && integration

// Teste de integração: move um arquivo de verdade para a Lixeira do usuário.
//
// Fica atrás da tag `integration` porque tem efeito colateral visível — o
// arquivo aparece na Lixeira e fica lá até ser esvaziada. Um teste com esse tipo
// de consequência não pode rodar por acidente num `go test ./...`.
//
//	go test -tags integration ./internal/adapter/trash/ -v
package trash_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danfigueroa/mac-cleaner/internal/adapter/trash"
)

func TestTrashMoveParaALixeira(t *testing.T) {
	trasher := trash.New()
	t.Logf("usando a API nativa do sistema: %v", trasher.Native())

	origem := filepath.Join(t.TempDir(), "mac-cleaner-teste.txt")
	if err := os.WriteFile(origem, []byte("descartável"), 0o600); err != nil {
		t.Fatalf("criando o arquivo de teste: %v", err)
	}

	if err := trasher.Trash(origem); err != nil {
		t.Fatalf("Trash: %v", err)
	}

	if _, err := os.Stat(origem); !os.IsNotExist(err) {
		t.Errorf("o arquivo continua na origem: %v", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("descobrindo o home: %v", err)
	}
	naLixeira := filepath.Join(home, ".Trash", "mac-cleaner-teste.txt")
	if _, err := os.Stat(naLixeira); err != nil {
		t.Errorf("o arquivo não chegou em ~/.Trash: %v", err)
	} else {
		t.Logf("arquivo na Lixeira: %s (remova manualmente ou esvazie a Lixeira)", naLixeira)
	}
}
