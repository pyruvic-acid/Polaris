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
#include <netinet/tcp.h>
#include <ctime>
#include <chrono>
#include <errno.h>
#include "common.h"

int main(int argc, char *argv[]){
    if(argc < 3){
        printf("Usage: testclient <remote IP> <remote port>\n");
        return 1;
    }

    struct sockaddr_in serverAddr;
    serverAddr.sin_family = AF_INET;
    serverAddr.sin_port = htons(std::atoi(argv[2]));
    socklen_t addrLen = sizeof(serverAddr);

    if(inet_aton(argv[1], &serverAddr.sin_addr) == 0){
        printf("Invalid address: %s\n", argv[1]);
        return 2;
    }

    int skConn = socket(AF_INET, SOCK_STREAM, 0);
    int flag = 1;
    if(setsockopt(skConn, IPPROTO_TCP, TCP_NODELAY, (void *)&flag, sizeof(flag)) != 0){
        printf("Cannot set socket options\n");
    }

    if(connect(skConn, (sockaddr *)&serverAddr, addrLen) != 0){
        printf("Cannot connect to server\n");
        return 3;
    }

    char *mem = (char *)malloc(len_mem);

    double accElapsedTime = 0;
    for(int t = 0; t < testNumber; ++t){
        auto t_start = std::chrono::high_resolution_clock::now();

        int accSendSize = 0;
        do{
            ssize_t sendsize = send(skConn, mem + accSendSize, len_mem - accSendSize, 0);

            if(sendsize == -1){
                int err = errno;
                printf("send error! errno = %d\n", err);
                return 1;
            }
            accSendSize += sendsize;
        }while(accSendSize < len_mem);

        int accRecvSize = 0;
        while(accRecvSize < len_ack){
            ssize_t recvsize = recv(skConn, mem + accRecvSize, len_ack - accRecvSize, 0);
            accRecvSize += recvsize;
            if(recvsize == 0){
                break;
            }
        }
        
        auto t_end = std::chrono::high_resolution_clock::now();
        printf("Client: %d bytes sent. (Should be %d)\n", accSendSize, len_mem);
        printf("Client: %d bytes received. (Should be %d)\n", accRecvSize, len_ack);

        double elapsed_secs = std::chrono::duration<double>(t_end-t_start).count();
        accElapsedTime += elapsed_secs;
        printf("Client: %d bytes ACKed in %lf seconds.\n", len_mem, elapsed_secs);
        printf("Client: Thoughput is %lf bytes/s\n", len_mem / elapsed_secs);
    }

    printf("Client: Avg throughput is %lf bytes/s\n", (len_mem / accElapsedTime) * testNumber);

    free(mem);
    close(skConn);

    return 0;
}
