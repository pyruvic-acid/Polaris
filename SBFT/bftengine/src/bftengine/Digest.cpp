//Concord
//
//Copyright (c) 2018 VMware, Inc. All Rights Reserved.
//
//This product is licensed to you under the Apache 2.0 license (the "License").  You may not use this product except in compliance with the Apache 2.0 License. 
//
//This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.

#include "Digest.hpp"



#include <string.h>
#include <stdio.h>
#include "Crypto.hpp"


namespace bftEngine
{
	namespace impl
	{
		
		Digest::Digest(char* buf, size_t len) {
			DigestUtil::compute(buf, len, (char*)d, sizeof(Digest));
		}

		
		std::string Digest::toString() const
		{
			char c[DIGEST_SIZE * 2];
			char t[3];
			for (size_t i = 0; i < DIGEST_SIZE; i++)
			{
				unsigned int b = (unsigned int)d[i];
				snprintf(t, 3, "%02X", b);
				c[i * 2] = t[0];
				c[i * 2 + 1] = t[1];
			}

			std::string ret(c, DIGEST_SIZE * 2);

			return ret;
		}

		
		void Digest::print()
		{
			printf("digest=[%s]", toString().c_str());
		}

		
		void Digest::digestOfDigest(const Digest& inDigest, Digest& outDigest)
		{
			DigestUtil::compute(inDigest.d, sizeof(Digest), outDigest.d, sizeof(Digest));
		}

		void Digest40::digestOfDigest(const Digest40& inDigest, Digest40& outDigest)
		{
			DigestUtil::compute(inDigest.d, sizeof(Digest40), outDigest.d, sizeof(Digest40));
		}

		
		void Digest::XOROfTwoDigests(const Digest& inDigest1, const Digest& inDigest2, Digest& outDigest) 
		{
			memcpy(outDigest.d, inDigest1.d, DIGEST_SIZE);
			for (size_t i = 0; i < DIGEST_SIZE; i++)
			{
				outDigest.d[i] = inDigest2.d[i] ^ outDigest.d[i];
			}
		}

		Digest40 concatWithCycleNumbers(const Digest& inDigest, uint32_t lastCycleNumber, uint32_t currentCycleNumber){
			Digest40 res;
			char *data = res.content();
			char *indata = inDigest.content();
			memcpy(data, &lastCycleNumber, sizeof(lastCycleNumber));
			memcpy(data + sizeof(lastCycleNumber), &currentCycleNumber, sizeof(currentCycleNumber));
			memcpy(data + sizeof(lastCycleNumber) + sizeof(currentCycleNumber), indata, DIGEST_SIZE);
			return res;
		}
	}
}
