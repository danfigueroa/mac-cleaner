#ifndef MAC_CLEANER_TRASH_H
#define MAC_CLEANER_TRASH_H

// mac_cleaner_trash move o item para a Lixeira do usuário.
//
// Devolve 1 em caso de sucesso. Em caso de falha devolve 0 e, se error_out não
// for NULL, grava nele uma string alocada com malloc contendo a descrição do
// erro — a liberação fica a cargo de quem chamou.
int mac_cleaner_trash(const char *path, char **error_out);

#endif
