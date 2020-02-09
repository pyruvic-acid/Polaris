#pragma once

#include <cstdlib>
#include <cstring>
#include <cstdint>

struct PolarisKey {
    uint64_t seqNum;
    uint64_t viewNum;
    uint64_t countInBlock;
    uint32_t lastCycle;
    uint32_t currentCycle;
    size_t sizeOfkey;

    static PolarisKey* alloc(size_t sizeOfkey) {

        size_t bufSize = sizeof (PolarisKey) + sizeOfkey;
        char* pBuf = new char[bufSize];
        memset(pBuf, 0, bufSize);
        return (PolarisKey*) (pBuf);
    }

    static size_t size(size_t sizeOfkey) {
        // total size = size of the header + #TxKV*writes i.e. the TxBlock
        return sizeof (PolarisKey) + sizeOfkey;
    }

    size_t size() {
        return size(sizeOfkey);
    }

    char* thresholdSignature() {
        return (char*) (((char*) this) + sizeof (PolarisKey));
        //        return key;
    }

    size_t signatureLen() {
        return sizeOfkey;
    }

    static void free(PolarisKey* p) {
        char* p1 = (char*) p;
        delete[] p1;
    }
};
