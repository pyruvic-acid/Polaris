//Concord
//
//Copyright (c) 2018 VMware, Inc. All Rights Reserved.
//
//This product is licensed to you under the Apache 2.0 license (the "License").  You may not use this product except in compliance with the Apache 2.0 License. 
//
//This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.

#include "PrePrepareMsg.hpp"
#include "SysConsts.hpp"
#include "Crypto.hpp"
#include "ClientRequestMsg.hpp"

namespace bftEngine
{
	namespace impl
	{
		static Digest40 nullDigest(0x18);

		///////////////////////////////////////////////////////////////////////////////
		// PrePrepareMsg
		///////////////////////////////////////////////////////////////////////////////

		MsgSize PrePrepareMsg::maxSizeOfPrePrepareMsg()
		{
			return maxExternalMessageSize;
		}

		MsgSize PrePrepareMsg::maxSizeOfPrePrepareMsgInLocalBuffer()
		{
			return maxSizeOfPrePrepareMsg() + sizeof(RawHeaderOfObjAndMsg);
		}

		PrePrepareMsg* PrePrepareMsg::createNullPrePrepareMsg(ReplicaId sender, ViewNum v, SeqNum s, CommitPath firstPath)
		{
			PrePrepareMsg* p = new PrePrepareMsg(sender, v, s, firstPath, true);
			return p;
		}

		const Digest40& PrePrepareMsg::digestOfNullPrePrepareMsg()
		{
			return nullDigest;
		}

		bool PrePrepareMsg::ToActualMsgType(const ReplicasInfo& repInfo, MessageBase* inMsg, PrePrepareMsg*& outMsg)
		{
			Assert(inMsg->type() == MsgCode::PrePrepare);
			Assert(inMsg->senderId() != repInfo.myId()); 

			// header size
			if (inMsg->size() < sizeof(PrePrepareMsgHeader)) return false;

			// sender
			if (!repInfo.isIdOfReplica(inMsg->senderId())) return false;
			// NB: the actual expected sender is verified outside this class (becuase in some cases, during view-change protocol,  this message may sent by a non-primary replica to the primary replica).

			PrePrepareMsg* tmp = (PrePrepareMsg*)inMsg;

			// check seqNum
			if (tmp->b()->seqNum == 0) return false;

			// check flags
			const uint16_t flags = tmp->b()->flags;
			const bool isNull = ((flags & 0x1) == 0);
			const bool isReady = (((flags >> 1) & 0x1) == 1);
			const uint16_t firstPath = ((flags >> 2) & 0x3);
			const uint16_t reservedBits = (flags >> 4);
			if (isNull) return false; // we don't send null requests
			if (!isReady) return false; // not ready
			if (firstPath >= 3) return false; // invalid first path
			if ((tmp->firstPath() == CommitPath::FAST_WITH_THRESHOLD) && (repInfo.cVal() == 0)) return false;
			if (reservedBits != 0) return false;

			// size
			if (tmp->b()->endLocationOfLastRequest > tmp->size()) return false;

			// requests
			if (tmp->b()->numberOfRequests == 0) return false;
			if (tmp->b()->numberOfRequests >= tmp->b()->endLocationOfLastRequest) return false;
			if (!tmp->checkRequests()) return false;

			// digest
                        // name-removed::: 
//			Digest d;
//			const char* requestBuffer = (char*)&(tmp->b()->numberOfRequests);
//			const uint32_t requestSize = (tmp->b()->endLocationOfLastRequest - prePrepareHeaderPrefix);

			uint16_t numOfReq = tmp->b()->numberOfRequests;
			const char* requestBody = tmp->body() + sizeof(PrePrepareMsg::PrePrepareMsgHeader); // (char *)msgBody_ + sizeof(PrePrepareMsg::PrePrepareMsgHeader)
			Digest calDigestOfRequests;
			calDigestOfRequests.makeZero();

			for (int i = 0; i < numOfReq; i++) 
			{
				bftEngine::duanqn::SkipCycleClientReqMsg req((bftEngine::duanqn::ClReqHdrEx*) requestBody);
				const char* requestBuffer = req.requestBuf(); // return body() + sizeof(ClientRequestMsgHeader)
				const uint32_t requestSize = req.requestLength();
				
				Digest d;
				DigestUtil::compute(requestBuffer, requestSize, (char*)&d, sizeof(Digest));
				if (calDigestOfRequests.isZero()) {
					calDigestOfRequests = d;
				} else {
					// Use XOR to combine digest of each ClientRequestMsg
					Digest::XOROfTwoDigests(calDigestOfRequests, d, calDigestOfRequests);
				}
				
				// move on to the requestBuf of the next request
				requestBody = requestBody + req.size();
			}

			Digest::digestOfDigest(calDigestOfRequests, calDigestOfRequests);
			auto concatDigest = concatWithCycleNumbers(calDigestOfRequests, tmp->b()->lastCycleNumber, tmp->b()->currentCycleNumber);
                        
			if (concatDigest != tmp->b()->digestOfRequests) {
				std::cout << "ERROR: ToActualMsgType() - concatDigest != tmp->b()->digestOfRequests" << std::endl;
				std::cout << "concatDigest: "<< std::endl;
				hexDump(concatDigest.content(), sizeof(concatDigest), stdout);
				std::cout << "digestOfRequests: "<< std::endl;
				hexDump(tmp->b()->digestOfRequests.content(), sizeof(tmp->b()->digestOfRequests), stdout);
				return false;
			}

			outMsg = (PrePrepareMsg*)inMsg;

			return true;
		}


		PrePrepareMsg::PrePrepareMsg(ReplicaId sender, ViewNum v, SeqNum s, CommitPath firstPath, bool isNull) :
			MessageBase(sender, MsgCode::PrePrepare, (isNull ? sizeof(PrePrepareMsgHeader) : maxSizeOfPrePrepareMsg()))

		{
			b()->viewNum = v;
			b()->seqNum = s;

			bool ready = isNull; // if null, then message is ready 
			b()->flags = computeFlagsForPrePrepareMsg(isNull, ready, firstPath);

			if (!isNull) // not null
				b()->digestOfRequests.makeZero();
			else // null
				b()->digestOfRequests = nullDigest;

			b()->numberOfRequests = 0;
			b()->endLocationOfLastRequest = sizeof(PrePrepareMsgHeader);
		}


		uint32_t PrePrepareMsg::remainingSizeForRequests() const
		{
			Assert(!isReady());
			Assert(!isNull());
			Assert(b()->endLocationOfLastRequest >= sizeof(PrePrepareMsgHeader));

			return (internalStorageSize() - b()->endLocationOfLastRequest);
		}

		void PrePrepareMsg::addRequest(char* pRequest, uint32_t requestSize)
		{
			// name-removed: new way to compute size
			bftEngine::duanqn::ClReqHdrEx *tmp = (bftEngine::duanqn::ClReqHdrEx *)pRequest;
			const uint32_t reqSize = tmp->oldHeader.requestLength + sizeof(bftEngine::duanqn::ClReqHdrEx);
			Assert(reqSize == requestSize);
			Assert(!isNull());
			Assert(!isReady());
			Assert(remainingSizeForRequests() >= requestSize);

			char* insertPtr = body() + b()->endLocationOfLastRequest;

			memcpy(insertPtr, pRequest, requestSize);

			b()->endLocationOfLastRequest = b()->endLocationOfLastRequest + requestSize;
			b()->numberOfRequests = b()->numberOfRequests + 1;
		}



		void PrePrepareMsg::finishAddingRequests()
		{
			Assert(!isNull());
			Assert(!isReady());
			Assert(b()->numberOfRequests > 0);
			Assert(b()->endLocationOfLastRequest > sizeof(PrePrepareMsgHeader));
			Assert(b()->digestOfRequests.isZero());

			// check requests (for debug - consider to remove)
			Assert(checkRequests());

			// mark as ready
			b()->flags |= 0x2;
			Assert(isReady());

			uint16_t numOfReq = b()->numberOfRequests;
			const char* requestBody = body() + sizeof(PrePrepareMsg::PrePrepareMsgHeader); // (char *)msgBody_ + sizeof(PrePrepareMsg::PrePrepareMsgHeader)

			Digest accDigest;
			accDigest.makeZero();
			for (int i = 0; i < numOfReq; i++) 
			{
				bftEngine::duanqn::SkipCycleClientReqMsg req((bftEngine::duanqn::ClReqHdrEx*) requestBody);
				const char* requestBuffer = req.requestBuf(); // return body() + sizeof(ClientRequestMsgHeader), the place of request only
				const uint32_t requestSize = req.requestLength(); // size of request only
				
				Digest d;
				DigestUtil::compute(requestBuffer, requestSize, (char*)&d, sizeof(Digest));
				if (accDigest.isZero()) {
					accDigest = d;
				} else {
					// Use XOR to combine digest of each ClientRequestMsg
					Digest::XOROfTwoDigests(accDigest, d, accDigest);
				}
				
				// move on to the requestBuf of the next request
				requestBody = requestBody + req.size(); // add the size of an entire ClientRequestMsg
			}

			Digest::digestOfDigest(accDigest, accDigest);
			b()->digestOfRequests = concatWithCycleNumbers(accDigest, b()->lastCycleNumber, b()->currentCycleNumber);

			printf("---------------- num of requests = %d total size: %d\n", b()->numberOfRequests, b()->endLocationOfLastRequest);
			// size
			setMsgSize(b()->endLocationOfLastRequest);
			shrinkToFit();
		}

		void PrePrepareMsg::setCurrentCycleNumber(uint32_t cycleNumber) {
			b()->currentCycleNumber = cycleNumber;
		}

		void PrePrepareMsg::setLastCycleNumber(uint32_t cycleNumber) {
			b()->lastCycleNumber = cycleNumber;
		}


		CommitPath PrePrepareMsg::firstPath() const
		{
			const uint16_t firstPathNum = ((b()->flags >> 2) & 0x3);
			Assert(firstPathNum <= 2);
			CommitPath retVal = (CommitPath)firstPathNum; // TODO(GG): check
			return retVal;
		}


		void PrePrepareMsg::updateView(ViewNum v, CommitPath firstPath)
		{
			b()->viewNum = v;
			b()->flags = computeFlagsForPrePrepareMsg(isNull(), isReady(), firstPath);
		}



		int16_t PrePrepareMsg::computeFlagsForPrePrepareMsg(bool isNull, bool isReady, CommitPath firstPath)
		{
			int16_t retVal = 0;

			Assert(!isNull || isReady); // isNull --> isReady

			int16_t firstPathNum = (int16_t)firstPath;
			Assert(firstPathNum <= 2);

			retVal |= (firstPathNum << 2);
			retVal |= ((isReady ? 1 : 0) << 1);
			retVal |= (isNull ? 0 : 1);

			return retVal;
		}

		bool PrePrepareMsg::checkRequests()
		{
			uint16_t remainReqs = b()->numberOfRequests;

			if (remainReqs == 0)
				return (b()->endLocationOfLastRequest == sizeof(PrePrepareMsgHeader));

			uint32_t i = sizeof(PrePrepareMsgHeader);

			if (i >= b()->endLocationOfLastRequest) return false;

			while (true)
			{
				const char* req = body() + i;
				// name-removed: new way to compute size
				bftEngine::duanqn::ClReqHdrEx *tmp = (bftEngine::duanqn::ClReqHdrEx *)req;
				const uint32_t reqSize = tmp->oldHeader.requestLength + sizeof(bftEngine::duanqn::ClReqHdrEx);

				remainReqs--;
				i += reqSize;

				if (remainReqs > 0)
				{
					if (i >= b()->endLocationOfLastRequest) return false;
				}
				else
				{
					return (i == b()->endLocationOfLastRequest);
				}
			}

			Assert(false);
			return false;
		}
                
                Digest PrePrepareMsg::recomputeDigest() {
                    Digest d;
                    const char* requestBuffer = (char*)&(b()->numberOfRequests);
                    const uint32_t requestSize = (b()->endLocationOfLastRequest - prePrepareHeaderPrefix);
                    DigestUtil::compute(requestBuffer, requestSize, (char*)&d, sizeof(Digest));

                    return d;
                }


		///////////////////////////////////////////////////////////////////////////////
		// RequestsIterator
		///////////////////////////////////////////////////////////////////////////////

		RequestsIterator::RequestsIterator(const PrePrepareMsg* const m)
			: msg{ m }, currLoc{ sizeof(PrePrepareMsg::PrePrepareMsgHeader) }
		{
			Assert(msg->isReady());
		}

		void RequestsIterator::restart()
		{
			currLoc = sizeof(PrePrepareMsg::PrePrepareMsgHeader);
		}

		bool RequestsIterator::getCurrent(char*& pRequest) const
		{
			if (end()) return false;

			char* p = msg->body() + currLoc;
			pRequest = p;

			return true;
		}

		bool RequestsIterator::end() const
		{
			Assert(currLoc <= msg->b()->endLocationOfLastRequest);

			return (currLoc == msg->b()->endLocationOfLastRequest);
		}

		void RequestsIterator::gotoNext()
		{
			Assert(!end());
			char* p = msg->body() + currLoc;
			uint32_t size = ((bftEngine::duanqn::ClReqHdrEx*)p)->oldHeader.requestLength + sizeof(bftEngine::duanqn::ClReqHdrEx);
			currLoc += size;
			Assert(currLoc <= msg->b()->endLocationOfLastRequest);
		}

		bool RequestsIterator::getAndGoToNext(char*& pRequest)
		{
			bool atEnd = !getCurrent(pRequest);

			if (atEnd) return false;

			gotoNext();

			return true;
		}

	}
}
