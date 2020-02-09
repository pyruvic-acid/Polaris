/*
 *
 * Copyright 2015 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

#include <iostream>
#include <memory>
#include <string>

#include <grpcpp/grpcpp.h>

#ifdef BAZEL_BUILD
#include "examples/protos/polaris.grpc.pb.h"
#else
#include "polaris.grpc.pb.h"
#endif

using grpc::Channel;
using grpc::ClientContext;
using grpc::Status;
using rpc::SLRequestMessage;
using rpc::LeaderIdReply;
//using rpc::SLRequestContent;
//using rpc::TxnHeaderBlock;
using rpc::TransactionHeader;
//using rpc::TransactionContent;

using rpc::Polaris;



//TransactionContent MakeTransactionContent(std::string content, std::string sender_id) {
//  TransactionContent tc;
//  tc.set_txncontent(content);
//  tc.set_sender_id(sender_id);
//  return tc;
//}

//void MakeTransactionHeader(TxnHeaderBlock* txBlock, std::string content, std::string sender_id) {
//    TransactionHeader* tm = txBlock->add_txnheaderlist();
//    tm->set_hashcontent(content);
//    tm->set_sender_id(sender_id);
//    //tm->mutable_content()->CopyFrom(tc);
//    //tm->set_signature(signature);
//}

//SLRequestContent MakeSLRequestContent(TxnBlock txBlock, std::string sender_id) {
//    SLRequestContent sc;
//    sc.mutable_txnblock()->CopyFrom(txBlock);
//    sc.set_sender_id(sender_id);
//    return sc;
//}

SLRequestMessage MakeSLRequestMessage(std::string sender_id) {
    SLRequestMessage sm;
    //sm.set_signature(signature);
    //sm.mutable_txnheaderblock()->CopyFrom(txnBlock);
    sm.set_sender_id(sender_id);
    return sm;
}

class PolarisClient {
 public:
  PolarisClient(std::shared_ptr<Channel> channel)
      : stub_(Polaris::NewStub(channel)) {}

    // Assembles the client's payload, sends it and presents the response back
    // from the server.
    std::string OrderRequest(const std::string& request) {
        //std::string content = "content";
        //std::string signature = "signature";
        std::string sender_id = "sender_id";
        
        // Data we are sending to the server.
        //TxnHeaderBlock txBlock;
        //for(auto i = 0; i < 10; i++) {
        //    std::string message = "TM" + std::to_string(i);
        //    //TransactionContent tc = MakeTransactionContent(message, sender_id);
        //    MakeTransactionHeader(&txBlock, message, sender_id);
        //    //MakeTransactionHeader(&txBlock, tc, signature);
        //}
        //SLRequestContent sc = MakeSLRequestContent(txBlock, sender_id +  ":sc");
        SLRequestMessage sm = MakeSLRequestMessage(sender_id);

        // Container for the data we expect from the server.
        LeaderIdReply reply;

        // Context for the client. It could be used to convey extra information to
        // the server and/or tweak certain RPC behaviors.
        ClientContext context;

        // The actual RPC - sends a bunch of tx;
        Status status = stub_->OrderRequest(&context, sm, &reply);
    
        // Act upon its status.
        if (status.ok()) {
                std::cout << reply.leader() << ": " << std::endl;

                return reply.leader();
        } else {
          std::cout << status.error_code() << ": " << status.error_message()
                    << std::endl;
          return "RPC failed";
        }
  }

 private:
  std::unique_ptr<Polaris::Stub> stub_;
};

int main(int argc, char** argv) {
  // Instantiate the client. It requires a channel, out of which the actual RPCs
  // are created. This channel models a connection to an endpoint (in this case,
  // localhost at port 50051). We indicate that the channel isn't authenticated
  // (use of InsecureChannelCredentials()).
  PolarisClient greeter(grpc::CreateChannel(
      "localhost:50051", grpc::InsecureChannelCredentials()));
  //std::string user("world");

  std::string reply = greeter.OrderRequest("world");
//  greeter.OrderRequest("world");
  std::cout << "Polaris received: "<< reply <<  std::endl;

  return 0;
}
