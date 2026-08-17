#import <Foundation/Foundation.h>
#include <stdlib.h>
#include <string.h>

#include "trash_darwin.h"

// Usamos trashItemAtURL: em vez de mover os arquivos para ~/.Trash na mão.
//
// A API do sistema faz duas coisas que um `mv` não faz: resolve colisão de nome
// quando já existe algo com o mesmo nome na Lixeira, e grava o metadado que
// alimenta o "Colocar de Volta" do Finder. Sem esse metadado, "reversível" vira
// "os arquivos estão em algum lugar, descubra sozinho onde eles moravam".
int mac_cleaner_trash(const char *path, char **error_out) {
  @autoreleasepool {
    NSString *itemPath = [NSString stringWithUTF8String:path];
    if (itemPath == nil) {
      if (error_out != NULL) {
        *error_out = strdup("caminho não é UTF-8 válido");
      }
      return 0;
    }

    NSURL *url = [NSURL fileURLWithPath:itemPath];
    NSError *error = nil;

    BOOL moved = [[NSFileManager defaultManager] trashItemAtURL:url
                                              resultingItemURL:nil
                                                         error:&error];
    if (moved) {
      return 1;
    }

    if (error_out != NULL) {
      const char *description =
          error != nil ? [[error localizedDescription] UTF8String] : "erro desconhecido";
      *error_out = strdup(description);
    }
    return 0;
  }
}
