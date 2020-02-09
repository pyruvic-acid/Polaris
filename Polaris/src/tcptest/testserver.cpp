#include <cstdio>

#include <sys/types.h>
#include <sys/socket.h>
#include <netinet/in.h>
#include <arpa/inet.h>
#include <cstdlib>
#include <unistd.h>
#include <sys/mman.h>
#include <errno.h>
#include <sys/time.h>
#include <sys/resource.h>
#include <inttypes.h>
#include <netinet/tcp.h>
#include "common.h"

int main(int argc, char *argv[]){
    if(argc < 3){
        printf("Usage: testserver <local IP> <local port>\n");
        return 1;
    }

    struct sockaddr_in bindingAddr;
    bindingAddr.sin_family = AF_INET;
    bindingAddr.sin_port = htons(std::atoi(argv[2]));
    socklen_t addrLen = sizeof(bindingAddr);

    if(inet_aton(argv[1], &bindingAddr.sin_addr) == 0){
        printf("Invalid address: %s\n", argv[1]);
        return 2;
    }

    int skListen = socket(AF_INET, SOCK_STREAM, 0);

    if(bind(skListen, (sockaddr *)&bindingAddr, addrLen) != 0){
        printf("Cannot bind to address\n");
        return 4;
    }

    listen(skListen, max_connection);

    char *mem = (char *)malloc(len_mem);

    struct sockaddr_in clientAddr;
    addrLen = sizeof(clientAddr);
    int skConn = accept(skListen, (sockaddr *)&clientAddr, &addrLen);

    int flag = 1;
    if(setsockopt(skConn, IPPROTO_TCP, TCP_NODELAY, (void *)&flag, sizeof(flag)) != 0){
        printf("Cannot set socket options\n");
    }

    for(int t = 0; t < testNumber; ++t){
        int count = 0;
        while(count < len_mem){
            ssize_t recvsize = recv(skConn, mem + count, len_mem - count, 0);
            count += recvsize;
            if(recvsize == 0){
                break;
            }
        }

        ssize_t sendsize = send(skConn, mem, 16, 0);
        printf("Server: %d bytes received.\n", count);
        printf("Server: %d bytes sent as response.\n", int(sendsize));
    }
    
    free(mem);
    close(skConn);
    close(skListen);
    
    return 0;
}
