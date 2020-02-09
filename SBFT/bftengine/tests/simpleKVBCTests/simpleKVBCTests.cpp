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

#include <iostream>
#include <cstdio>

#include "simpleKVBCTests.h"
#include "SkipCycleMsg.h"
#include "PolarisKey.h"
//#include "Crypto.hpp"

#include <inttypes.h>
#include <map>
#include <set>
#include <list>
#include <chrono>
#ifndef _WIN32
#include <unistd.h>
#endif

using std::list;
using std::map;
using std::set;
#ifdef BAZEL_BUILD
#include "polaris.grpc.pb.h"
#else
#include "polaris.grpc.pb.h"
#include "DatabaseInterface.h"
#endif

//using grpc::Server;
//using grpc::ServerBuilder;
//using grpc::ServerContext;
//using grpc::Status;
using rpc::SLRequestMessage;
using rpc::LeaderIdReply;
//using rpc::SLRequestContent;
//using rpc::TxnHeaderBlock;
using rpc::TransactionHeader;
//using rpc::TransactionContent;
using rpc::Polaris;

#define KV_LEN (2048)
#define NUMBER_OF_KEYS (200) 
#define CONFLICT_DISTANCE (49)
#define MAX_WRITES_IN_REQ (7)
#define MAX_READ_SET_SIZE_IN_REQ (10)
#define MAX_READS_IN_REQ (7)

using namespace SimpleKVBC;

#define CHECKMSG(_cond_, _msg_)  if (!(_cond_)) { \
      printf("\nTest failed: %s\n", _msg_); \
      assert(_cond_); \
}

void bin2hex(const void * bin, int binLen, char * hexBuf, int hexBufCapacity) {
    int needed = binLen * 2 + 1;
    static const char hex[] = "0123456789abcdef";

    if (hexBufCapacity < needed) {
        throw std::runtime_error("bin2hex not enough capacity for hexbuf");
    }

    const unsigned char * bytes = reinterpret_cast<const unsigned char *> (bin);
    for (int i = 0; i < binLen; i++) {
        unsigned char c = bytes[i];
        hexBuf[2 * i] = hex[c >> 4]; // translate the upper 4 bits
        hexBuf[2 * i + 1] = hex[c & 0xf]; // translate the lower 4 bits
    }
    hexBuf[2 * binLen] = '\0';
}

namespace BasicRandomTests {
    namespace Internal {
#pragma pack(push,1)
        struct SimpleKV {
            char key[KV_LEN];
            char val[KV_LEN];
        };

        struct SimpleKey {
            char key[KV_LEN];
        };

        struct SimpleVal {
            char v[KV_LEN];
        };

        struct SimpleBlock {
            BlockId id;
            size_t numberOfItems;
            SimpleKV items[1];

            static SimpleBlock* alloc(size_t items) {
                size_t size = sizeof (SimpleBlock) + sizeof (SimpleKV) * (items - 1);
                char* pBuf = new char[size];
                memset(pBuf, 0, size);
                return (SimpleBlock*) (pBuf);
            }

            static void free(SimpleBlock* p) {
                char* p1 = (char*) p;
                delete[] p1;
            }

            //static void print(SimpleBlock* block)
            //{
            //	printf("\nBlockId=%" PRId64 " Items=%zu", block->id, block->numberOfItems);
            //	for (size_t i = 0; i < block->numberOfItems; i++)
            //	{
            //		printf("\n");
            //		printf("Block id %" PRId64 " item %3zu key=", block->id, i);
            //		for (int k = 0; k < KV_LEN; k++) printf("%02X", block->items[i].key[k]);
            //		printf("   val=");
            //		for (int k = 0; k < KV_LEN; k++) printf("%02X", block->items[i].val[k]);
            //	}
            //}
        };

        struct SimpleRequestHeader {
            char type; // 1 == conditional write , 2 == read, 3 == get last block

            static void free(SimpleRequestHeader* p) {
                char* p1 = (char*) p;
                delete[] p1;
            }


        };

        struct TxWriteHeaderBatch {
            SimpleRequestHeader h;
            size_t sizeOfBlock;
            //            char* block;

            static TxWriteHeaderBatch* alloc(size_t sizeOfBlock) {
                size_t s = size(sizeOfBlock);
                char* pBuf = new char[s];
                memset(pBuf, 0, s);
                return (TxWriteHeaderBatch*) (pBuf);
            }

            static void free(TxWriteHeaderBatch* p) {
                char* p1 = (char*) p;
                delete[] p1;
            }

            static size_t size(size_t sizeOfBlock) {
                // total size = size of the header + #TxKV*writes i.e. the TxBlock
                return sizeof (TxWriteHeaderBatch) + sizeOfBlock;
            }

            size_t size() {
                return size(sizeOfBlock);
            }

            char* content() {
                return (char*) (((char*) this) + sizeof (TxWriteHeaderBatch));
            }

        };

        struct SimpleConditionalWriteHeader {
            SimpleRequestHeader h; // TODO: this is ugly ....
            BlockId readVersion;
            size_t numberOfKeysInReadSet;
            size_t numberOfWrites;
            // followed by SimpleKey[numberOfKeysInReadSet]
            // followed by SimpleKV[numberOfWrites]

            static SimpleConditionalWriteHeader* alloc(size_t numOfKeysInReadSet, size_t numOfWrites) {
                size_t s = size(numOfKeysInReadSet, numOfWrites);
                char* pBuf = new char[s];
                memset(pBuf, 0, s);
                return (SimpleConditionalWriteHeader*) (pBuf);
            }

            static void free(SimpleConditionalWriteHeader* p) {
                char* p1 = (char*) p;
                delete[] p1;
            }

            static size_t size(size_t numOfKeysInReadSet, size_t numOfWrites) {
                return sizeof (SimpleConditionalWriteHeader) + numOfKeysInReadSet * sizeof (SimpleKey) + numOfWrites * sizeof (SimpleKV);
            }

            size_t size() {
                return size(numberOfKeysInReadSet, numberOfWrites);
            }

            SimpleKey* readSetArray() {
                return (SimpleKey*) (((char*) this) + sizeof (SimpleConditionalWriteHeader));
            }

            SimpleKV* keyValArray() {
                return (SimpleKV*) (((char*) this) + sizeof (SimpleConditionalWriteHeader) + numberOfKeysInReadSet * sizeof (SimpleKey));
            }
        };

        struct SimpleReadHeader {
            SimpleRequestHeader h;
            BlockId readVersion; // if 0, then read from the latest version
            size_t numberOfKeysToRead;
            SimpleKey keys[1];

            static SimpleReadHeader* alloc(size_t numOfKeysToRead) {
                size_t size = sizeof (SimpleReadHeader) + (sizeof (SimpleKey) * (numOfKeysToRead - 1));
                char* pBuf = new char[size];
                memset(pBuf, 0, size);
                return (SimpleReadHeader*) (pBuf);
            }

            static void free(SimpleReadHeader* p) {
                char* p1 = (char*) p;
                delete[] p1;
            }

            static size_t size(size_t numOfKeysToRead) {
                return sizeof (SimpleReadHeader) + (numOfKeysToRead - 1) * sizeof (SimpleKey);
            }

            size_t size() {
                return size(numberOfKeysToRead);
            }

            SimpleKey* keysArray() {
                return ((SimpleKey*) ((char*) keys));
            }


        };

        struct SimpleGetLastBlockHeader {

            static SimpleGetLastBlockHeader* alloc() {
                size_t size = sizeof (SimpleGetLastBlockHeader);
                char* pBuf = new char[size];
                memset(pBuf, 0, size);
                return (SimpleGetLastBlockHeader*) (pBuf);
            }

            static void free(SimpleGetLastBlockHeader* p) {
                char* p1 = (char*) p;
                delete[] p1;
            }

            static size_t size() {
                return sizeof (SimpleGetLastBlockHeader);
            }

            SimpleRequestHeader h;
        };

        struct SimpleReplyHeader {
            char type; // 1 == conditional write , 2 == read, 3 == get last block

            static void free(SimpleReplyHeader* p) {
                char* p1 = (char*) p;
                delete[] p1;
            }

        };

        struct SimpleReplyHeader_ConditionalWrite {

            static SimpleReplyHeader_ConditionalWrite* alloc() {
                size_t s = sizeof (SimpleReplyHeader_ConditionalWrite);
                char* pBuf = new char[s];
                memset(pBuf, 0, s);
                return (SimpleReplyHeader_ConditionalWrite*) (pBuf);
            }

            static void free(SimpleReplyHeader_ConditionalWrite* p) {
                char* p1 = (char*) p;
                delete[] p1;
            }


            SimpleReplyHeader h;
            bool succ;
            BlockId latestBlock;
            char signature[128];
            size_t sigLen;
        };

        struct SimpleReplyHeader_Read {
            SimpleReplyHeader h;
            size_t numberOfElements;
            SimpleKV elements[1];

            static size_t size(size_t numOfElements) {
                size_t size = sizeof (SimpleReplyHeader_Read) + (sizeof (SimpleKV) * (numOfElements - 1));
                return size;
            }

            size_t size() {
                return size(numberOfElements);
            }

            static SimpleReplyHeader_Read* alloc(size_t numOfElements) {
                size_t size = sizeof (SimpleReplyHeader_Read) + (sizeof (SimpleKV) * (numOfElements - 1));
                char* pBuf = new char[size];
                memset(pBuf, 0, size);
                return (SimpleReplyHeader_Read*) (pBuf);
            }

            static void free(SimpleReplyHeader_Read* p) {
                char* p1 = (char*) p;
                delete[] p1;
            }



        };

        struct SimpleReplyHeader_GetLastBlockHeader {

            static SimpleReplyHeader_GetLastBlockHeader* alloc() {
                size_t s = sizeof (SimpleReplyHeader_GetLastBlockHeader);
                char* pBuf = new char[s];
                memset(pBuf, 0, s);
                return (SimpleReplyHeader_GetLastBlockHeader*) (pBuf);
            }

            static void free(SimpleReplyHeader_GetLastBlockHeader* p) {
                char* p1 = (char*) p;
                delete[] p1;
            }



            SimpleReplyHeader h;
            BlockId latestBlock;
        };


        //static void print(SimpleRequestHeader* r)
        //{
        //	
        //	if (r->type == 1)
        //	{
        //	}
        //	else if (r->type == 2)
        //	{
        //		
        //		SimpleReadHeader* p = (SimpleReadHeader*)r;
        //		printf("\n");
        //		printf("Read: version=%" PRId64 " numberOfKeysToRead=%zu keys=", p->readVersion, p->numberOfKeysToRead);
        //		for (size_t i = 0; i < p->numberOfKeysToRead; i++)
        //		{
        //			printf("%4s", " ");
        //			for (int k = 0; k < KV_LEN; k++) printf("%02X", p->keys[i].key[k]);
        //		}
        //		
        //	}
        //	else if (r->type == 3)
        //	{

        //	}
        //	else
        //	{
        //		assert(0);
        //	}			
        //}


        //static void print(SimpleReplyHeader* r)
        //{			
        //	if (r->type == 1)
        //	{
        //	}
        //	else if (r->type == 2)
        //	{
        //		
        //		SimpleReplyHeader_Read* p = (SimpleReplyHeader_Read*)r;
        //		printf("\n");
        //		printf("Read reply: numOfelements=%zu", p->numberOfElements);
        //		for (size_t i = 0; i < p->numberOfElements; i++)
        //		{
        //			printf("%4s", " ");
        //			printf("< ");
        //			for (int k = 0; k < KV_LEN; k++) printf("%02X", p->elements[i].key[k]);
        //			printf(" ; ");
        //			for (int k = 0; k < KV_LEN; k++) printf("%02X", p->elements[i].val[k]);
        //			printf(" >");
        //		}
        //		
        //	}
        //	else if (r->type == 3)
        //	{

        //	}
        //	else
        //	{
        //		assert(0);
        //	}			
        //}

        // internal types

        class SimpleKIDPair // represents <key,blockId>
        {
        public:
            const SimpleKey key;
            const BlockId blockId;

            SimpleKIDPair(SimpleKey s, BlockId i) : key(s), blockId(i) {
            }

            bool operator<(const SimpleKIDPair& k) const {
                int c = memcmp((char*) & this->key, (char*) &k.key, sizeof (SimpleKey));
                if (c == 0)
                    return this->blockId > k.blockId;
                else
                    return c < 0;
            }

            bool operator==(const SimpleKIDPair& k) const {
                if (this->blockId != k.blockId) return false;
                int c = memcmp((char*) & this->key, (char*) &k.key, sizeof (SimpleKey));
                return (c == 0);
            }
        };

        struct TxReadHeaderBatch {
            SimpleRequestHeader h;
            BlockId readVersion; // if 0, then read from the latest version
            size_t sizeOfBlock;

            static TxReadHeaderBatch* alloc(size_t sizeOfBlock) {
                size_t s = size(sizeOfBlock);
                char* pBuf = new char[s];
                memset(pBuf, 0, s);
                return (TxReadHeaderBatch*) (pBuf);
            }

            static void free(TxReadHeaderBatch* p) {
                char* p1 = (char*) p;
                delete[] p1;
            }

            static size_t size(size_t sizeOfBlock) {
                return sizeof (TxReadHeaderBatch) + sizeOfBlock;
            }

            size_t size() {
                return size(sizeOfBlock);
            }

            char* content() {
                return ((char*) ((char*) this) + sizeof (TxReadHeaderBatch));
            }


        };

        struct TxReplyHeader_Read {
            SimpleReplyHeader h;
            size_t sizeOfBlock;

            static size_t size(size_t sizeOfBlock) {
                size_t size = sizeof (TxReplyHeader_Read) + sizeOfBlock;
                return size;
            }

            size_t size() {
                return size(sizeOfBlock);
            }

            static TxReplyHeader_Read* alloc(size_t sizeOfBlock) {
                size_t size = sizeof (TxReplyHeader_Read) + sizeOfBlock;
                char* pBuf = new char[size];
                memset(pBuf, 0, size);
                return (TxReplyHeader_Read*) (pBuf);
            }

            static void free(TxReplyHeader_Read* p) {
                char* p1 = (char*) p;
                delete[] p1;
            }

            char* content() {
                return ((char*) ((char*) this) + sizeof (TxReplyHeader_Read));
            }

        };
#pragma pack(pop)

        class InternalTestsBuilder {
        private:

            friend void BasicRandomTests::run(IClient* client, const size_t numOfOperations);

            static void createRandomTest(size_t numOfRequests, size_t seed,
                    int64_t testPrefix,
                    list<SimpleRequestHeader*>& outRequests,
                    list<SimpleReplyHeader*>& outReplies) {
                InternalTestsBuilder t(testPrefix);
                t.create(numOfRequests, seed);

                outRequests = t.m_requests;
                outReplies = t.m_replies;

                for (map<BlockId, SimpleBlock*>::iterator it = t.m_internalBlockchain.begin(); it != t.m_internalBlockchain.end(); it++)
                    SimpleBlock::free(it->second);
            }

            static void free(std::list<SimpleRequestHeader*>& outRequests, std::list<SimpleReplyHeader*>& outReplies) {
                for (list<SimpleRequestHeader*>::iterator it = outRequests.begin(); it != outRequests.end(); it++)
                    SimpleRequestHeader::free(*it);

                for (list<SimpleReplyHeader*>::iterator it = outReplies.begin(); it != outReplies.end(); it++)
                    SimpleReplyHeader::free(*it);
            }

            //			const int64_t m_testPrefix; // TODO(GG): can be used to support multi-executions of the test on the same blockchain

            std::list<SimpleRequestHeader*> m_requests;
            std::list<SimpleReplyHeader*> m_replies;

            std::map<BlockId, SimpleBlock*> m_internalBlockchain;
            std::map<SimpleKIDPair, SimpleVal> m_map;

            BlockId m_lastBlockId;

            InternalTestsBuilder(int64_t testPrefix) //: m_testPrefix(testPrefix)
            {
                m_lastBlockId = 0;
            }

            void create(size_t numOfRequests, size_t seed) {
                srand(seed);

                for (size_t i = 0; i < numOfRequests; i++) {
                    int prc = rand() % 100 + 1;
                    if (prc <= 50)
                        createAndInsertRandomRead();
                    else if (prc <= 95)
                        createAndInsertRandomConditionalWrite();
                    else if (prc <= 100)
                        createAndInsertGetLastBlock();
                    else assert(0);
                }

                for (std::map<BlockId, SimpleBlock*>::iterator it = m_internalBlockchain.begin();
                        it != m_internalBlockchain.end(); it++) {
                    BlockId bId = it->first;
                    SimpleBlock* block = it->second;
                    (void) bId;
                    (void) block;

                    assert(bId == block->id);
                }


            }

            void createAndInsertRandomConditionalWrite() {
                // Create request

                BlockId readVer = m_lastBlockId;
                if (m_lastBlockId > CONFLICT_DISTANCE) readVer -= (rand() % CONFLICT_DISTANCE);

                size_t numberOfWrites = (rand() % (MAX_WRITES_IN_REQ - 1)) + 1;
                size_t numberOfKeysInReadSet = (rand() % MAX_READ_SET_SIZE_IN_REQ);

                SimpleConditionalWriteHeader* pHeader = SimpleConditionalWriteHeader::alloc(numberOfKeysInReadSet, numberOfWrites);

                // fill request
                pHeader->h.type = 1;
                pHeader->readVersion = readVer;
                pHeader->numberOfKeysInReadSet = numberOfKeysInReadSet;
                pHeader->numberOfWrites = numberOfWrites;
                SimpleKey* pReadKeysArray = pHeader->readSetArray();
                SimpleKV* pWritesKVArray = pHeader->keyValArray();

                for (size_t i = 0; i < numberOfKeysInReadSet; i++) {
                    size_t k = rand() % NUMBER_OF_KEYS;
                    //memcpy(pReadKeysArray[i].key, &m_testPrefix, sizeof(int64_t));
                    memcpy(pReadKeysArray[i].key /*+ sizeof(int64_t)*/, &k, sizeof (size_t));
                }


                std::set<size_t> usedKeys;
                for (size_t i = 0; i < numberOfWrites; i++) {
                    size_t k = 0;
                    do // avoid duplications 
                    {
                        k = rand() % NUMBER_OF_KEYS;
                    } while (usedKeys.count(k) > 0);
                    usedKeys.insert(k);

                    size_t v = rand();
                    //memcpy(pWritesKVArray[i].key, &m_testPrefix, sizeof(int64_t));
                    //memcpy(pWritesKVArray[i].val, &m_testPrefix, sizeof(int64_t));
                    memcpy(pWritesKVArray[i].key /*+ sizeof(int64_t)*/, &k, sizeof (size_t));
                    memcpy(pWritesKVArray[i].val /*+ sizeof(int64_t)*/, &v, sizeof (size_t));
                }

                // add request to m_requests
                m_requests.push_back((SimpleRequestHeader*) pHeader);

                // look for conflicts
                bool foundConflict = false;
                for (BlockId i = readVer + 1; (i <= m_lastBlockId) && !foundConflict; i++) {
                    SimpleBlock* currBlock = m_internalBlockchain[i];

                    for (size_t a = 0; (a < numberOfKeysInReadSet) && !foundConflict; a++)
                        for (size_t b = 0; (b < currBlock->numberOfItems) && !foundConflict; b++) {
                            if (memcmp(pReadKeysArray[a].key, currBlock->items[b].key, KV_LEN) == 0)
                                foundConflict = true;
                        }
                }

                // add expected reply to m_replies

                SimpleReplyHeader_ConditionalWrite* pReply = SimpleReplyHeader_ConditionalWrite::alloc();
                pReply->h.type = 1;
                if (foundConflict) {
                    pReply->succ = false;
                    pReply->latestBlock = m_lastBlockId;
                } else {
                    pReply->succ = true;
                    pReply->latestBlock = m_lastBlockId + 1;
                }

                m_replies.push_back((SimpleReplyHeader*) pReply);

                // if needed, add new block into the blockchain
                if (!foundConflict) {
                    m_lastBlockId++;

                    const size_t N = pHeader->numberOfWrites;

                    SimpleBlock* pNewBlock = SimpleBlock::alloc(N);

                    pNewBlock->id = m_lastBlockId;
                    pNewBlock->numberOfItems = N;

                    for (size_t i = 0; i < N; i++) {
                        pNewBlock->items[i] = pWritesKVArray[i];

                        SimpleKey sk;
                        memcpy(sk.key, pWritesKVArray[i].key, KV_LEN);

                        SimpleVal sv;
                        memcpy(sv.v, pWritesKVArray[i].val, KV_LEN);

                        SimpleKIDPair kiPair(sk, m_lastBlockId);
                        m_map[kiPair] = sv;
                    }

                    m_internalBlockchain[m_lastBlockId] = pNewBlock;
                }
            }

            void createAndInsertRandomRead() {
                // Create request

                BlockId readVer = (rand() % (m_lastBlockId + 1));
                size_t numberOfReads = (rand() % (MAX_READS_IN_REQ - 1)) + 1;

                SimpleReadHeader* pHeader = SimpleReadHeader::alloc(numberOfReads);

                // fill request
                pHeader->h.type = 2;
                pHeader->readVersion = readVer;
                pHeader->numberOfKeysToRead = numberOfReads;

                for (size_t i = 0; i < numberOfReads; i++) {
                    size_t k = rand() % NUMBER_OF_KEYS;
                    //memcpy(pHeader->keys[i].key, &m_testPrefix, sizeof(int64_t));
                    memcpy(pHeader->keys[i].key /*+ sizeof(int64_t)*/, &k, sizeof (size_t));
                }

                // add request to m_requests
                m_requests.push_back((SimpleRequestHeader*) pHeader);

                // compute expected reply
                SimpleReplyHeader_Read* pReply = SimpleReplyHeader_Read::alloc(numberOfReads);
                pReply->h.type = 2;
                pReply->numberOfElements = numberOfReads;

                for (size_t i = 0; i < numberOfReads; i++) {
                    memcpy(pReply->elements[i].key, pHeader->keys[i].key, KV_LEN);

                    SimpleKey sk = pHeader->keys[i];

                    SimpleKIDPair kiPair(sk, pHeader->readVersion);

                    std::map<SimpleKIDPair, SimpleVal>::const_iterator p = m_map.lower_bound(kiPair);

                    if (p != m_map.end() && (pHeader->readVersion >= p->first.blockId) && (memcmp(p->first.key.key, pHeader->keys[i].key, KV_LEN) == 0)) {
                        memcpy(pReply->elements[i].val, p->second.v, KV_LEN);
                    } else {
                        memset(pReply->elements[i].val, 0, KV_LEN);
                    }
                }

                // add reply to m_replies

                m_replies.push_back((SimpleReplyHeader*) pReply);
            }

            void createAndInsertGetLastBlock() {
                // Create request

                SimpleGetLastBlockHeader* pHeader = SimpleGetLastBlockHeader::alloc();

                // fill request
                pHeader->h.type = 3;

                // add request to m_requests
                m_requests.push_back((SimpleRequestHeader*) pHeader);

                // compute expected reply
                SimpleReplyHeader_GetLastBlockHeader* pReply = SimpleReplyHeader_GetLastBlockHeader::alloc();
                pReply->h.type = 3;
                pReply->latestBlock = m_lastBlockId;

                // add reply to m_replies
                m_replies.push_back((SimpleReplyHeader*) pReply);
            }


        };

        class InternalCommandsHandler : public ICommandsHandler {
        public:

            static size_t simpleHash(const char *data, const size_t len) {
                size_t hash = 5381;
                size_t t;
                for (size_t i = 0; i < len; i++) {
                    t = data[i];
                    hash = ((hash << 5) + hash) + t;
                }
                return hash;
            }

            // probably should pass the reference of command
            virtual bool executeCommand(const Slice command,
                    const ILocalKeyValueStorageReadOnly& roStorage,
                    IBlocksAppender& blockAppender,
                    const size_t maxReplySize,
                    char* outReply, size_t& outReplySize,
                    uint64_t sequenceNum,
                    uint64_t viewNum,
                    bftEngine::impl::Digest digest,
                    const char *thresholdSignature,
                    uint16_t thresholdSignatureLen) const {

                SLRequestMessage r;
                r.ParseFromArray((void *) command.data, command.size);

                
                std::cout << "receive exec senderId: " << r.sender_id() << std::endl;
                std::cout << "receive exec nonce: " << r.nonce() << std::endl;

                // Comment out for debugging purpose 
                bftEngine::impl::Digest digestOfRequestsTmp;
                bftEngine::impl::DigestUtil::compute(command.data, command.size, (char*) &digestOfRequestsTmp, sizeof (bftEngine::impl::Digest));
                uint32_t sigHexSize = thresholdSignatureLen * 2 + 1;
                char * thresholdSignatureHex = (char *) malloc(sigHexSize);
                bin2hex(thresholdSignature, thresholdSignatureLen, thresholdSignatureHex, sigHexSize); //copy outreply to threshsignature

                std::cout << "CommitProof=" << thresholdSignatureHex << " size=" << sigHexSize << std::endl;
                printf("Expected Digest=");
                digest.output(stdout);
                printf("\n");
                printf("DigestOfReq=");
                digestOfRequestsTmp.output(stdout);
                printf("\n");
                std::cout << "SeqNum=" << sequenceNum << " viewNum=" << viewNum << std::endl;

                // Add the entire block, use the hash as its key value;
                BlockId currBlock = roStorage.getLastBlock();
                SetOfKeyValuePairs updates;
                
                //std::cout << "SeqNum=" << sequenceNum << " viewNum=" << viewNum << std::endl;
                PolarisKey* compositeKey = PolarisKey::alloc(sigHexSize);
                //std::cout << "SeqNum=" << sequenceNum << " viewNum=" << viewNum << std::endl;
                compositeKey->seqNum = sequenceNum;
                compositeKey->viewNum = viewNum;
                compositeKey->sizeOfkey = sigHexSize;
                //std::cout << "thresholdSignatureHex=" << thresholdSignatureHex << " sigHexSize=" << sigHexSize << std::endl;
                memcpy(compositeKey->thresholdSignature(), thresholdSignatureHex, sigHexSize);
                //std::cout << "SeqNum=" << sequenceNum << " viewNum=" << viewNum << std::endl;
                
                //somehow store the seq number,viewnumber and digest, and threshold signature
                Slice key((const char*) compositeKey, compositeKey->size());
                Slice value((const char*) command.data, command.size);
                KeyValuePair kv(key, value);
                updates.insert(kv);
                BlockId newBlockId = 0;
                Status addSucc = blockAppender.addBlock(updates, newBlockId);
                assert(addSucc.ok());
                assert(newBlockId == currBlock + 1);
                std::cout << "newBlockId=" << newBlockId << " updates=" << updates.size() <<  " data size:" << command.size <<std::endl;
                
                SimpleReplyHeader_ConditionalWrite* pReply = (SimpleReplyHeader_ConditionalWrite*) outReply;
                // outReply has size of outSize where do we check outSize >= sizeof(SimpleReplyHeader_ConditionalWrite)
                memset(pReply, 0, sizeof (SimpleReplyHeader_ConditionalWrite));
                pReply->h.type = 1;
                pReply->succ = 1;
                pReply->latestBlock = roStorage.getLastBlock();
                pReply->sigLen = sigHexSize; // byte version=== 
                // sigHexSize <= 128 assert()
                memcpy(pReply->signature, thresholdSignatureHex, sigHexSize);

                
                outReplySize = sizeof (SimpleReplyHeader_ConditionalWrite);
                
                //sanity check
//                BlockId lastBlockId = roStorage.getLastBlock();
//                    
//                SetOfKeyValuePairs updates1;
//                roStorage.getBlockData(lastBlockId, updates1);
//                std::cout << "executeReadOnlyCommand(): Retreviing block " << lastBlockId << " retrieve: should be" << updates1.size() << std::endl;    
//                assert(updates1.size() >= 1);
                return true;
            }

            virtual bool executeMultiCommands(const std::vector<Slice>& commands,
                    const ILocalKeyValueStorageReadOnly& roStorage,
                    IBlocksAppender& blockAppender,
                    const size_t maxReplySize,
                    char* outReply, size_t& outReplySize,
                    uint64_t sequenceNum,
                    uint64_t viewNum,
                    bftEngine::impl::Digest40 ExDigest,
                    const char *thresholdSignature,
                    uint16_t thresholdSignatureLen) const {

                uint32_t sigHexSize = thresholdSignatureLen * 2 + 1;
                char * thresholdSignatureHex = (char *) malloc(sigHexSize);
                bin2hex(thresholdSignature, thresholdSignatureLen, thresholdSignatureHex, sigHexSize); //copy outreply to threshsignature

                // Add the entire block, use the hash as its key value;
                BlockId currBlock = roStorage.getLastBlock();
                SetOfKeyValuePairs updates;
                
                PolarisKey* compositeKey = PolarisKey::alloc(sigHexSize);
                compositeKey->seqNum = sequenceNum;
                compositeKey->viewNum = viewNum;
                compositeKey->sizeOfkey = sigHexSize;
                compositeKey->lastCycle = ExDigest.getLastCycleNumber();
                compositeKey->currentCycle = ExDigest.getCurrentCycleNumber();
                memcpy(compositeKey->thresholdSignature(), thresholdSignatureHex, sigHexSize);
                
                bftEngine::impl::Digest digestOfRequests;
                digestOfRequests.makeZero();

                uint64_t count = 0;
                for (const auto& command : commands) {
                    SLRequestMessage r;
                    r.ParseFromArray((void *) command.data, command.size);

                    std::cout << "receive exec senderId: " << r.sender_id() << std::endl;
                    std::cout << "receive exec nonce: " << r.nonce() << std::endl;
                    std::cout << "stored key size=" << compositeKey->size() << std::endl;

                    compositeKey->countInBlock = count;
                    Slice key((const char*) compositeKey, compositeKey->size());
                    Slice value((const char*) command.data, command.size);
                    KeyValuePair kv(key, value);
                    updates.insert(kv);

                    bftEngine::impl::Digest digestOfRequestsTmp;
                    bftEngine::impl::DigestUtil::compute(command.data, command.size, (char*) &digestOfRequestsTmp, sizeof (bftEngine::impl::Digest));
                    if (digestOfRequests.isZero()) {
                        digestOfRequests = digestOfRequestsTmp;
                    } else {
                        bftEngine::impl::Digest::XOROfTwoDigests(digestOfRequests, digestOfRequestsTmp, digestOfRequests);    
                    }
                    count++;
                }

                bftEngine::impl::Digest::digestOfDigest(digestOfRequests, digestOfRequests);
                auto digestAfterConcat = bftEngine::impl::concatWithCycleNumbers(digestOfRequests, ExDigest.getLastCycleNumber(), ExDigest.getCurrentCycleNumber());

                BlockId newBlockId = 0;
                Status addSucc = blockAppender.addBlock(updates, newBlockId);
                assert(addSucc.ok());
                assert(newBlockId == currBlock + 1);
                std::cout << "newBlockId=" << newBlockId << " updates=" << updates.size() <<std::endl;
                
                bftEngine::impl::Digest40 calcDigest;
                bftEngine::impl::Digest40::calcCombination(digestAfterConcat, viewNum, sequenceNum, calcDigest);
                
                // Assert after calculation of (digestOfRequests, seqNum, viewNum) is the same as digest
                std::cout << "CommitProof=" << thresholdSignatureHex << " size=" << sigHexSize << std::endl;
                printf("DigestOfReq=");
                digestOfRequests.output(stdout);
                printf("\n");
                printf("Calculated Digest=");
                calcDigest.output(stdout);
                printf("\n");
                printf("Expected Digest=");
                ExDigest.output(stdout);
                printf("\n");
                std::cout << "SeqNum=" << sequenceNum << " viewNum=" << viewNum << std::endl;
                
                SimpleReplyHeader_ConditionalWrite* pReply = (SimpleReplyHeader_ConditionalWrite*) outReply;
                // outReply has size of outSize where do we check outSize >= sizeof(SimpleReplyHeader_ConditionalWrite)
                memset(pReply, 0, sizeof (SimpleReplyHeader_ConditionalWrite));
                pReply->h.type = 1;
                pReply->succ = 1;
                pReply->latestBlock = roStorage.getLastBlock();
                pReply->sigLen = sigHexSize; // byte version=== 
                memcpy(pReply->signature, thresholdSignatureHex, sigHexSize);

                outReplySize = sizeof (SimpleReplyHeader_ConditionalWrite);
                
                return true;                

            }

            virtual bool executeCommand(const Slice command,
                    const ILocalKeyValueStorageReadOnly& roStorage,
                    IBlocksAppender& blockAppender,
                    const size_t maxReplySize,
                    char* outReply, size_t& outReplySize) const {

                //SLRequestMessage r;
                //r.ParseFromArray((void *) command.data, command.size);

                
                //std::cout << "receive exec senderId: " << r.sender_id() << std::endl;
                //std::cout << "receive exec nonce: " << r.nonce() << std::endl;
                printf("Got message of size %zu  %ld\n", command.size, outReplySize);
                
                // GEt the proof from outReply
                uint32_t sigHexSize = outReplySize * 2 + 1;
                char * thresholdSignature = (char *) malloc(sigHexSize);
                uint32_t sigLen = outReplySize;
                bin2hex(outReply, sigLen, thresholdSignature, sigHexSize); //copy outreply to threshsignature
                std::cout << "--------------->Replica sig=" << thresholdSignature << "sigLen=" << sigHexSize << std::endl;


                //                size_t size = request->ByteSizeLong(); 
                //                void *buffer = malloc(command.size);

                //                std::cout << "-----------executeCommand():" << sizeof(request) << "-----:" << request.ByteSize() << std::endl;
                //                std::cout << "-----------executeCommand():" << sizeof(request) << "-----:" << request.signature() << std::endl;
                // TODO: execute ReadOnly

                //                SimpleRequestHeader* p = (SimpleRequestHeader*) command.data;
                //                std::cout << "--------------->Tyoe" << p->type << std::endl;

                //                if (p->type != 1) return executeReadOnlyCommand(command, roStorage, maxReplySize, outReply, outReplySize);


                //                std::cout << "--------------->Tyoe Second" << p->type << std::endl;

                //                TxWriteHeaderBatch* pCondWrite = (TxWriteHeaderBatch*) command.data;
                //                char* content = pCondWrite->content();


                SLRequestMessage request;
                request.ParseFromArray((void *) command.data, command.size);
                //                std::cout << "--------------->Tyoe sig  " << request.signature() << std::endl;

                //                SimpleKey* readSetArray = pCondWrite->readSetArray();

                // Add the entire block, use the hash as its key value;
                BlockId currBlock = roStorage.getLastBlock();
                SetOfKeyValuePairs updates;
                /*
                size_t hashVal = simpleHash(command.data, command.size);
                char hashStr[256];
                snprintf(hashStr, sizeof hashStr, "%zu", hashVal);
                Slice key((const char*) hashStr, sizeof (hashStr));
                 */
                //                char threshSig[256];
                //                memcpy(threshSig, thresholdSignature, sigHexSize);
                //                snprintf(hashStr, sizeof hashStr, "%zu", hashVal);
                //                strcpy(threshSig, thresholdSignature);
                Slice key((const char*) thresholdSignature, sigHexSize);
                Slice value((const char*) command.data, command.size);
                //                std::cout << "--------------HashStr size" << key.data << "------" << sigHexSize << std::endl;
                KeyValuePair kv(key, value);
                updates.insert(kv);
                BlockId newBlockId = 0;
                Status addSucc = blockAppender.addBlock(updates, newBlockId);
                assert(addSucc.ok());
                assert(newBlockId == currBlock + 1);

                //                SimpleRequestHeader* p = (SimpleRequestHeader*) command.data;
                //                if (p->type != 1) return executeReadOnlyCommand(command, roStorage, maxReplySize, outReply, outReplySize);
                //
                //                // conditional write
                //                SimpleConditionalWriteHeader* pCondWrite = (SimpleConditionalWriteHeader*) command.data;
                //                BlockId currBlock = roStorage.getLastBlock();
                //                SimpleKV* keyValArray = pCondWrite->keyValArray();
                //                SetOfKeyValuePairs updates;
                //
                //                //printf("\nAdding BlockId=%" PRId64 " ", currBlock + 1);
                //
                //                for (size_t i = 0; i < pCondWrite->numberOfWrites; i++) {
                //                    Slice key(keyValArray[i].key, KV_LEN);
                //                    Slice val(keyValArray[i].val, KV_LEN);
                //                    KeyValuePair kv(key, val);
                //                    updates.insert(kv);
                //                }
                //                //printf("\n\n");
                //                BlockId newBlockId = 0;
                //                Status addSucc = blockAppender.addBlock(updates, newBlockId);
                //                assert(addSucc.ok());
                //                assert(newBlockId == currBlock + 1);
                //				}

                //                assert(sizeof (SimpleReplyHeader_ConditionalWrite) <= maxReplySize);
                SimpleReplyHeader_ConditionalWrite* pReply = (SimpleReplyHeader_ConditionalWrite*) outReply;
                memset(pReply, 0, sizeof (SimpleReplyHeader_ConditionalWrite));
                pReply->h.type = 1;
                pReply->succ = 1;
                pReply->latestBlock = newBlockId; //roStorage.getLastBlock();
                pReply->sigLen = sigHexSize;
                memcpy(pReply->signature, thresholdSignature, sigHexSize);

                outReplySize = sizeof (SimpleReplyHeader_ConditionalWrite);
                return true;
            }

            virtual bool executeEmptyCommand(const ILocalKeyValueStorageReadOnly& roStorage,
                    IBlocksAppender& blockAppender,
                    uint64_t sequenceNum,
                    uint64_t viewNum, 
                    bftEngine::impl::Digest40 digest, 
                    const char* thresholdSignature, 
                    uint16_t thresholdSignatureLen) const {

                // *** Mimic simpleKVBCTests.cpp, executeCommand(), line 807 ***
                std::cout << "executeEmpty(): SeqNum=" << sequenceNum << " viewNum=" << viewNum << std::endl;
                uint32_t sigHexSize = thresholdSignatureLen * 2 + 1;
                char * thresholdSignatureHex = (char *) malloc(sigHexSize);
                bin2hex(thresholdSignature, thresholdSignatureLen, thresholdSignatureHex, sigHexSize); //copy outreply to threshsignature

                BlockId currBlock = roStorage.getLastBlock();
                SetOfKeyValuePairs updates;
                
                PolarisKey* compositeKey = PolarisKey::alloc(sigHexSize);
                compositeKey->seqNum = sequenceNum;
                compositeKey->viewNum = viewNum;
                compositeKey->sizeOfkey = sigHexSize;
                memcpy(compositeKey->thresholdSignature(), thresholdSignatureHex, sigHexSize);
                
                //somehow store the seq number,viewnumber and digest, and threshold signature
                Slice key((const char*) compositeKey, compositeKey->size());
                Slice value((const char*) nullptr, 0);
                KeyValuePair kv(key, value);
                updates.insert(kv);
                BlockId newBlockId = 0;
                Status addSucc = blockAppender.addBlock(updates, newBlockId);
                assert(addSucc.ok());
                assert(newBlockId == currBlock + 1);
                std::cout << "executeEmpty(): newBlockId=" << newBlockId << " updates=" << updates.size() <<  " data size:" << 0 <<std::endl;

                return true;
            }

            virtual bool executeReadOnlyCommand(const Slice command,
                    const ILocalKeyValueStorageReadOnly& roStorage,
                    const size_t maxReplySize,
                    char* outReply, size_t& outReplySize) const {
                std::cout << "executeReadOnlyCommand(): " << std::endl;
                //                CHECKMSG(command.size >= sizeof (SimpleRequestHeader), "small message");
                SimpleRequestHeader* p = (SimpleRequestHeader*) command.data;
                if (p->type == 2) {
                    // read
                    //                    CHECKMSG(command.size >= sizeof (SimpleReadHeader), "small message");
                    //                    TxReadHeaderBatch* pRead = (TxReadHeaderBatch*) command.data;
                    //                    CHECKMSG(command.size >= pRead->size(), "small message");
                    //                    size_t numOfElements = pRead->numberOfKeysToRead;
                    //                    size_t replySize = SimpleReplyHeader_Read::size(numOfElements);

                    //                    CHECKMSG(maxReplySize >= replySize, "small message");

                    //					printf("\nRead request");  print(p);






                    TxReplyHeader_Read* pReply = (TxReplyHeader_Read*) (outReply);


                    //                    

                    //                    pReply->h.type = 2;
                    //                    pReply->numberOfElements = numOfElements;

                    BlockId lastBlockId = roStorage.getLastBlock();
                    std::cout << "executeReadOnlyCommand(): Retreviing block " << lastBlockId << std::endl;
                    SetOfKeyValuePairs updates;
                    roStorage.getBlockData(lastBlockId, updates);
                    //                    std::cout << "executeReadOnlyCommand(): Got block " << lastBlockId << " status: " << error << std::endl;

                    //                    SimpleKey* keysArray = pRead->keysArray();
                    auto i = 0;
                    for (auto it = updates.begin(); it != updates.end(); ++it) {
                        //                        const KeyValuePair& kvPair = KeyValuePair(it->first, it->second);
                        std::cout << "executeReadOnlyCommand(): Got block " << (int) lastBlockId << " soize: " << it->second.size << std::endl;
                        //                        memcpy(pReply->elements[i].key, (char *)it->first.data, KV_LEN);
                        //                        memcpy(pReply->elements[i].val, (char *) it->second.data, KV_LEN);

                        outReplySize = it->second.size + sizeof (TxReplyHeader_Read);
                        memset(pReply, 0, outReplySize);

                        //                        pReply->h.type = 2;
                        char* content = pReply->content();
                        memcpy(content, (void*) it->second.data, it->second.size);
                        pReply->h.type = 2;
                        pReply->sizeOfBlock = it->second.size;

                        i++;
                        break;
                    }

                    std::cout << "executeReadOnlyCommand(): Got data " << (int) pReply->sizeOfBlock << " key: " << std::endl;

                    //                    SimpleKey* keysArray = pRead->keysArray();
                    //                    for (size_t i = 0; i < numOfElements; i++) {
                    //                        memcpy(pReply->elements[i].key, keysArray[i].key, KV_LEN);
                    //                        Slice val;
                    //                        Slice k(keysArray[i].key, KV_LEN);
                    //                        BlockId outBlock = 0;
                    //                        roStorage.get(pRead->readVersion, k, val, outBlock);
                    //                        if (val.size > 0)
                    //                            memcpy(pReply->elements[i].val, val.data, KV_LEN);
                    //                        else
                    //                            memset(pReply->elements[i].val, 0, KV_LEN);
                    //                    }

                    //					printf("\nRead reply");  print((SimpleReplyHeader*)pReply);

                    return true;


                } else if (p->type == 3) {
                    // read
                    CHECKMSG(command.size >= sizeof (SimpleGetLastBlockHeader), "small message");
                    //					SimpleGetLastBlockHeader* pGetLast = (SimpleGetLastBlockHeader*)command.data;

                    CHECKMSG(maxReplySize >= sizeof (SimpleReplyHeader_GetLastBlockHeader), "small message");
                    SimpleReplyHeader_GetLastBlockHeader* pReply = (SimpleReplyHeader_GetLastBlockHeader*) (outReply);
                    outReplySize = sizeof (SimpleReplyHeader_GetLastBlockHeader);
                    memset(pReply, 0, sizeof (SimpleReplyHeader_GetLastBlockHeader));
                    pReply->h.type = 3;
                    pReply->latestBlock = roStorage.getLastBlock();

                    return true;
                } else {
                    outReplySize = 0;
                    CHECKMSG(false, "illegal message");
                    return false;
                }
            }


        };

        static size_t sizeOfReq(SimpleRequestHeader* req) {
            if (req->type == 1) {
                SimpleConditionalWriteHeader* p = (SimpleConditionalWriteHeader*) req;
                return p->size();
            } else if (req->type == 2) {
                SimpleReadHeader* p = (SimpleReadHeader*) req;
                return p->size();
            } else if (req->type == 3) {
                return SimpleGetLastBlockHeader::size();
            }
            assert(0);
            return 0;
        }

        static size_t sizeOfRep(SimpleReplyHeader* rep) {
            if (rep->type == 1) {
                return sizeof (SimpleReplyHeader_ConditionalWrite);
            } else if (rep->type == 2) {
                SimpleReplyHeader_Read* p = (SimpleReplyHeader_Read*) rep;
                return p->size();
            } else if (rep->type == 3) {
                return sizeof (SimpleReplyHeader_GetLastBlockHeader);
            }
            assert(0);
            return 0;
        }

        static void verifyEmptyBlockchain(IClient* client) {
            SimpleGetLastBlockHeader* p = SimpleGetLastBlockHeader::alloc();
            p->h.type = 3;
            Slice command((const char*) p, sizeof (SimpleGetLastBlockHeader));
            Slice reply;

            client->invokeCommandSynch(command, true, reply);

            assert(reply.size == sizeof (SimpleReplyHeader_GetLastBlockHeader));

            SimpleReplyHeader_GetLastBlockHeader* pReplyData = (SimpleReplyHeader_GetLastBlockHeader*) reply.data;
            (void) pReplyData;

            assert(pReplyData->h.type == 3);
            assert(pReplyData->latestBlock == 0);

            client->release(reply);
        }
    }

    void run(IClient* client, const size_t numOfOperations) {
        assert(!client->isRunning());

        std::list<Internal::SimpleRequestHeader*> requests;
        std::list<Internal::SimpleReplyHeader*> expectedReplies;

        Internal::InternalTestsBuilder::createRandomTest(numOfOperations, 1111, INT64_MIN /* INT64_MAX */, requests, expectedReplies);

        client->start();

        Internal::verifyEmptyBlockchain(client);

        assert(requests.size() == expectedReplies.size());

        int ops = 0;

        while (!requests.empty()) {
#ifndef _WIN32
            if (ops % 100 == 0) usleep(100 * 1000);
#endif   
            Internal::SimpleRequestHeader* pReq = requests.front();
            Internal::SimpleReplyHeader* pExpectedRep = expectedReplies.front();
            requests.pop_front();
            expectedReplies.pop_front();

            bool readOnly = (pReq->type != 1);
            size_t expectedReplySize = Internal::sizeOfRep(pExpectedRep);

            Slice command((const char*) pReq, Internal::sizeOfReq(pReq));
            Slice reply;

            client->invokeCommandSynch(command, readOnly, reply);

            bool equiv = (reply.size == expectedReplySize);

            if (equiv)
                equiv = (memcmp(reply.data, pExpectedRep, expectedReplySize) == 0);

            //			if (!equiv)	{
            //				print(pReq);
            //				print(pExpectedRep);
            //				print((Internal::SimpleReplyHeader*)reply.data());
            //				assert(0); 
            //			}				

            CHECKMSG(equiv, "actual reply != expected reply");

            if (equiv) {
                ops++;
                if (ops % 20 == 0) printf("\nop %d passed", ops);
            }

            client->release(reply);
        }

        client->stop();

        Internal::InternalTestsBuilder::free(requests, expectedReplies);
    }

    ICommandsHandler* commandsHandler() {
        return new Internal::InternalCommandsHandler();
    }
}
