// Package service contém os casos de uso da aplicação: medir o disco e executar
// um plano de limpeza. É aqui que moram as interfaces das dependências externas,
// no pacote que as consome — não num pacote "ports" central, que não é como Go
// organiza abstrações.
package service

import (
	"time"

	"github.com/danfigueroa/mac-cleaner/internal/domain"
)

// FileSystem é o mínimo que o Scanner precisa do disco.
//
// Não usamos io/fs de propósito. fs.FileInfo não expõe número de blocos, inode
// nem device, e sem esses três é impossível medir uso real, deduplicar hardlink
// e parar na fronteira de volume — que é justamente o que separa uma medição
// correta de um número inventado. Testar com fstest.MapFS seria testar a coisa
// errada.
type FileSystem interface {
	// ReadDir devolve os nomes das entradas de um diretório, sem seguir links.
	ReadDir(path string) ([]string, error)

	// Lstat descreve um caminho sem seguir symlink. Não seguir é essencial: um
	// link para /Applications não deve fazer o scanner medir /Applications.
	Lstat(path string) (FileInfo, error)
}

// FileInfo é o que o scanner extrai de cada caminho.
//
// É struct concreta, não interface, porque é dado e não comportamento.
type FileInfo struct {
	// Size é o espaço realmente ocupado no disco (blocos alocados), não o
	// tamanho aparente do arquivo. Um arquivo esparso de 10 GB pode ocupar 4 KB,
	// e prometer 10 GB de espaço livre em cima dele seria mentira.
	Size domain.Bytes

	IsDir   bool
	ModTime time.Time

	// Inode e Device identificam o arquivo fisicamente. Dois caminhos com o
	// mesmo par são o mesmo dado em disco (hardlink) e devem ser contados uma
	// vez só — pnpm e Homebrew usam hardlink em escala, então sem isso o
	// relatório infla.
	Inode  uint64
	Device uint64

	// Links é a contagem de hardlinks. Serve para evitar trabalho: só arquivos
	// com Links > 1 precisam entrar na tabela de deduplicação, e eles são a
	// minoria. Guardar o inode de cada um dos milhões de arquivos de ~/Library
	// custaria mais memória do que o scan inteiro.
	Links uint32
}

// Volumer informa a ocupação do disco. Separado de FileSystem porque tem uma
// implementação e um propósito distintos: statfs, não travessia.
type Volumer interface {
	Volume(path string) (domain.Volume, error)
}
