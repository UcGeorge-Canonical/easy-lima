//go:build darwin && cgo

#import <Foundation/Foundation.h>
#import <Security/Security.h>
#import <LocalAuthentication/LocalAuthentication.h>
#include <stdlib.h>
#include <string.h>
#include "keychain_darwin.h"

static NSMutableDictionary *query(const char *account, int interactive) {
    LAContext *context = [[LAContext alloc] init];
    context.interactionNotAllowed = !interactive;
    return [@{(__bridge id)kSecClass:(__bridge id)kSecClassGenericPassword,
              (__bridge id)kSecAttrService:@"io.github.easy-lima.passphrase",
              (__bridge id)kSecAttrAccount:[NSString stringWithUTF8String:account],
              (__bridge id)kSecUseAuthenticationContext:context} mutableCopy];
}
int el_get_external_secret(const char *service,const char *account, int interactive, void **value, size_t *length) {
    @autoreleasepool {
        NSMutableDictionary *q=query(account,interactive);
        q[(__bridge id)kSecAttrService]=[NSString stringWithUTF8String:service];
        q[(__bridge id)kSecReturnData]=@YES;
        q[(__bridge id)kSecMatchLimit]=(__bridge id)kSecMatchLimitOne;
        CFTypeRef result=NULL;
        OSStatus status=SecItemCopyMatching((__bridge CFDictionaryRef)q,&result);
        if (status!=errSecSuccess) return (int)status;
        NSData *data=CFBridgingRelease(result);
        if (data.length>65536) return (int)errSecDecode;
        *length=data.length;
        *value=malloc(data.length?data.length:1);
        if (!*value) return (int)errSecAllocate;
        memcpy(*value,data.bytes,data.length);
        return 0;
    }
}
int el_put_secret(const char *account,const void *value,size_t length,int interactive) {
    @autoreleasepool {
        NSMutableDictionary *q=query(account,interactive);
        NSData *data=[NSData dataWithBytes:value length:length];
        OSStatus status=SecItemUpdate((__bridge CFDictionaryRef)q,(__bridge CFDictionaryRef)@{(__bridge id)kSecValueData:data});
        if(status==errSecItemNotFound) {
            q[(__bridge id)kSecValueData]=data;
            q[(__bridge id)kSecAttrAccessible]=(__bridge id)kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly;
            status=SecItemAdd((__bridge CFDictionaryRef)q,NULL);
        }
        return (int)status;
    }
}
int el_delete_secret(const char *account,int interactive) {
    @autoreleasepool {OSStatus s=SecItemDelete((__bridge CFDictionaryRef)query(account,interactive));return s==errSecItemNotFound?0:(int)s;}
}
void el_free_secret(void *value,size_t length) {if(value){memset_s(value,length,0,length);free(value);}}

int el_get_secret(const char *account,int interactive,void **value,size_t *length) {return el_get_external_secret("io.github.easy-lima.passphrase",account,interactive,value,length);}
