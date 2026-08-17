package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/danfigueroa/mac-cleaner/internal/adapter/memfs"
	"github.com/danfigueroa/mac-cleaner/internal/domain"
	"github.com/danfigueroa/mac-cleaner/internal/service"
)

// dirSize é o tamanho que o memfs atribui a cada diretório. Os totais esperados
// abaixo somam explicitamente esse custo, porque o Scanner conta o bloco do
// próprio diretório — como o `du` faz.
const dirSize = domain.Bytes(64)

func newScanner(fs *memfs.FS) *service.Scanner {
	// Concorrência maior que o número de diretórios nos testes garante que o
	// caminho paralelo do walker seja exercitado, e não só o inline.
	return service.NewScanner(fs, fs, service.WithConcurrency(8))
}

func TestMeasurePathSomaArvore(t *testing.T) {
	t.Parallel()

	fs := memfs.New()
	fs.MkdirAll("/root/sub")
	fs.WriteFile("/root/a.bin", 1_000)
	fs.WriteFile("/root/sub/b.bin", 2_000)

	want := 3_000 + 2*dirSize // dois diretórios: /root e /root/sub

	size, denied, err := newScanner(fs).MeasurePath(t.Context(), "/root")
	if err != nil {
		t.Fatalf("MeasurePath devolveu erro: %v", err)
	}
	if len(denied) != 0 {
		t.Errorf("denied = %v, quer vazio", denied)
	}
	if size != want {
		t.Errorf("tamanho = %d, quer %d", int64(size), int64(want))
	}
}

// TestMeasurePathDeduplicaHardlink cobre o caso que mais infla relatórios de
// disco no mundo real: o pnpm liga cada pacote de node_modules ao mesmo arquivo
// no store, e contar os dois lados dobra o número.
func TestMeasurePathDeduplicaHardlink(t *testing.T) {
	t.Parallel()

	fs := memfs.New()
	fs.MkdirAll("/store")
	fs.WriteFile("/store/pacote.tgz", 5_000)
	fs.Link("/store/copia-1.tgz", "/store/pacote.tgz")
	fs.Link("/store/copia-2.tgz", "/store/pacote.tgz")

	want := 5_000 + dirSize // o conteúdo existe uma única vez em disco

	size, _, err := newScanner(fs).MeasurePath(t.Context(), "/store")
	if err != nil {
		t.Fatalf("MeasurePath devolveu erro: %v", err)
	}
	if size != want {
		t.Errorf("tamanho = %d, quer %d — hardlink foi contado mais de uma vez",
			int64(size), int64(want))
	}
}

// TestMeasurePathParaNaFronteiraDeVolume protege contra o cenário em que o
// scanner desce por um firmlink do macOS e passa a medir o sistema operacional
// inteiro achando que é cache do usuário.
func TestMeasurePathParaNaFronteiraDeVolume(t *testing.T) {
	t.Parallel()

	fs := memfs.New()
	fs.MkdirAll("/root")
	fs.WriteFile("/root/local.bin", 1_000)
	fs.Mount("/root/outro-volume", 99)
	fs.WriteFile("/root/outro-volume/enorme.bin", 500*domain.Gigabyte)

	// O diretório de montagem em si conta; o conteúdo dele, não.
	want := 1_000 + 2*dirSize

	size, _, err := newScanner(fs).MeasurePath(t.Context(), "/root")
	if err != nil {
		t.Fatalf("MeasurePath devolveu erro: %v", err)
	}
	if size != want {
		t.Errorf("tamanho = %d, quer %d — o scanner cruzou a fronteira de volume",
			int64(size), int64(want))
	}
}

// TestMeasurePathRegistraPermissaoNegada verifica que um diretório ilegível vira
// um aviso, e não uma falha: sem Acesso Total ao Disco — a configuração padrão
// de qualquer Mac — parte de ~/Library sempre responde EACCES.
func TestMeasurePathRegistraPermissaoNegada(t *testing.T) {
	t.Parallel()

	fs := memfs.New()
	fs.MkdirAll("/root")
	fs.WriteFile("/root/visivel.bin", 1_000)
	fs.Deny("/root/protegido")

	size, denied, err := newScanner(fs).MeasurePath(t.Context(), "/root")
	if err != nil {
		t.Fatalf("MeasurePath abortou por causa de um diretório ilegível: %v", err)
	}
	if len(denied) != 1 || denied[0] != "/root/protegido" {
		t.Errorf("denied = %v, quer [/root/protegido]", denied)
	}
	if size != 1_000+dirSize {
		t.Errorf("tamanho = %d, quer %d", int64(size), int64(1_000+dirSize))
	}
}

func TestMeasurePathCaminhoInexistente(t *testing.T) {
	t.Parallel()

	size, denied, err := newScanner(memfs.New()).MeasurePath(t.Context(), "/nao/existe")
	if err != nil {
		t.Fatalf("caminho inexistente devolveu erro: %v", err)
	}
	if size != 0 || len(denied) != 0 {
		t.Errorf("tamanho = %d, denied = %v, quer 0 e vazio", int64(size), denied)
	}
}

func TestMeasurePathRespeitaCancelamento(t *testing.T) {
	t.Parallel()

	fs := memfs.New()
	for _, dir := range []string{"a", "b", "c"} {
		fs.MkdirAll("/root/" + dir)
		fs.WriteFile("/root/"+dir+"/arquivo.bin", 1_000)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, _, err := newScanner(fs).MeasurePath(ctx, "/root"); !errors.Is(err, context.Canceled) {
		t.Errorf("erro = %v, quer context.Canceled", err)
	}
}

func TestMeasurePathArquivoSolto(t *testing.T) {
	t.Parallel()

	fs := memfs.New()
	fs.WriteFile("/root/solto.bin", 4_242)

	size, _, err := newScanner(fs).MeasurePath(t.Context(), "/root/solto.bin")
	if err != nil {
		t.Fatalf("MeasurePath devolveu erro: %v", err)
	}
	if size != 4_242 {
		t.Errorf("tamanho = %d, quer 4242", int64(size))
	}
}

// TestMeasurePathNaoSegueSymlink garante que um link para fora da árvore não
// arrasta o destino para dentro da medição.
func TestMeasurePathNaoSegueSymlink(t *testing.T) {
	t.Parallel()

	fs := memfs.New()
	fs.MkdirAll("/root")
	fs.MkdirAll("/Applications")
	fs.WriteFile("/Applications/Xcode.app", 20*domain.Gigabyte)
	fs.Symlink("/root/atalho-para-apps")

	want := 8 + dirSize // apenas o próprio link, de 8 bytes

	size, _, err := newScanner(fs).MeasurePath(t.Context(), "/root")
	if err != nil {
		t.Fatalf("MeasurePath devolveu erro: %v", err)
	}
	if size != want {
		t.Errorf("tamanho = %d, quer %d", int64(size), int64(want))
	}
}
