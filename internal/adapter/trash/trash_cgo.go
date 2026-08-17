//go:build darwin && cgo

// Package trash move arquivos para a Lixeira do macOS.
package trash

/*
#cgo LDFLAGS: -framework Foundation
#include <stdlib.h>
#include "trash_darwin.h"
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// Trasher move caminhos para a Lixeira usando a API do sistema.
type Trasher struct{}

// New devolve um Trasher.
func New() Trasher { return Trasher{} }

// Trash move o caminho para a Lixeira do usuário.
//
// Importante para quem chama: isto NÃO libera espaço em disco. Os arquivos
// continuam ocupando o volume até a Lixeira ser esvaziada, e a interface é
// obrigada a dizer isso — sem o aviso, o usuário roda `df`, não vê diferença e
// conclui que a limpeza falhou.
func (Trasher) Trash(path string) error {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	var cError *C.char
	if C.mac_cleaner_trash(cPath, &cError) != 0 {
		return nil
	}

	message := "erro desconhecido"
	if cError != nil {
		message = C.GoString(cError)
		// A string veio de um strdup do lado C; liberá-la é nossa obrigação.
		C.free(unsafe.Pointer(cError))
	}
	return fmt.Errorf("movendo %s para a Lixeira: %s", path, message)
}

// Native informa que esta implementação usa a API do sistema.
func (Trasher) Native() bool { return true }
