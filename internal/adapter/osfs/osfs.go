//go:build darwin

// Package osfs implementa service.FileSystem e service.Volumer contra o disco
// real do macOS.
//
// Usa golang.org/x/sys/unix em vez do pacote syscall, que está congelado desde o
// Go 1.4 e não recebe correções. É o caminho recomendado para chamadas de
// sistema em código novo.
package osfs

import (
	"os"

	"golang.org/x/sys/unix"

	"github.com/danfigueroa/mac-cleaner/internal/domain"
	"github.com/danfigueroa/mac-cleaner/internal/service"
)

// blockSize é a unidade em que o campo st_blocks é reportado. O valor é fixo em
// 512 bytes por POSIX, independente do tamanho de bloco real do sistema de
// arquivos — é o mesmo que o `du` assume.
const blockSize = 512

// FS acessa o sistema de arquivos local. Não tem estado.
type FS struct{}

// New devolve um FS.
func New() FS { return FS{} }

// ReadDir devolve os nomes das entradas de um diretório.
//
// Usa Readdirnames em vez de os.ReadDir de propósito: os.ReadDir faz um lstat em
// cada entrada e ordena o resultado, e nós fazemos o lstat por conta própria
// logo em seguida. Numa varredura de centenas de milhares de arquivos, evitar o
// trabalho duplicado é a diferença entre segundos e dezenas deles.
func (FS) ReadDir(path string) ([]string, error) {
	dir, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = dir.Close() }()

	names, err := dir.Readdirnames(-1)
	if err != nil {
		return nil, err
	}
	return names, nil
}

// Lstat descreve um caminho sem seguir symlink.
func (FS) Lstat(path string) (service.FileInfo, error) {
	var st unix.Stat_t
	if err := unix.Lstat(path, &st); err != nil {
		// Envolver num os.PathError é o que faz errors.Is(err, fs.ErrPermission)
		// e fs.ErrNotExist funcionarem lá no scanner: unix.Errno já sabe se
		// traduzir para essas sentinelas, mas o caminho se perde sem o wrapper.
		return service.FileInfo{}, &os.PathError{Op: "lstat", Path: path, Err: err}
	}

	return service.FileInfo{
		// st_blocks, e não st_size: queremos o espaço realmente alocado. Um
		// arquivo esparso de 10 GB pode ocupar 4 KB, e a diferença entre os dois
		// números é a diferença entre prometer espaço e entregá-lo.
		Size:    domain.Bytes(st.Blocks) * blockSize,
		IsDir:   st.Mode&unix.S_IFMT == unix.S_IFDIR,
		ModTime: timespecToTime(st.Mtim),
		Inode:   st.Ino,
		Device:  uint64(uint32(st.Dev)),
		Links:   uint32(st.Nlink),
	}, nil
}

// Volume devolve a ocupação do sistema de arquivos que contém path.
func (FS) Volume(path string) (domain.Volume, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return domain.Volume{}, &os.PathError{Op: "statfs", Path: path, Err: err}
	}

	unit := domain.Bytes(st.Bsize)
	total := domain.Bytes(st.Blocks) * unit
	free := domain.Bytes(st.Bavail) * unit

	// Free vem de Bavail e bate exatamente com a coluna Avail do `df`.
	//
	// Used é derivado de Total-Free em vez de Blocks-Bfree porque, no APFS, o
	// statfs devolve Bfree igual a Bavail — a conta clássica daria o mesmo
	// resultado. O `df` do macOS mostra na coluna Used um valor bem menor,
	// obtido por outra via, que desconta espaço purgável e snapshots; a
	// porcentagem dele, porém, é calculada como fazemos aqui, e por isso ela
	// bate com a nossa. Como não dá para reproduzir os dois números do `df` ao
	// mesmo tempo a partir do statfs, a interface lidera com o espaço livre, que
	// é exato, e trata o usado como o complemento dele.
	return domain.Volume{
		Path:  unix.ByteSliceToString(st.Mntonname[:]),
		Total: total,
		Free:  free,
		Used:  total - free,
	}, nil
}
