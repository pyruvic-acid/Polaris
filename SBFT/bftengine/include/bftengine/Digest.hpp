//Concord
//
//Copyright (c) 2018 VMware, Inc. All Rights Reserved.
//
//This product is licensed to you under the Apache 2.0 license (the "License").  You may not use this product except in compliance with the Apache 2.0 License. 
//
//This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.

#pragma once

#include <memory.h>
#include <stdint.h>
#include <string>
#include "DigestType.h"
#include <cstdio>

namespace bftEngine
{
	namespace duanqn{
		const int SkipCycleDigestSize = (DIGEST_SIZE + sizeof(uint32_t) * 2);
	}

	namespace impl
	{
		class Digest
		{
		public:

			Digest() { memset(d, 0, DIGEST_SIZE); }

			Digest(unsigned char initVal) { memset(d, initVal, DIGEST_SIZE); }

			Digest(char* buf, size_t len);

			Digest(const Digest& other) { memcpy(d, other.d, DIGEST_SIZE); }
                        
			Digest(char* digest) { memcpy(d, digest, DIGEST_SIZE);}
  
			bool isZero() const
			{
				for (int i = 0; i < DIGEST_SIZE; i++)
				{
					if (d[i] != 0) return false;
				}
				return true;
			}

			bool operator==(const Digest& other) const
			{
				int r = memcmp(d, other.d, DIGEST_SIZE);
				return (r == 0);
			}

			bool operator!=(const Digest& other) const
			{
				int r = memcmp(d, other.d, DIGEST_SIZE);
				return (r != 0);
			}

			Digest& operator=(const Digest& other)
			{
				memcpy(d, other.d, DIGEST_SIZE);
				return *this;
			}

			int hash() const
			{
				uint64_t* p = (uint64_t*)d;
				int h = (int)p[0];
				return h;
			}

			void makeZero() { memset(d, 0, DIGEST_SIZE); }

			char* content() const { return (char*)d; }

			std::string toString() const;

			void print();


			static void calcCombination(const Digest& inDigest, int64_t inDataA, int64_t inDataB, Digest& outDigest) // TODO(GG): consider to change this function (TBD - check security)
			{
				const size_t X = ((DIGEST_SIZE / sizeof(uint64_t)) / 2);

				memcpy(outDigest.d, inDigest.d, DIGEST_SIZE);

				uint64_t* ptr = (uint64_t*)outDigest.d;
				size_t locationA = ptr[0] % X;
				size_t locationB = (ptr[0] >> 8) % X;
				ptr[locationA] = ptr[locationA] ^ (inDataA);
				ptr[locationB] = ptr[locationB] ^ (inDataB);
			}

			static void digestOfDigest(const Digest& inDigest, Digest& outDigest);

			static void XOROfTwoDigests(const Digest& inDigest1, const Digest& inDigest2, Digest& outDigest);

			void output(FILE *f){
				for(int i = 0; i < DIGEST_SIZE; ++i){
					fprintf(f, "%02hhx", d[i]);
				}
			}

		protected:

			char d[DIGEST_SIZE]; // DIGEST_SIZE should be >= 8 bytes

		};
		
		static_assert(DIGEST_SIZE >= sizeof(uint64_t), "DIGEST_SIZE should be >= sizeof(uint64_t)");
		static_assert(sizeof(Digest) == DIGEST_SIZE, "sizeof(Digest) != DIGEST_SIZE");

		class Digest40
		{
		private:
			const static int SIZE40 = DIGEST_SIZE + 2 * sizeof(uint32_t);
		public:

			Digest40() { memset(d, 0, SIZE40); }

			Digest40(unsigned char initVal) { memset(d, initVal, SIZE40); }

			Digest40(const Digest40& other) { memcpy(d, other.d, SIZE40); }
                        
			Digest40(char* digest) { memcpy(d, digest, SIZE40);}

			void makeZero() { memset(d, 0, SIZE40); }

			char* content() const { return (char*)d; }

			bool isZero() const
			{
				for (int i = 0; i < SIZE40; i++)
				{
					if (d[i] != 0) return false;
				}
				return true;
			}

			bool operator==(const Digest40& other) const
			{
				int r = memcmp(d, other.d, SIZE40);
				return (r == 0);
			}

			bool operator!=(const Digest40& other) const
			{
				int r = memcmp(d, other.d, SIZE40);
				return (r != 0);
			}

			int hash() const
			{
				uint64_t* p = (uint64_t*)(d + sizeof(uint32_t) * 2);
				int h = (int)p[0];
				return h;
			}

			uint32_t getLastCycleNumber(){
				uint32_t *ptr = (uint32_t *)d;
				return *ptr;
			}

			uint32_t getCurrentCycleNumber(){
				uint32_t *ptr = (uint32_t *)(d + sizeof(uint32_t));
				return *ptr;
			}

			static void calcCombination(const Digest40& inDigest, int64_t inDataA, int64_t inDataB, Digest40& outDigest) // TODO(GG): consider to change this function (TBD - check security)
			{
				const size_t X = ((DIGEST_SIZE / sizeof(uint64_t)) / 2);

				memcpy(outDigest.d, inDigest.d, SIZE40);

				// name-removed: Protect the cycle numbers! They are the first 8 bytes.
				uint64_t* ptr = (uint64_t*)(outDigest.d + 2 * sizeof(uint32_t));
				size_t locationA = ptr[0] % X;
				size_t locationB = (ptr[0] >> 8) % X;
				ptr[locationA] = ptr[locationA] ^ (inDataA);
				ptr[locationB] = ptr[locationB] ^ (inDataB);
			}

			void output(FILE *f){
				for(int i = 0; i < SIZE40; ++i){
					fprintf(f, "%02hhx", d[i]);
				}
			}

			static void digestOfDigest(const Digest40& inDigest, Digest40& outDigest);

		protected:

			char d[SIZE40]; // DIGEST_SIZE should be >= 8 bytes

		};

		Digest40 concatWithCycleNumbers(const Digest& inDigest, uint32_t lastCycleNumber, uint32_t currentCycleNumber);
	}
}
