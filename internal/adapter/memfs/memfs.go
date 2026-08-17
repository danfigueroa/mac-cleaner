// Package memfs implementa service.FileSystem em memória, para testes.
//
// Existe porque o Scanner precisa ser testado contra situações que são difíceis
// ou impossíveis de montar num diretório temporário de verdade: hardlinks entre
// arquivos específicos, um volume separado montado no meio da árvore, e um
// diretório que devolve "permissão negada". Sem isso, os três detalhes que fazem
// a medição estar certa ficariam sem cobertura.
package memfs

import (
	"os"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/danfigueroa/mac-cleaner/internal/domain"
	"github.com/danfigueroa/mac-cleaner/internal/service"
)

// defaultDevice é o volume onde tudo é criado, salvo indicação em contrário.
const defaultDevice = 1

type entry struct {
	info     service.FileInfo
	children map[string]struct{}
	denied   bool
}

// FS é um sistema de arquivos falso. É seguro para uso concorrente, porque o
// Scanner percorre a árvore em várias goroutines.
type FS struct {
	mu      sync.RWMutex
	entries map[string]*entry
	nextIno uint64

	volume domain.Volume
}

// New cria um FS vazio contendo apenas a raiz.
func New() *FS {
	fs := &FS{
		entries: make(map[string]*entry),
		nextIno: 1,
		volume: domain.Volume{
			Path:  "/",
			Total: 100 * domain.Gigabyte,
			Free:  10 * domain.Gigabyte,
			Used:  90 * domain.Gigabyte,
		},
	}
	fs.MkdirAll("/")
	return fs
}

// MkdirAll cria o diretório e todos os ancestrais que faltarem.
func (f *FS) MkdirAll(path string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mkdirAllLocked(filepath.Clean(path))
}

func (f *FS) mkdirAllLocked(path string) *entry {
	if existing, ok := f.entries[path]; ok {
		return existing
	}

	device := uint64(defaultDevice)
	if parent := filepath.Dir(path); parent != path {
		parentEntry := f.mkdirAllLocked(parent)
		device = parentEntry.info.Device
		parentEntry.children[filepath.Base(path)] = struct{}{}
	}

	created := &entry{
		info: service.FileInfo{
			// Um diretório vazio ocupa blocos no disco real, e o Scanner conta
			// isso. Reproduzir aqui mantém as expectativas dos testes honestas.
			Size:    domain.Bytes(64),
			IsDir:   true,
			ModTime: time.Now(),
			Inode:   f.takeInode(),
			Device:  device,
			Links:   2,
		},
		children: make(map[string]struct{}),
	}
	f.entries[path] = created
	return created
}

// WriteFile cria um arquivo com o tamanho indicado.
func (f *FS) WriteFile(path string, size domain.Bytes) {
	f.mu.Lock()
	defer f.mu.Unlock()

	parent := f.mkdirAllLocked(filepath.Dir(path))
	parent.children[filepath.Base(path)] = struct{}{}

	f.entries[filepath.Clean(path)] = &entry{
		info: service.FileInfo{
			Size:    size,
			ModTime: time.Now(),
			Inode:   f.takeInode(),
			Device:  parent.info.Device,
			Links:   1,
		},
	}
}

// Link cria um hardlink para um arquivo existente: mesmo inode, mesmo tamanho,
// contagem de links incrementada nos dois caminhos.
//
// É assim que pnpm monta node_modules e como o Homebrew liga o Cellar. Um
// scanner que conte os dois lados infla o relatório.
func (f *FS) Link(newPath, existingPath string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	target, ok := f.entries[filepath.Clean(existingPath)]
	if !ok {
		panic("memfs: hardlink para caminho inexistente " + existingPath)
	}

	parent := f.mkdirAllLocked(filepath.Dir(newPath))
	parent.children[filepath.Base(newPath)] = struct{}{}

	// A contagem exata só importa para o Scanner enquanto for maior que 1: é
	// esse o gatilho da deduplicação. Por isso os dois caminhos compartilham
	// inode e tamanho, mas não sincronizamos o contador retroativamente.
	target.info.Links++
	f.entries[filepath.Clean(newPath)] = &entry{info: target.info}
}

// Symlink cria um link simbólico. No Lstat ele aparece como um arquivo comum e
// minúsculo, nunca como diretório — que é exatamente o comportamento real e o
// motivo pelo qual o Scanner não desce nele.
func (f *FS) Symlink(path string) {
	f.WriteFile(path, 8)
}

// Mount marca um diretório e tudo criado abaixo dele como pertencente a outro
// volume, simulando os firmlinks que o macOS monta dentro da árvore do usuário.
func (f *FS) Mount(path string, device uint64) {
	f.mu.Lock()
	defer f.mu.Unlock()

	mounted := f.mkdirAllLocked(filepath.Clean(path))
	mounted.info.Device = device
}

// Deny faz o caminho responder "permissão negada", como acontece em ~/Library
// quando o terminal não tem Acesso Total ao Disco.
func (f *FS) Deny(path string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	target := f.mkdirAllLocked(filepath.Clean(path))
	target.denied = true
}

// SetVolume define o que Volume devolve.
func (f *FS) SetVolume(v domain.Volume) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.volume = v
}

func (f *FS) takeInode() uint64 {
	f.nextIno++
	return f.nextIno
}

// ReadDir implementa service.FileSystem.
func (f *FS) ReadDir(path string) ([]string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	found, ok := f.entries[filepath.Clean(path)]
	if !ok {
		return nil, &os.PathError{Op: "open", Path: path, Err: syscall.ENOENT}
	}
	if found.denied {
		return nil, &os.PathError{Op: "open", Path: path, Err: syscall.EACCES}
	}
	if !found.info.IsDir {
		return nil, &os.PathError{Op: "open", Path: path, Err: syscall.ENOTDIR}
	}

	names := make([]string, 0, len(found.children))
	for name := range found.children {
		names = append(names, name)
	}
	// Ordem estável: o mapa de children não tem ordem, e um scanner correto não
	// depende dela — mas um teste que falha intermitentemente é inútil.
	sort.Strings(names)
	return names, nil
}

// Lstat implementa service.FileSystem.
func (f *FS) Lstat(path string) (service.FileInfo, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	found, ok := f.entries[filepath.Clean(path)]
	if !ok {
		return service.FileInfo{}, &os.PathError{Op: "lstat", Path: path, Err: syscall.ENOENT}
	}
	if found.denied {
		return service.FileInfo{}, &os.PathError{Op: "lstat", Path: path, Err: syscall.EACCES}
	}
	return found.info, nil
}

// Volume implementa service.Volumer.
func (f *FS) Volume(string) (domain.Volume, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.volume, nil
}
