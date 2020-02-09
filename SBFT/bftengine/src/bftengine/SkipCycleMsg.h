// No annoying copyright headers

#pragma once

#include "ClientRequestMsg.hpp"
#include "ClientMsgs.hpp"
#include "DigestType.h"
#include "Digest.hpp"

void hexDump(const void *_mem, size_t size, FILE *out);

namespace bftEngine{
    namespace duanqn{

const int NOSKIP = -1;

struct SkipCycleProof{
    // Not implemented
    int externalGroupID;
    int sigSize;
    char digest[SkipCycleDigestSize];    
    char signature[];
};

#pragma pack(push, 1)
struct ClReqHdrEx{
    struct ClientRequestMsgHeader oldHeader;

    // Set to nullptr if this is not a SkipCycle request
    struct SkipCycleProof *pProof_opt;

    // Set to bftengine::duanqn::NOSKIP if this is not a SkipCycle request
    int32_t cycleNumberHint;
};
#pragma pack(pop)

class SkipCycleClientReqMsg final : public MessageBase{
    public: 
    static bool ToActualMsgType(const bftEngine::impl::ReplicasInfo& repInfo, MessageBase* inMsg, SkipCycleClientReqMsg*& outMsg);

    SkipCycleClientReqMsg(bftEngine::impl::NodeIdType sender, bool isReadOnly, uint64_t reqSeqNum, uint32_t requestLength, const char* request, int proofOffsetInRequest, int cycleNumber);

    SkipCycleClientReqMsg(ClReqHdrEx* body);

    // virtual ~SkipCycleClientReqMsg(){}

    uint32_t maxRequestLength() const { return internalStorageSize() - sizeof(ClReqHdrEx); }

    uint16_t clientProxyId() const { return pHeader()->oldHeader.idOfClientProxy; }

    bool isReadOnly() const { return (pHeader()->oldHeader.flags & 0x1) != 0; }

    ReqId requestSeqNum() const { return pHeader()->oldHeader.reqSeqNum; }

    uint32_t requestLength() const { return pHeader()->oldHeader.requestLength; }

    int32_t skipCycleHint() const { return pHeader()->cycleNumberHint; }

    char* requestBuf() const { return body() + sizeof(ClReqHdrEx); }

    void set(ReqId reqSeqNum, uint32_t requestLength, bool isReadOnly);

    void setAsReadWrite();

    bool verifySize(size_t target){
        return pHeader()->oldHeader.requestLength + sizeof(ClReqHdrEx) == target;
    }

    void dumpHeader(){
        hexDump(pHeader(), sizeof(ClReqHdrEx), stdout);
    }

    protected:
    ClReqHdrEx* pHeader() const {
        return (ClReqHdrEx*)body();
    }
};

    }
}

