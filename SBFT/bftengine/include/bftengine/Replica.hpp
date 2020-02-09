// Concord
//
// Copyright (c) 2018 VMware, Inc. All Rights Reserved.
//
// This product is licensed to you under the Apache 2.0 license (the "License").
// You may not use this product except in compliance with the Apache 2.0 License.
//
// This product may include a number of subcomponents with separate copyright
// notices and license terms. Your use of these subcomponents is subject to the
// terms and conditions of the subcomponent's license, as noted in the LICENSE
// file.

#pragma once

#include <cstddef>
#include <memory>
#include <stdint.h>
#include <string>
#include "IStateTransfer.hpp"
#include "ICommunication.hpp"
#include "MetadataStorage.hpp"
#include "Metrics.hpp"
#include "ReplicaConfig.hpp"
#include "Digest.hpp"
#include "../../src/bftengine/SkipCycleMsg.h"

namespace bftEngine {
class RequestsHandler {
 public:
  virtual int execute(uint16_t clientId,
                      uint64_t sequenceNum,
                      bool readOnly,
                      uint32_t requestSize,
                      const char *request,
                      uint32_t maxReplySize,
                      char *outReply,
                      uint32_t &outActualReplySize) = 0;
  
  virtual int execute(uint16_t clientId,
                      uint64_t sequenceNum,
                      uint64_t viewNum,
                      impl::Digest40 digest,
                      const char *thresholdSignature,
                      uint16_t thresholdSignatureLen,//Note: includes vectorofshares 
                      bool readOnly,
                      uint32_t requestSize,
                      const char *request,
                      uint32_t maxReplySize,
                      char *outReply,
                      uint32_t &outActualReplySize) = 0;

  virtual int executeMulti(const std::vector<std::pair <const char*, size_t>>& commands,
                      uint64_t sequenceNum, 
                      uint64_t viewNum, 
                      bftEngine::impl::Digest40 digest, 
                      const char* thresholdSignature, uint16_t thresholdSignatureLen, 
                      uint32_t maxReplySize, 
                      char* outReply, 
                      uint32_t& outActualReplySize) = 0;

  virtual int executeEmpty(uint64_t sequenceNum,
                      uint64_t viewNum, 
                      bftEngine::impl::Digest40 digest, 
                      const char* thresholdSignature, 
                      uint16_t thresholdSignatureLen) = 0;
};

class Replica {
 public:
  static Replica *createNewReplica(ReplicaConfig *replicaConfig,
                                   RequestsHandler *requestsHandler,
                                   IStateTransfer *stateTransfer,
                                   ICommunication *communication,
                                   MetadataStorage *metadataStorage);

  static Replica *loadExistingReplica(RequestsHandler *requestsHandler,
                                      IStateTransfer *stateTransfer,
                                      ICommunication *communication,
                                      MetadataStorage *metadataStorage);

  virtual ~Replica();

  virtual bool isRunning() const = 0;

  virtual uint64_t getLastExecutedSequenceNum() const = 0;

  virtual void start() = 0;

  virtual void stop() = 0;

  virtual void SetAggregator(std::shared_ptr<concordMetrics::Aggregator> a) = 0;
};

}  // namespace bftEngine
