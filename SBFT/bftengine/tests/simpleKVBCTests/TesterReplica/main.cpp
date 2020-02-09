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

#include <stdio.h>
#include <string.h>
#include <sstream>
#include <signal.h>
#include <stdlib.h>
#include <thread>
#include <iostream>

#include "KVBCInterfaces.h"
#include "simpleKVBCTests.h"
#include "CommFactory.hpp"
#include "test_comm_config.hpp"
#include "test_parameters.hpp"
#include "MetricsServer.hpp"
#include "ReplicaImp.h"
#include "SimpleClient.hpp"
#include "PolarisKey.h"

#ifndef _WIN32
#include <sys/param.h>
#include <unistd.h>
#else
#include "winUtils.h"
#endif

#ifdef USE_LOG4CPP
#include <log4cplus/configurator.h>
#endif

using namespace SimpleKVBC;
using namespace bftEngine;
using bftEngine::SeqNumberGeneratorForClientRequests;

using namespace std;
using std::string;
using ::TestCommConfig;

#include <grpcpp/grpcpp.h>
#include <algorithm>

#ifdef BAZEL_BUILD
#include "polaris.grpc.pb.h"
#else
#include "polaris.grpc.pb.h"
#endif

using grpc::Server;
using grpc::ServerBuilder;
using grpc::ServerContext;
using grpc::Status;
using grpc::ServerWriter;

using rpc::TxnHeaderBlock;
using rpc::SLRequestMessage;
using rpc::Polaris;
using rpc::LeaderReply;
using rpc::LeaderRequest;


//static const uint16_t base_port_rpc_ = 50052;
std::string port;

IReplica* r = nullptr;
ReplicaParams rp;
concordlogger::Logger replicaLogger =
        concordlogger::Logger::getLogger("skvbctest.replica");

class ReplicaServiceImpl final : public Polaris::Service {
    ReplicaImp* replica;
    BlockId currentBlockId;
    SeqNumberGeneratorForClientRequests* pSeqGen;
public:

    ReplicaServiceImpl(ReplicaImp* r) {
        //create and init client 
        replica = r;
        currentBlockId = 0;
        pSeqGen = SeqNumberGeneratorForClientRequests::
                createSeqNumberGeneratorForClientRequests();
    }

    grpc::Status GetOrder(ServerContext* context, const LeaderRequest* request,
            ServerWriter<LeaderReply>* writer) override {
        if ((!r->isRunning())) {
            std::cout << "Oops replica is not running" << std::endl;
            return grpc::Status::CANCELLED;
        }

        //std::cout << "Replica GRPC server serve the order, " << std::endl;
        BlockId lastBlockId = replica->getLastBlock();
        while (currentBlockId <= lastBlockId) {
            std::cout << "GetOrder(): currentBlockId: " << currentBlockId << std::endl;
            SetOfKeyValuePairs updates;
            replica->getBlockData(currentBlockId, updates);

            if (updates.size() == 0) {
                currentBlockId++;
                std::cout << "Empty block; continue" << std::endl;
                continue;
            }
            std::cout << "GetOrder(): currentBlockId: " << currentBlockId 
            << " updates.size(): " << updates.size() << std::endl;

            LeaderReply reply;

            // name-removed: All keys in the block are similar.
            // The only difference is the "countInBlock" field.
            // Therefore we can extract the common information from the first KV pair.
            PolarisKey* compositeKey = (PolarisKey*) updates.begin()->first.data;
            reply.set_sequence(compositeKey->seqNum);
            reply.set_view(compositeKey->viewNum);
            reply.set_nonce(((uint64_t)0xDEADBEEF << 32) + rand());
            reply.set_last_cycle(compositeKey->lastCycle);
            reply.set_current_cycle(compositeKey->currentCycle);
            reply.mutable_commit_proof()->assign(compositeKey->thresholdSignature());

            for (auto itr = updates.begin(); itr != updates.end(); ++itr) {
                SLRequestMessage* message = reply.add_messagelist();

                if (itr->second.size > 0) {
                    message->ParseFromArray(itr->second.data, itr->second.size);
                } else {
                    message->set_sender_id("null");
                    TxnHeaderBlock* thb = new rpc::TxnHeaderBlock();
                    message->set_allocated_txnheaderblock(thb);
                    printf("Error: SLRequestMessage is empty in storage.");
                    throw 17;
                }
            }
            
            writer->Write(reply);
            currentBlockId++;
        }
        return grpc::Status::OK;
    }
};

void RunServer(ReplicaImp* r) {

    //std::string port(std::to_string(base_port_rpc_ + 2 * rp.replicaId));
    std::string server_address = "0.0.0.0:" + port;
    //initalize the service with the GRPC client
    ReplicaServiceImpl service(r);

    ServerBuilder builder;
    // Listen on the given address without any authentication mechanism.
    builder.AddListeningPort(server_address, grpc::InsecureServerCredentials());
    // Register "service" as the instance through which we'll communicate with
    // clients. In this case it corresponds to an *synchronous* service.
    builder.RegisterService(&service);
    // Finally assemble the server.
    std::unique_ptr<Server> server(builder.BuildAndStart());
  //  std::cout << "Replica GRPC Server listening on " << server_address << std::endl;

    // Wait for the server to shutdown. Note that some other thread must be
    // responsible for shutting down the server for this call to ever return.
    server->Wait();
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
    srand((unsigned int)time(nullptr));
    rp.replicaId = UINT16_MAX;

    // allows to attach debugger
    if (rp.debug) {
        std::this_thread::sleep_for(std::chrono::seconds(20));
    }

    char argTempBuffer[PATH_MAX + 10];
    string idStr;
    bool isSubstitute = false;

    int o = 0;
    while ((o = getopt(argc, argv, "r:i:k:n:s:p:t")) != EOF) {
        switch (o) {
            case 'i':
            {
                strncpy(argTempBuffer, optarg, sizeof (argTempBuffer) - 1);
                argTempBuffer[sizeof (argTempBuffer) - 1] = 0;
                idStr = argTempBuffer;
                int tempId = std::stoi(idStr);
                if (tempId >= 0 && tempId < UINT16_MAX)
                    rp.replicaId = (uint16_t) tempId;
                // TODO: check repId
            }
                break;

            case 'k':
            {
                strncpy(argTempBuffer, optarg, sizeof (argTempBuffer) - 1);
                argTempBuffer[sizeof (argTempBuffer) - 1] = 0;
                rp.keysFilePrefix = argTempBuffer;
                // TODO: check keysFilePrefix
            }
                break;

            case 'n':
            {
                strncpy(argTempBuffer, optarg, sizeof (argTempBuffer) - 1);
                argTempBuffer[sizeof (argTempBuffer) - 1] = 0;
                rp.configFileName = argTempBuffer;
            }
                break;
            case 's':
            {
                strncpy(argTempBuffer, optarg, sizeof (argTempBuffer) - 1);
                argTempBuffer[sizeof (argTempBuffer) - 1] = 0;
                idStr = argTempBuffer;
                int tempId = std::stoi(idStr);
                if (tempId >= 0 && tempId < UINT16_MAX)
                    rp.statusReportTimerMillisec = (uint16_t) tempId;
            }
                break;
            case 'p':
            {
                strncpy(argTempBuffer, optarg, sizeof (argTempBuffer) - 1);
                argTempBuffer[sizeof (argTempBuffer) - 1] = 0;
                port = argTempBuffer;
            }
                break;
            
            case 't':
            {
                isSubstitute = true;
                std::cout << "Starting a replica to substitute a failed one. Will connect to all other replicas and clients." << std::endl;
            }
                break;

            default:
                // nop
                break;
        }
    }

    if (rp.replicaId == UINT16_MAX || rp.keysFilePrefix.empty()) {
        fprintf(stderr, "%s -k KEYS_FILE_PREFIX -i ID -n COMM_CONFIG_FILE",
                argv[0]);
        exit(-1);
    }

    // Fault Tolerance: enable auto view change
    rp.viewChangeEnabled = true;
    rp.viewChangeTimeout = 15 * 1000;

    // TODO: check arguments

    //used to get info from parsing the key file
    bftEngine::ReplicaConfig replicaConfig;

    TestCommConfig testCommConfig(replicaLogger);
    testCommConfig.GetReplicaConfig(
            rp.replicaId, rp.keysFilePrefix, &replicaConfig);

    // This allows more concurrency and only affects known ids in the
    // communication classes.
    replicaConfig.numOfClientProxies = 100;
    replicaConfig.autoViewChangeEnabled = rp.viewChangeEnabled;
    replicaConfig.viewChangeTimerMillisec = rp.viewChangeTimeout;

    uint16_t numOfReplicas =
            (uint16_t) (3 * replicaConfig.fVal + 2 * replicaConfig.cVal + 1);
#ifdef USE_COMM_PLAIN_TCP
    PlainTcpConfig conf = testCommConfig.GetTCPConfig(true, rp.replicaId,
            replicaConfig.numOfClientProxies,
            numOfReplicas,
            rp.configFileName);
    conf.isSubstitute = isSubstitute;
#elif USE_COMM_TLS_TCP
    TlsTcpConfig conf = testCommConfig.GetTlsTCPConfig(true, rp.replicaId,
            replicaConfig.numOfClientProxies,
            numOfReplicas,
            rp.configFileName);
#else
    PlainUdpConfig conf = testCommConfig.GetUDPConfig(true,
            rp.replicaId,
            replicaConfig.numOfClientProxies,
            numOfReplicas,
            rp.configFileName);
#endif
    //used to run tests. TODO(IG): use the standard config structs for all tests
    SimpleKVBC::ReplicaConfig c;

    ICommunication *comm = CommFactory::create(conf);

    c.pathOfKeysfile = rp.keysFilePrefix + std::to_string(rp.replicaId);
    c.replicaId = rp.replicaId;
    c.fVal = replicaConfig.fVal;
    c.cVal = replicaConfig.cVal;
    c.numOfClientProxies = replicaConfig.numOfClientProxies;
    // Allow triggering of things like state transfer to occur faster in
    // tests.
    c.statusReportTimerMillisec = rp.statusReportTimerMillisec;
    c.concurrencyLevel = 1;
    c.autoViewChangeEnabled = rp.viewChangeEnabled;
    c.viewChangeTimerMillisec = rp.viewChangeTimeout;
    c.maxBlockSize = 30 * 1024 * 1024; // 2MB


    // UDP MetricsServer only used in tests.
    uint16_t metricsPort = conf.listenPort + 1000;
    concordMetrics::Server server(metricsPort);
    server.Start();

    r = createReplica(c, comm, BasicRandomTests::commandsHandler(), server.GetAggregator());
    r->start();
    std::cout << "Replica GRPC Server starting " << std::endl;
    std::thread t1(RunServer, (ReplicaImp*) r);

    while (r->isRunning()) {
        std::this_thread::sleep_for(std::chrono::seconds(1));
    }

    t1.join();
}
