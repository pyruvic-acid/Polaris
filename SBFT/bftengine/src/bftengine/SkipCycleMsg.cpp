#include "SkipCycleMsg.h"
#include "assertUtils.hpp"
#include <cstring>
using namespace bftEngine::duanqn;

void hexDump(const void *_mem, size_t size, FILE *out){
    const char *mem = (const char *)_mem;
    int * intptr = nullptr;
    fprintf(out, "=====Dump start=====\n");
    for(intptr = (int *)mem; (char *)(intptr + 1) <= mem + size; intptr++){
        fprintf(out, "%p: %08X\n", intptr, *intptr);
    }
    fprintf(out, "%p: ", intptr);
    for(char * charptr = (char *)intptr; charptr < mem + size; charptr++){
        fprintf(out, "%02hhX", *charptr);
    }
    fprintf(out, "\n");
    fprintf(out, "=====Dump end=====\n");
}

SkipCycleClientReqMsg::SkipCycleClientReqMsg(bftEngine::impl::NodeIdType sender, bool isReadOnly, uint64_t reqSeqNum, uint32_t requestLength, const char* requestAndSkipCycleProof, int proofOffsetInRequest, int cycleNumber):  MessageBase(sender, MsgCode::Request, (sizeof(ClReqHdrEx) + requestLength)){
    ClReqHdrEx *header = pHeader();
    header->oldHeader.idOfClientProxy = sender;
    header->oldHeader.flags = 0;
    if (isReadOnly) header->oldHeader.flags |= 0x1;
    header->oldHeader.reqSeqNum = reqSeqNum;
    header->oldHeader.requestLength = requestLength;

    memcpy(body() + sizeof(ClReqHdrEx), requestAndSkipCycleProof, requestLength);
    header->cycleNumberHint = cycleNumber;
    header->pProof_opt = (SkipCycleProof *)(body() + sizeof(ClReqHdrEx) + proofOffsetInRequest);
}

SkipCycleClientReqMsg::SkipCycleClientReqMsg(ClReqHdrEx* body): MessageBase(body->oldHeader.idOfClientProxy, (bftEngine::impl::MessageBase::Header*)body, (sizeof(ClReqHdrEx) + body->oldHeader.requestLength), false){}

bool SkipCycleClientReqMsg::ToActualMsgType(const bftEngine::impl::ReplicasInfo& repInfo, MessageBase* inMsg, SkipCycleClientReqMsg*& outMsg){
    Assert(inMsg->type() == MsgCode::Request);
    if (inMsg->size() < sizeof(SkipCycleClientReqMsg)) return false;

    SkipCycleClientReqMsg* t = (SkipCycleClientReqMsg*)inMsg;

    if (t->size() < (sizeof(ClReqHdrEx) + t->pHeader()->oldHeader.requestLength)) return false;

    outMsg = t;

    return true;
}