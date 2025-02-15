//Concord
//
//Copyright (c) 2018 VMware, Inc. All Rights Reserved.
//
//This product is licensed to you under the Apache 2.0 license (the "License").  You may not use this product except in compliance with the Apache 2.0 License. 
//
//This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.

#pragma once

#include <stdint.h>

#include "PrimitiveTypes.hpp"
#include "assertUtils.hpp"
#include "Digest.hpp"
#include "MessageBase.hpp"
#include "SkipCycleMsg.h"

namespace bftEngine
{
	namespace impl
	{


		class RequestsIterator;

		class PrePrepareMsg : public MessageBase
		{

		public:
#pragma pack(push,1)
			struct PrePrepareMsgHeader
			{
				MessageBase::Header header;
				ViewNum viewNum;
				SeqNum  seqNum;

				// name-removed: Add 2 fields to keep track of Polaris cycle numbers
				uint32_t lastCycleNumber;
				uint32_t currentCycleNumber;

				uint16_t flags;
				Digest40  digestOfRequests;

				uint16_t numberOfRequests;
				uint32_t endLocationOfLastRequest;

				// bits in flags
				// bit 0: 0=null , 1=non-null 
				// bit 1: 0=not ready , 1=ready 
				// bits 2-3: represent the first commit path that should be tried (00 = OPTIMISTIC_FAST, 01 = FAST_WITH_THRESHOLD, 10 = SLOW)
				// bits 4-15: zero
			};
#pragma pack(pop)
			// name-removed: Add 8 bytes to the header (2 uint32 fields)
			static_assert(sizeof(PrePrepareMsgHeader) == (2 + 8 + 8 + 4 + 4 + 2 + sizeof(PrePrepareMsgHeader::digestOfRequests) + 2 + 4), "PrePrepareMsgHeader is 66B");

			static const size_t prePrepareHeaderPrefix = sizeof(PrePrepareMsgHeader) - sizeof(PrePrepareMsgHeader::numberOfRequests) - sizeof(PrePrepareMsgHeader::endLocationOfLastRequest);

//		public:

			// static

			static MsgSize maxSizeOfPrePrepareMsg();

			static MsgSize maxSizeOfPrePrepareMsgInLocalBuffer();

			static PrePrepareMsg* createNullPrePrepareMsg(ReplicaId sender, ViewNum v, SeqNum s, CommitPath firstPath = CommitPath::SLOW); // TODO(GG): why static method ?

			static const Digest40& digestOfNullPrePrepareMsg();

			static bool ToActualMsgType(const ReplicasInfo& repInfo, MessageBase* inMsg, PrePrepareMsg*& outMsg);

			// ctor and other build methods

			PrePrepareMsg(ReplicaId sender, ViewNum v, SeqNum s, CommitPath firstPath, bool isNull = false);

			uint32_t remainingSizeForRequests() const;

			void addRequest(char* pRequest, uint32_t requestSize);

			void finishAddingRequests();

			uint32_t getCurrentCycleNumber(){
				return b()->currentCycleNumber;
			}

			uint32_t getLastCycleNumber(){
				return b()->lastCycleNumber;
			}

			void setCurrentCycleNumber(uint32_t cycleNumber);

			void setLastCycleNumber(uint32_t cycleNumber);

			// getter methods 

			ViewNum viewNumber() const { return b()->viewNum; }

			SeqNum seqNumber() const { return b()->seqNum; }

			CommitPath firstPath() const;

			bool isNull() const { return ((b()->flags & 0x1) == 0); }

			Digest40& digestOfRequests() const { return b()->digestOfRequests; }

			uint16_t numberOfRequests() const { return b()->numberOfRequests; }

			// update view and first path 

			void updateView(ViewNum v, CommitPath firstPath = CommitPath::SLOW);
                        
						// name-removed: Need to rewrite
                        Digest recomputeDigest();
		protected:

			static int16_t computeFlagsForPrePrepareMsg(bool isNull, bool isReady, CommitPath firstPath);

			bool isReady() const { return (((b()->flags >> 1) & 0x1) == 1); }

			bool checkRequests();

			PrePrepareMsgHeader* b() const
			{
				return (PrePrepareMsgHeader*)msgBody_;
			}

			friend class RequestsIterator;
		};

		class RequestsIterator
		{
		public:
			RequestsIterator(const PrePrepareMsg* const m);

			void restart();

			bool getCurrent(char*& pRequest) const;

			bool end() const;

			void gotoNext();

			bool getAndGoToNext(char*& pRequest);
		protected:
			const PrePrepareMsg* const msg;
			uint32_t currLoc;
		};

	}
}
