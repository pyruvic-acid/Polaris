//Concord
//
//Copyright (c) 2018 VMware, Inc. All Rights Reserved.
//
//This product is licensed to you under the Apache 2.0 license (the "License").  You may not use this product except in compliance with the Apache 2.0 License. 
//
//This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.

#pragma once

// TODO(GG): this class/file should be replaced by an instance of CollectorOfThresholdSignatures

#include <set>

#include "PrimitiveTypes.hpp"
#include "Digest.hpp"
#include "TimeUtils.hpp"
#include "SkipCycleMsg.h"
//#include "VectorOfShares.h"

class IThresholdVerifier;
class IThresholdAccumulator;
class VectorOfShares;
//using BLS::Relic::G1T;

namespace bftEngine
{
	namespace impl
	{

		class InternalReplicaApi;

		class PartialCommitProofMsg;
		class FullCommitProofMsg;

		class PartialProofsSet
		{
		public:
			PartialProofsSet(InternalReplicaApi* const rep);
			~PartialProofsSet();

			void addSelfMsgAndPPDigest(PartialCommitProofMsg* m, Digest40& digest);

			void setTimeOfSelfPartialProof(const Time& t);

			bool addMsg(PartialCommitProofMsg* m);

			bool addMsg(FullCommitProofMsg* m);

			PartialCommitProofMsg* getSelfPartialCommitProof();

			bool hasFullProof();

			FullCommitProofMsg* getFullProof();

			Time getTimeOfSelfPartialProof();

			bool hasPartialProofFromReplica(ReplicaId repId) const;

			void resetAndFree();
                        //using
                        std::vector<std::string> getValidShares() ;
                        
                        VectorOfShares getValidSharesBits() ;
                        
			Digest40 getExpectedDigest();

		protected:

			void addImp(PartialCommitProofMsg* m, CommitPath cPath);

			IThresholdVerifier* thresholdVerifier(CommitPath cPath);
			IThresholdAccumulator* thresholdAccumulator(CommitPath cPath);

			void tryToCreateFullProof();

			InternalReplicaApi* const replica;

			const size_t numOfRquiredPartialProofsForFast;
			const size_t numOfRquiredPartialProofsForOptimisticFast;

			SeqNum seqNumber;
			FullCommitProofMsg* fullCommitProof;
			PartialCommitProofMsg* selfPartialCommitProof;
			std::set<ReplicaId> participatingReplicasInFast; // not including the current replica
			std::set<ReplicaId> participatingReplicasInOptimisticFast; // not including the current replica
			Digest40 expectedDigest;
			Time timeOfSelfPartialProof;
			IThresholdAccumulator* thresholdAccumulatorForFast;
			IThresholdAccumulator* thresholdAccumulatorForOptimisticFast;
		};

	}
}
