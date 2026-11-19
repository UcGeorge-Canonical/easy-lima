#include <stddef.h>
int el_get_secret(const char *account, int interactive, void **value, size_t *length);
int el_put_secret(const char *account, const void *value, size_t length, int interactive);
int el_delete_secret(const char *account, int interactive);
void el_free_secret(void *value, size_t length);

int el_get_external_secret(const char *service,const char *account,int interactive,void **value,size_t *length);
