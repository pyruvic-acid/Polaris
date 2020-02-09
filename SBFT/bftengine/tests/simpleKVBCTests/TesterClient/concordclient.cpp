// Concord
//
// Copyright (c) 2018 VMware, Inc. All Rights Reserved.
//
// This product is licensed to you under the Apache 2.0 license (the "License").
// You may not use this product except in compliance with the Apache 2.0
// License.
//
// This product may include a number of subcomponents with separate copyright
// notices and license terms. Your use of these subcomponents is subject to the
// terms and conditions of the subcomponent's license, as noted in the LICENSE
// file.


// Concord Client 
// - Runs a GRPC server to receive block using OrderRequest()
// - Create a concord client (c) to send to concord replica
// - Transform these blocks from polaris Class to Concord internal representation
// - Sends this to the Replica server using the concord client (c)

#include <stdio.h>
#include <string.h>
#include <signal.h>
#include <stdlib.h>
#include <thread>

#include "KVBCInterfaces.h"
#include "simpleKVBCTests.h"
#include "CommFactory.hpp"
#include "test_parameters.hpp"
#include "test_comm_config.hpp"
#include "TestDefs.h"
//#include "simpleKVBCTests.cpp"
#include "SkipCycleMsg.h"

#include <iostream>
#include <memory>
#include <string>

#include <grpcpp/grpcpp.h>

#ifdef BAZEL_BUILD
#include "polaris.grpc.pb.h"
#else
#include "polaris.grpc.pb.h"
//#include "Crypto.hpp"
#endif

using grpc::Server;
using grpc::ServerBuilder;
using grpc::ServerContext;
using grpc::Status;
using rpc::SLRequestMessage;
using rpc::LeaderIdReply;
//using rpc::TxnHeaderBlock;
using rpc::TransactionHeader;
using rpc::Polaris;

#ifndef _WIN32
#include <sys/param.h>
#include <unistd.h>
#else
#include "winUtils.h"
#endif

#ifdef USE_LOG4CPP
#include <log4cplus/configurator.h>
#endif

#include <string>

using namespace SimpleKVBC;
using namespace bftEngine;
//using namespace BasicRandomTests;
//using namespace BasicRandomTests::Internal;

using std::string;

concordlogger::Logger clientLogger =
        concordlogger::Logger::getLogger("skvbctest.client");

#pragma pack(push,1)

struct SimpleReplyHeader {
    char type; // 1 == conditional write , 2 == read, 3 == get last block

    static void free(SimpleReplyHeader* p) {
        char* p1 = (char*) p;
        delete[] p1;
    }

};

struct SimpleReplyHeader_ConditionalWrite {

    static SimpleReplyHeader_ConditionalWrite* alloc() {
        size_t s = sizeof (SimpleReplyHeader_ConditionalWrite);
        char* pBuf = new char[s];
        memset(pBuf, 0, s);
        return (SimpleReplyHeader_ConditionalWrite*) (pBuf);
    }

    static void free(SimpleReplyHeader_ConditionalWrite* p) {
        char* p1 = (char*) p;
        delete[] p1;
    }

    SimpleReplyHeader h;
    bool succ;
    BlockId latestBlock;
    char signature[128];
    size_t sigLen;
};

#pragma pack(pop)

// Logic and data behind the server's behavior.
class PolarisServiceImpl final : public Polaris::Service {
    IClient* client;
public:

    PolarisServiceImpl(IClient* c) {
        //create and init client 
        client = c;
        client->start();
    }

    grpc::Status OrderRequest(ServerContext* context, const SLRequestMessage* request,
            LeaderIdReply* reply) override {
        if ((!client->isRunning())) {
            std::cout << "Oops client not running" << std::endl;
            return grpc::Status::CANCELLED;
        }

        //std::cout << "receive senderId: " << request->sender_id() << std::endl;
        //std::cout << "receive nonce: " << request->nonce() << std::endl;

        if(!request->has_txnheaderblock()){
            printf("Error! TxnHeaderBlock cannot be null!\n");
            throw 15;
        }

        ::rpc::TxnHeaderBlock tBlock = request->txnheaderblock();
        printf("Request contains %d txn\n", tBlock.txnheaderlist_size());

        size_t size = request->ByteSizeLong();
        void *buffer = std::malloc(size + 2 * sizeof(unsigned int));
        // check return value
        request->SerializeToArray((char *)buffer + 2 * sizeof(unsigned int), size);

        // name-removed: Sneak in our 2 int fields to the front of the buffer
        if(request->cycle_suggestion() == 0U){
            // No SkipCycle request
            *(unsigned int *)((char *)buffer) = 0;
            *(unsigned int *)((char *)buffer + sizeof(unsigned int)) = bftEngine::duanqn::NOSKIP;
        }
        else{
            *(unsigned int *)((char *)buffer + sizeof(unsigned int)) = (unsigned int)(request->cycle_suggestion());

            std::string proofFromReq = request->proof();
            int proofLen = proofFromReq.size();
            
            // Search for the proof in the serialized bytes!
            // Not worth it to write a KMP...
            int matchIndex = -1;
            char * const msgStart = (char *)buffer + 2 * sizeof(unsigned int);
            for(char *ptr = msgStart; ptr + proofLen <= msgStart + size; ++ptr){
                bool match = true;
                for(int i = 0; i < proofLen; ++i){
                    // Always have: ptr + i < (char *)buffer + 2 * sizeof(unsigned int) + size
                    if(ptr[i] != proofFromReq[i]){
                        match = false;
                        break;
                    }
                }
                if(match){
                    matchIndex = ptr - msgStart;
                    break;
                }
            }
            if(matchIndex == -1){
                printf("Cannot find proof in the serialized message.\n");
                throw 10;
            }

            *(unsigned int *)((char *)buffer) = (unsigned int) matchIndex;
        }

        bool readOnly = false;
        //        Slice command((const char*) pHeader, pHeader->size());
        Slice command((const char*) buffer, size + 2 * sizeof(unsigned int));
        Slice out;
        //        
        //SLRequestMessage r;
        //r.ParseFromArray((void *) command.data, command.size);

        
        //std::cout << "receive de senderId: " << r.sender_id() << std::endl;
        //std::cout << "receive de nonce: " << r.nonce() << std::endl;
        
        client->invokeCommandSynch(command, readOnly, out);
        //        std::cout << "Invoking command..........to concord .......Done" << std::endl;
        //        
        SimpleReplyHeader_ConditionalWrite* tx = (SimpleReplyHeader_ConditionalWrite*) out.data;
        std::cout << "Got: " << tx->signature << " Len:" << tx->sigLen << " BlockId: " << tx->latestBlock << std::endl;
        //
        //        bftEngine::impl::Digest dTmpBuf;
        //        bftEngine::impl::DigestUtil::compute(command.data, command.size, (char*) &dTmpBuf, sizeof (bftEngine::impl::Digest));
        //        
        //        
        //        
        //Read the last Block: Invoke a second command for testing purposes

        client->release(out);
        client->stop();

        //        std::string prefix(sender_id + "-" + signature);
        //        Slice reply;
        //        memcmp(reply.data, pExpectedRep, expectedReplySize);
        reply->set_leader("Done");
        return grpc::Status::OK;
    }
};

//static const uint16_t base_port_rpc_ = 50051;
std::string port;

void RunServer(IClient* c) {

    //std::string port(std::to_string(base_port_rpc_ + 2 * c->getClientId()));
    std::string server_address = "0.0.0.0:" + port;

    //    std::string server_address("0.0.0.0:50051");
    //initalize the service with the GRPC client
    PolarisServiceImpl service(c);

    ServerBuilder builder;
    // Listen on the given address without any authentication mechanism.
    builder.AddListeningPort(server_address, grpc::InsecureServerCredentials());
    // Register "service" as the instance through which we'll communicate with
    // clients. In this case it corresponds to an *synchronous* service.
    builder.RegisterService(&service);
    // Finally assemble the server.
    std::unique_ptr<Server> server(builder.BuildAndStart());
    std::cout << "Concord GRPC Server listening on " << server_address << std::endl;

    // Wait for the server to shutdown. Note that some other thread must be
    // responsible for shutting down the server for this call to ever return.
    server->Wait();
}

IClient* createConfig(int argc, char **argv) {
    char argTempBuffer[PATH_MAX + 10];
    ClientParams cp;
    cp.clientId = UINT16_MAX;
    cp.numOfFaulty = UINT16_MAX;
    cp.numOfSlow = UINT16_MAX;
    cp.numOfOperations = UINT16_MAX;

    int o = 0;
    while ((o = getopt(argc, argv, "i:f:c:p:n:r:")) != EOF) {
        switch (o) {
            case 'i':
            {
                strncpy(argTempBuffer, optarg, sizeof (argTempBuffer) - 1);
                argTempBuffer[sizeof (argTempBuffer) - 1] = 0;
                std::string idStr = argTempBuffer;
                int tempId = std::stoi(idStr);
                if (tempId >= 0 && tempId < UINT16_MAX)
                    cp.clientId = (uint16_t) tempId;
                // TODO: check clientId
            }
                break;

            case 'f':
            {
                strncpy(argTempBuffer, optarg, sizeof (argTempBuffer) - 1);
                argTempBuffer[sizeof (argTempBuffer) - 1] = 0;
                std::string fStr = argTempBuffer;
                int tempfVal = std::stoi(fStr);
                if (tempfVal >= 1 && tempfVal < UINT16_MAX)
                    cp.numOfFaulty = (uint16_t) tempfVal;
                // TODO: check fVal
            }
                break;

            case 'c':
            {
                strncpy(argTempBuffer, optarg, sizeof (argTempBuffer) - 1);
                argTempBuffer[sizeof (argTempBuffer) - 1] = 0;
                std::string cStr = argTempBuffer;
                int tempcVal = std::stoi(cStr);
                if (tempcVal >= 0 && tempcVal < UINT16_MAX)
                    cp.numOfSlow = (uint16_t) tempcVal;
                // TODO: check cVal
            }
                break;

            case 'p':
            {
                strncpy(argTempBuffer, optarg, sizeof (argTempBuffer) - 1);
                argTempBuffer[sizeof (argTempBuffer) - 1] = 0;
                std::string numOfOpsStr = argTempBuffer;
                int tempfVal = std::stoi(numOfOpsStr);
                if (tempfVal >= 1 && tempfVal < UINT32_MAX)
                    cp.numOfOperations = (uint32_t) tempfVal;
                // TODO: check numOfOps
            }
                break;

            case 'n':
            {
                strncpy(argTempBuffer, optarg, sizeof (argTempBuffer) - 1);
                argTempBuffer[sizeof (argTempBuffer) - 1] = 0;
                cp.configFileName = argTempBuffer;
            }
                break;
            case 'r':
            {
                strncpy(argTempBuffer, optarg, sizeof (argTempBuffer) - 1);
                argTempBuffer[sizeof (argTempBuffer) - 1] = 0;
                port = argTempBuffer;
            }
                break;

            default:
                // nop
                break;
        }
    }

    if (cp.clientId == UINT16_MAX ||
            cp.numOfFaulty == UINT16_MAX ||
            cp.numOfSlow == UINT16_MAX ||
            cp.numOfOperations == UINT32_MAX
            ) {
        fprintf(stderr, "%s -f F -c C -p NUM_OPS -i ID -n "
                "COMM_CONFIG_FILE_NAME", argv[0]);
        exit(-1);
    }

    // TODO: check arguments

    TestCommConfig testCommConfig(clientLogger);
    uint16_t numOfReplicas = cp.get_numOfReplicas();
#ifdef USE_COMM_PLAIN_TCP
    PlainTcpConfig conf = testCommConfig.GetTCPConfig(false, cp.clientId,
            cp.numOfClients,
            numOfReplicas,
            cp.configFileName);
#elif USE_COMM_TLS_TCP
    TlsTcpConfig conf = testCommConfig.GetTlsTCPConfig(false, cp.clientId,
            cp.numOfClients,
            numOfReplicas,
            cp.configFileName);
#else
    PlainUdpConfig conf = testCommConfig.GetUDPConfig(false, cp.clientId,
            cp.numOfClients,
            numOfReplicas,
            cp.configFileName);
#endif

    ICommunication* comm = bftEngine::CommFactory::create(conf);
    SimpleKVBC::ClientConfig config;

    config.clientId = cp.clientId;
    config.fVal = cp.numOfFaulty;
    config.cVal = cp.numOfSlow;
    config.maxReplySize = maxMsgSize;

    IClient* c = createClient(config, comm);
    return c;
}

int main(int argc, char **argv) {
#if defined(_WIN32)
    initWinSock();
#endif

#ifdef USE_LOG4CPP
    using namespace log4cplus;
    initialize();
    BasicConfigurator logConfig;
    logConfig.configure();
#endif
    IClient* c = createConfig(argc, argv);
    RunServer(c);
}
