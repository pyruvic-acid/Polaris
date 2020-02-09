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

#include <fstream>
#include <iostream>
#include <unordered_map>
#include <unordered_set>

#include "bftengine/Crypto.hpp"
#include "threshsign/ThresholdSignaturesTypes.h"
#include "KeyfileIOUtils.hpp"
#include "Utils.h"
#include "Digest.hpp"

#include <grpcpp/grpcpp.h>

#ifdef BAZEL_BUILD
#include "polaris.grpc.pb.h"
#else
#include "polaris.grpc.pb.h"
#endif

using namespace BLS::Relic;
using namespace bftEngine::impl;


using grpc::Server;
using grpc::ServerBuilder;
using grpc::ServerContext;
using grpc::Status;

using rpc::Polaris;
using rpc::LeaderReply;
using rpc::VerifyStatus;
using rpc::VerifyRequest;

//static const uint16_t base_port_rpc_ = 50101;
std::string port;


// How often to output status when testing cryptosystems, measured as an
// interval measured in tested signaturs.
static const size_t kTestProgressReportingInterval = 128;

// Hash(es) to test for each signer combination that will be validated.
static const std::string kHashesToTest[1] = {
    "986699702ad64879360bbaccf7df4b2255a777f85a57c699c895992ea6fff2ac3c5e386c634f"
    "9f9588be531154368468d8f454e82bee8d780ea13f7466f90a30ae0627fab054798ab217dfd6"
    "2a988ca5875b555df5392b07664dcf4e947034f66e0ec51a9e469d78badf9c9c357d6ea14bcf"
    "cd86d5f23e7f19ed11f9c18c859a9600320700d23e6a935fbf58c2310aeba2faaac00a7075de"
    "02e6fd1cc94448daa7ead2aa6b4750b571211b01af392e8f100c0cd8011709690c2ef44847a6"
    "47f60cfd8dd0005cd39fb599f9f8ad5be5a939c542707fc3d7c55c1b4e8dcde5ab02b9c9f421"
    "2ae2d9bad10bf6299e2e02979e2c6ecc4d9989986d01ef58da511f671e983fe16169895534c3"
    "ead052e9fd0d3b2963e64601362cc441c0e3a5c5a892edb3dab2b8db54c8d2025e4e9ba2aef1"
    "ba2054751c7473d47fac4ccc407752fcbb30596c1815e806b3a88860e5e0d1f340556b55fb19"
    "4b3b9761108bc14c33b4f957d5c3cbb74f7f7531dfbd37e14e773d453ea51209d485d1623df9"
    "13875038247edb228b7b34075ee93d970f73e768fc2982c33e091a1a29fd2c29760b46aa1a63"
    "d26992682b906b96f98dc5140babeb98dbccadceaf8f5e31b6f0906c773b6426df866dae32a2"
    "1d88c54ee130f6f60761bc49a5b0864ac8f89bd464e5e2c8e8e8938570a47933d83e934d6b39"
    "6e45479128168ce36f61880e93038a7895b1fb4303b50a59b21f92c721ca7ebdafcd96b9f23e"
    "d6b0e276417034a564061c3d8c055afd9afbf6d933b41504c9fa39fe7c100ad538ea41be536c"
    "b1e4e7eee0644c4d02beb00ff2be77ed84e17e65105c25a5caf74aca6c4204df6cacf07fa914"
    "3260767dd8f6c8b794b16cf22cf2353da51840e0d20348b48907c44da481298dfa3f45b55d08"
    "69acd530f82d06542970d15753ae1806a4229341b7af785175998944d5b575b00196fa01ed10"
    "f4ffef06912a1b5964eb0604c8633929e7e056bdeb814cd0a719149c164a464bbc855e38f9aa"
    "7bd19505dd85e487a55fff1bfc579c12f3816e87776273c6e3e72222c6a61132fac441e3af3b"
    "db465f44dac867c66c2e83d925cdc976ebac4569945532ffbed26693ec61ad54b2897097dd67"
    "88e6d5da2390a2cf0842783779e39478a91b06c32911fdd3466562a4cef2ff490bba670e20fe"
    "6122a4d936c703e9cf6f5847c4d4e202074326ad953b37d97264c9b7d36ba26413d14ca38108"
    "f4ceabe84b52768653168c54ff4d25c05c99c653e3cd606b87d5dbae249f57dc406969a5fcf0"
    "eb3b1a893c5111853bed1cc60fe074a55aa51a975cc3217594ff1479036a05d01c01b2f610b4"
    "e4adbc629e06dc636b65872072cdf084ee5a7e0a63afe42931025d4e5ed60d069cfa71d91bc7"
    "451a8a2109529181fd150fc055ad42ce5c30c9512cd64ee3789f8c0d0069501be65f1950"
};

enum class CommitPath {
    NA = -1,
    OPTIMISTIC_FAST = 0,
    FAST_WITH_THRESHOLD = 1,
    SLOW = 2
};

// Helper functions to main acting as sub-components of the test.

// validateFundamentalFields and validateConfigStructIntegrity are sanity checks
// for the partially-filled-out ReplicaConfig structs read from the keyfiles to
// validate they are in order before attempting tests of the actual
// cryptographic keys.

static bool validateFundamentalFields(
        const std::vector<bftEngine::ReplicaConfig>& configs) {
    uint16_t numReplicas = configs.size();
    uint16_t fVal = configs.front().fVal;
    uint16_t cVal = configs.front().cVal;

    // Note we validate agreement of numReplicas, fVal, and cVal with 32-bit
    // integers in case 3F + 2C + 1 overflows a 16-bit integer.
    uint32_t expectedNumReplicas = 3 * (uint32_t) fVal + 2 * (uint32_t) cVal + 1;

    // Note that check fVal >= 1 before enforcing agreement of fVal and cVal with
    // numReplicas to give precedence to complaining about invalid F over
    // disagreement with numReplicas.
    if ((fVal >= 1) && (expectedNumReplicas != (uint32_t) numReplicas)) {
        std::cout << "Verifier: FAILURE: fVal (" << fVal << ") and cVal ("
                << cVal << ") (according to key file 0) do not agree with the number"
                " of replicas (" << numReplicas << "). It is required that numReplicas ="
                " (3 * fVal + 2 * cVal + 1).\n";
        return false;
    }

    for (uint16_t i = 0; i < numReplicas; ++i) {
        const bftEngine::ReplicaConfig& config = configs[i];
        if (config.replicaId != i) {
            std::cout << "Verifier: FAILURE: Key file " << i
                    << " specifies a replica ID disagreeing with its filename.\n";
            return false;
        }
        if (config.fVal < 1) {
            std::cout << "Verifier: FAILURE: Replica " << i << " has an"
                    " invalid F value: " << config.fVal << ".\n.";
            return false;
        }
        if (config.fVal != fVal) {
            std::cout << "Verifier: FAILURE: Replica " << i << " has an F"
                    " value inconsistent with replica(s) 0 through " << (i - 1) << ".\n";
            return false;
        }
        if (config.cVal != cVal) {
            std::cout << "Verifier: FAILURE: Replica " << i << " has a C"
                    " value inconsistent with replica(s) 0 through " << (i - 1) << ".\n";
            return false;
        }
    }

    return true;
}

static bool validateConfigStructIntegrity(
        const std::vector<bftEngine::ReplicaConfig>& configs) {
    uint16_t numReplicas = configs.size();

    for (uint16_t i = 0; i < numReplicas; ++i) {
        const bftEngine::ReplicaConfig& config = configs[i];
        if (config.publicKeysOfReplicas.size() != numReplicas) {
            std::cout << "Verifier: FAILURE: Size of the set of public keys"
                    " of replicas in replica " << i << "'s key file does not match the"
                    " number of replicas.\n";
            return false;
        }

        std::set<uint16_t> foundIDs;
        for (auto entry : config.publicKeysOfReplicas) {
            uint16_t id = entry.first;
            if (id >= numReplicas) {
                std::cout << "Verifier: FAILURE: Entry with invalid replica"
                        " ID (" << id << ") in set of public keys of replicas for replica "
                        << i << ".\n";
                return false;
            }
            if (foundIDs.count(id) > 0) {
                std::cout << "Verifier: FAILURE: Set of public keys of"
                        " replicas for replica " << i << " contains duplicate entries for"
                        " replica " << id << ".\n";
                return false;
            }
            foundIDs.insert(id);
        }

        if (!config.thresholdSignerForExecution) {
            std::cout << "Verifier: FAILURE: No threshold signer for"
                    " execution for replica " << i << ".\n";
            return false;
        }
        if (!config.thresholdVerifierForExecution) {
            std::cout << "Verifier: FAILURE: No threshold verifier for"
                    " execution for replica " << i << ".\n";
            return false;
        }
        if (!config.thresholdSignerForSlowPathCommit) {
            std::cout << "Verifier: FAILURE: No threshold signer for slow"
                    " path commit for replica " << i << ".\n";
            return false;
        }
        if (!config.thresholdVerifierForSlowPathCommit) {
            std::cout << "Verifier: FAILURE: No threshold verifier for slow"
                    " path commit for replica " << i << ".\n";
            return false;
        }
        if (!config.thresholdSignerForCommit) {
            std::cout << "Verifier: FAILURE: No threshold signer for commit"
                    " for replica " << i << ".\n";
            return false;
        }
        if (!config.thresholdVerifierForCommit) {
            std::cout << "Verifier: FAILURE: No threshold verifier for"
                    " commit for replica " << i << ".\n";
            return false;
        }
        if (!config.thresholdSignerForOptimisticCommit) {
            std::cout << "Verifier: FAILURE: No threshold signer for"
                    " optimistic commit for replica " << i << ".\n";
            return false;
        }
        if (!config.thresholdVerifierForOptimisticCommit) {
            std::cout << "Verifier: FAILURE: No threshold verifier for"
                    " optimistic commit for replica " << i << ".\n";
            return false;
        }
    }

    return true;
}

// Helper function to test RSA keys to test the compatibility of a single key
// pair.
static bool testRSAKeyPair(const std::string& privateKey,
        const std::string& publicKey,
        uint16_t replicaID) {

    // The signer and verifier are stored with unique pointers rather than by
    // value so that they can be constructed in try/catch statements without
    // limiting their scope to those statements; declaring them by value is not
    // possible in this case becuause they lack paramter-less default
    // constructors.
    std::unique_ptr<bftEngine::impl::RSASigner> signer;
    std::unique_ptr<bftEngine::impl::RSAVerifier> verifier_opt_fast;

    std::string invalidPrivateKey = "Verifier: FAILURE: Invalid RSA"
            " private key for replica " + std::to_string(replicaID) + ".\n";
    std::string invalidPublicKey = "Verifier: FAILURE: Invalid RSA"
            " public key for replica " + std::to_string(replicaID) + ".\n";

    try {
        signer.reset(new bftEngine::impl::RSASigner(privateKey.c_str()));
    } catch (std::exception e) {
        std::cout << invalidPrivateKey;
        return false;
    }
    try {
        verifier_opt_fast.reset(new bftEngine::impl::RSAVerifier(publicKey.c_str()));
    } catch (std::exception e) {
        std::cout << invalidPublicKey;
        return false;
    }

    for (auto iter = std::begin(kHashesToTest); iter != std::end(kHashesToTest);
            ++iter) {
        const std::string& hash = *iter;

        size_t signatureLength;
        try {
            signatureLength = signer->signatureLength();
        } catch (std::exception e) {
            std::cout << invalidPrivateKey;
            return false;
        }
        size_t returnedSignatureLength;
        char* signatureBuf = new char[signatureLength];

        try {
            if (!signer->sign(hash.c_str(), hash.length(), signatureBuf,
                    signatureLength, returnedSignatureLength)) {
                std::cout << "Verifier: FAILURE: Failed to sign data with"
                        " replica " << replicaID << "'s RSA private key.\n";
                delete[] signatureBuf;
                return false;
            }
        } catch (std::exception e) {
            std::cout << invalidPrivateKey;
            delete[] signatureBuf;
            return false;
        }

        try {
            if (!verifier_opt_fast->verify(hash.c_str(), hash.length(), signatureBuf,
                    returnedSignatureLength)) {
                std::cout << "Verifier: FAILURE: A signature with replica "
                        << replicaID << "'s RSA private key could not be verified with"
                        " replica " << replicaID << "'s RSA public key.\n";
                delete[] signatureBuf;
                return false;
            }
        } catch (std::exception e) {
            std::cout << invalidPublicKey;
            delete[] signatureBuf;
            return false;
        }

        delete[] signatureBuf;
    }

    return true;
}

// Test that the RSA key pairs given in the keyfiles work, that the keyfiles
// agree on what the public keys are, and that there are no duplicates.

static bool testRSAKeys(const std::vector<bftEngine::ReplicaConfig>& configs) {
    uint16_t numReplicas = configs.size();

    std::cout << "Testing " << numReplicas << " RSA key pairs...\n";
    std::unordered_map<uint16_t, std::string> expectedPublicKeys;
    std::unordered_set<std::string> rsaPublicKeysSeen;

    // Test that a signature produced with each replica's private key can be
    // verified with that replica's public key.
    for (uint16_t i = 0; i < numReplicas; ++i) {
        std::string privateKey = configs[i].replicaPrivateKey;
        std::string publicKey;

        for (auto publicKeyEntry : configs[i].publicKeysOfReplicas) {
            if (publicKeyEntry.first == i) {
                publicKey = publicKeyEntry.second;
            }
        }

        if (!testRSAKeyPair(privateKey, publicKey, i)) {
            return false;
        }

        if (rsaPublicKeysSeen.count(publicKey) > 0) {
            uint16_t existingKeyholder;
            for (auto publicKeyEntry : expectedPublicKeys) {
                if (publicKeyEntry.second == publicKey) {
                    existingKeyholder = publicKeyEntry.first;
                }
            }
            std::cout << "Verifier: FAILURE: Replicas " << existingKeyholder
                    << " and " << i << " share the same RSA public key.\n";
            return false;
        }
        expectedPublicKeys[i] = publicKey;

        if (((i + 1) % kTestProgressReportingInterval) == 0) {
            std::cout << "Tested " << (i + 1) << " out of " << numReplicas
                    << " RSA key pairs...\n";
        }
    }

    std::cout << "Verifying that all replicas agree on RSA public keys...\n";

    // Verify that all replicas' keyfiles agree on the RSA public keys.
    for (uint16_t i = 0; i < numReplicas; ++i) {
        for (auto publicKeyEntry : configs[i].publicKeysOfReplicas) {
            if (publicKeyEntry.second != expectedPublicKeys[publicKeyEntry.first]) {
                std::cout << "Verifier: FAILURE: Replica " << i << " has an"
                        " incorrect RSA public key for replica " << publicKeyEntry.first
                        << ".\n";
                return false;
            }
        }
    }

    std::cout << "All RSA key tests were successful.\n";

    return true;
}

// Function to simplify the process of freeing the dynamically allocated memory
// referenced by each ReplicaConfig struct, which the main function below may
// need to do in one of several places because it may return early in the event
// of a failure.

static void freeConfigs(const std::vector<bftEngine::ReplicaConfig>& configs) {
    for (auto config : configs) {
        if (config.thresholdSignerForExecution) {
            delete config.thresholdSignerForExecution;
        }
        if (config.thresholdVerifierForExecution) {
            delete config.thresholdVerifierForExecution;
        }
        if (config.thresholdSignerForSlowPathCommit) {
            delete config.thresholdSignerForSlowPathCommit;
        }
        if (config.thresholdVerifierForSlowPathCommit) {
            delete config.thresholdVerifierForSlowPathCommit;
        }
        if (config.thresholdSignerForCommit) {
            delete config.thresholdSignerForCommit;
        }
        if (config.thresholdVerifierForCommit) {
            delete config.thresholdVerifierForCommit;
        }
        if (config.thresholdSignerForOptimisticCommit) {
            delete config.thresholdSignerForOptimisticCommit;
        }
        if (config.thresholdVerifierForOptimisticCommit) {
            delete config.thresholdVerifierForOptimisticCommit;
        }
    }
}

// Helper function for determining whether --help was given.

static bool containsHelpOption(int argc, char** argv) {
    for (int i = 1; i < argc; ++i) {
        if (std::string(argv[i]) == "--help") {
            return true;
        }
    }
    return false;
}

class SigVerifierImpl {
private:
    std::vector<bftEngine::ReplicaConfig> configs;
    IThresholdVerifier* verifier_opt_fast;
    IThresholdAccumulator* accum_opt_fast;

    IThresholdVerifier* verifier_slow;
    IThresholdAccumulator* accum_slow;
    
public:

    SigVerifierImpl(std::vector<bftEngine::ReplicaConfig> configs)
    : configs(configs) {
        int idx = 0;

        verifier_opt_fast = configs[idx].thresholdVerifierForOptimisticCommit;
        accum_opt_fast = verifier_opt_fast->newAccumulator(true); //true -- has share verification enabled - uses multisig..
  
        verifier_slow = configs[idx].thresholdVerifierForSlowPathCommit;
        accum_slow = verifier_slow->newAccumulator(true);
        
    }

    ~SigVerifierImpl() {
    }

    bool checkThresholdSignature_opt_fast(char* messageDigest, int msgSize, std::string threshSig) {

        int sigSize = static_cast<int> (threshSig.size() / 2);
        //        int msgSize = static_cast<int> (messageDigest.size() / 2);
        unsigned char* sig = new unsigned char[sigSize];
        //        unsigned char* msg = new unsigned char[msgSize];

        Utils::hex2bin(threshSig, sig, sigSize);
        
        //if it is multisig-- the thressig should be appended with vector share bits == so it should be fine
        //if it is blsthreshold -- it will ignore it and compute based on the sigBits, --- This may break if it has vector of shares appeneded!!
        bool signatureValid = verifier_opt_fast->verify(reinterpret_cast<char *> (messageDigest), msgSize, reinterpret_cast<char *> (sig), sigSize);
        if (signatureValid) {
            std::cout << " Signature is valid in fast path" << std::endl;
        } else {
            std::cout << " Signature is invalid in fast path" << std::endl;
        }
        return signatureValid;
    }

    bool checkThresholdSignature_slow(char* messageDigest, int msgSize, std::string threshSig) {

        int sigSize = static_cast<int> (threshSig.size() / 2);
        unsigned char* sig = new unsigned char[sigSize];

        Utils::hex2bin(threshSig, sig, sigSize);

        //if it is multisig-- the thressig should be appended with vector share bits == so it should be fine
        //if it is blsthreshold -- it will ignore it and compute based on the sigBits, --- This may break if it has vector of shares appeneded!!
        bool signatureValid = verifier_slow->verify(reinterpret_cast<char *> (messageDigest), msgSize, reinterpret_cast<char *> (sig), sigSize);
        if (signatureValid) {
            std::cout << " Signature is valid in slow path" << std::endl;
        } else {
            std::cout << " Signature is invalid in slow path" << std::endl;
        }
        return signatureValid;
    }

    Digest40 combineDigest_fast(std::string digestOfSLRequest, int64_t viewNum, int64_t seqNum) {
        int msgSize = static_cast<int> (digestOfSLRequest.size() / 2);
        unsigned char* msgBin = new unsigned char[msgSize];
        //convert to byte
        Utils::hex2bin(digestOfSLRequest, msgBin, msgSize);
        Digest40 ppDigest((char*) msgBin);
        Digest40 digestRet;
        
        Digest40::calcCombination(ppDigest, viewNum, seqNum, digestRet);
        printf("Fast ComputedexpectedDigest=");
        digestRet.output(stdout);
        printf("\n");
        return digestRet;
    }

    Digest40 combineDigest_slow(std::string digestOfSLRequest, int64_t viewNum, int64_t seqNum) {
        int msgSize = static_cast<int> (digestOfSLRequest.size() / 2);
        unsigned char* msgBin = new unsigned char[msgSize];
        //convert to byte
        Utils::hex2bin(digestOfSLRequest, msgBin, msgSize);
        Digest40 ppDigest((char*) msgBin);

        Digest40 ppDoubleDigest;
        Digest40::digestOfDigest(ppDigest, ppDoubleDigest);

        Digest40 digestRet;
        Digest40::calcCombination(ppDoubleDigest, viewNum, seqNum, digestRet);

        printf("Slow ComputedexpectedDigest=");
        digestRet.output(stdout);
        printf("\n");
        return digestRet;
    }

};

class SigVerifierService final : public Polaris::Service {
private:
    SigVerifierImpl* sigImpl;

public:

    SigVerifierService(SigVerifierImpl* s) {
        sigImpl = s;
    }

    ~SigVerifierService() {
    }

    grpc::Status Verify(ServerContext* context, const VerifyRequest* request, VerifyStatus* status) override {

        std::cout << "received request - seqNum: " << request->sequence() << std::endl;

        uint64_t seqNum = request->sequence();
        uint64_t viewNum = request->view();
        std::string commitProofVectorOfShares = request->commit_proof();
        std::string digestOfSLRequest = request->request_digest();

        
        // convert 
        Digest40 computedDigest_fast = sigImpl->combineDigest_fast(digestOfSLRequest, viewNum, seqNum);
        
        
        std::cout << "seqNum=" << seqNum << " viewNum=" << viewNum << " digestOfSLRequest="
                << digestOfSLRequest << " proof=" << commitProofVectorOfShares << std::endl;
        
        bool isValid = sigImpl->checkThresholdSignature_opt_fast(computedDigest_fast.content(), sizeof(computedDigest_fast), commitProofVectorOfShares);
        // try slow path verification if fast path invalid
        if (!isValid) {
            Digest40 computedDigest_slow = sigImpl->combineDigest_slow(digestOfSLRequest, viewNum, seqNum);

            isValid = sigImpl->checkThresholdSignature_slow(computedDigest_slow.content(), sizeof(computedDigest_slow), commitProofVectorOfShares);
        }

        status->set_status(isValid);

        return grpc::Status::OK;
    }

};

void testSigVerifierService(std::vector<bftEngine::ReplicaConfig> configs) {
    std::cout << "Testing sig verifier_opt_fast " << std::endl;
    CommitPath commitPath = CommitPath::OPTIMISTIC_FAST; //concord uses optimistic fast
    SigVerifierImpl sigVerifier(configs);
    // TEST case

    //  sha256=486e092757e687561f20f0e30ae29dbae387e88b9ad62ed20a3a89dcd93ee263
    //  CommitProof=030e75e496bcb94aa25136ec60d27d094e11182b5457099f1dbe4c8d4b6dc76340 size=67
    //  Ex Digest=496e092757e687561f20f0e30ae29dbae387e88b9ad62ed20a3a89dcd93ee263
    //  DigestOfReq=486e092757e687561f20f0e30ae29dbae387e88b9ad62ed20a3a89dcd93ee263
    //  SeqNum = 1 viewNum = 0

    std::string expectedDigest = "0000000000000000496e092757e687561f20f0e30ae29dbae387e88b9ad62ed20a3a89dcd93ee263";
    std::string commitProofVectorOfShares = "030e75e496bcb94aa25136ec60d27d094e11182b5457099f1dbe4c8d4b6dc76340";
    std::string digestOfSLRequest = "0000000000000000486e092757e687561f20f0e30ae29dbae387e88b9ad62ed20a3a89dcd93ee263";
    int64_t seqNum = 1;
    int64_t viewNum = 0;

    //binary digest -- digest.content()
    Digest40 computedDigest_fast = sigVerifier.combineDigest_fast(digestOfSLRequest, viewNum, seqNum);
    
    std::cout << "expectedDigest=" << expectedDigest;
    printf(" Computed Digest=");
    computedDigest_fast.output(stdout);
    printf("\n");
    int digestSize = static_cast<int> (digestOfSLRequest.size() / 2);

    sigVerifier.checkThresholdSignature_opt_fast(computedDigest_fast.content(), digestSize, commitProofVectorOfShares);
}

void RunServer(std::vector<bftEngine::ReplicaConfig> configs) {

    //std::string port(std::to_string(base_port_rpc_));
    std::string server_address = "0.0.0.0:" + port;
    //initalize the service with the GRPC client
    std::cout << "Starting GRPC on " << server_address << std::endl;
    SigVerifierImpl* sigImpl;
    sigImpl = new SigVerifierImpl(configs);

    SigVerifierService service(sigImpl);
    std::cout << "Initiing builder " << server_address << std::endl;
    ServerBuilder builder;
    // Listen on the given address without any authentication mechanism.
    builder.AddListeningPort(server_address, grpc::InsecureServerCredentials());
    // Register "service" as the instance through which we'll communicate with
    // clients. In this case it corresponds to an *synchronous* service.
    builder.RegisterService(&service);
    // Finally assemble the server.
    std::unique_ptr<Server> server(builder.BuildAndStart());
    std::cout << "Replica GRPC Server listening on " << server_address << std::endl;

    // Wait for the server to shutdown. Note that some other thread must be
    // responsible for shutting down the server for this call to ever return.
    server->Wait();
}

/**
 * Main function for the Verifier executable. Verifier is
 * intended as a test for the correctness of GenerateConcordKeys, although it
 * can also validate a set of keyfiles that have been manually created or
 * modified as long as they comply with the output keyfile naming convention
 * that GenerateConcordKeys uses.
 *
 * @param argc The number of arguments to this executable, including the name by
 *             which this executable was launched as the first by convention.
 * @param argv Command line arguments to this executable, including the name by
 *             which it was launched as the first argument per convention. This
 *             executable can be run with the --help option for usage
 *             information on what command line arguments are expected.
 *
 * @return 0 if the testing is launched and completed successfully; -1 if the
 *         tests could not be launched (for example, because of issue with the
 *         command line parameters or missing input files) or if the input key
 *         files fail the tests.
 */
int main(int argc, char** argv) {
    std::string usageMessage =
            "Usage:\n"
            "  Verifier -n TOTAL_NUMBER_OF_REPLICAS -o KEYFILE_PREFIX.\n"
            "Verifier is intended to test the output of the\n"
            "GenerateConcordKeys utility; Verifier expects that\n"
            "GenerateConcordKeys has already been run and produced keyfiles for it to\n"
            "test. TOTAL_NUMBER_OF_REPLICAS should specify how many replicas there\n"
            "are in the deployment that Verifier has generated keys for, and\n"
            "KEYFILE_PREFIX should be the prefix passed to GenerateConcordKeys via\n"
            "its -o option that all. Verifier expects to find\n"
            "TOTAL_NUMBER_OF_REPLICAS keyfiles each named KEYFILE_PREFIX<i>, where\n"
            "<i> is an integer in the range [0, TOTAL_NUMBRER_OF_REPLICAS - 1],\n"
            "inclusive.\n\n"
            "Verifier can be run with no options or with the --help option\n"
            "to view this usage text.\n";

    // Output the usage message if no arguments were given. Note that argc should
    // equal 1 in that case, as the first argument is the command used to run this
    // executable by convention.
    if ((argc <= 1) || (containsHelpOption(argc, argv))) {
        std::cout << usageMessage;
        return 0;
    }

    uint16_t numReplicas;
    std::string outputPrefix;

    bool hasNumReplicas;
    bool hasOutputPrefix;
    bool hasPort;

    // Read input from the command line.
    // Note we ignore argv[0] because it contains the command that was used to run
    // this executable by convention.
    for (int i = 1; i < argc; ++i) {
        std::string option(argv[i]);

        if (option == "-n") {
            if (i >= argc - 1) {
                std::cout << "Expected an argument to -n.\n";
                return -1;
            }
            std::string arg = argv[i + 1];
            long long unvalidatedNumReplicas;

            std::string errorMessage = "Invalid value for -n; -n must be a positive"
                    " integer not exceeding " + std::to_string(UINT16_MAX) + ".\n";

            try {
                unvalidatedNumReplicas = std::stoll(arg);
            } catch (std::invalid_argument e) {
                std::cout << errorMessage;
                return -1;
            } catch (std::out_of_range e) {
                std::cout << errorMessage;
                return -1;
            }
            if ((unvalidatedNumReplicas < 1)
                    || (unvalidatedNumReplicas > UINT16_MAX)) {
                std::cout << errorMessage;
                return -1;
            } else {
                numReplicas = (uint16_t) unvalidatedNumReplicas;
                hasNumReplicas = true;
            }
            ++i;

        } else if (option == "-o") {
            if (i >= argc - 1) {
                std::cout << "Expected an argument to -o.\n";
                return -1;
            }
            outputPrefix = argv[i + 1];
            hasOutputPrefix = true;
            ++i;
        } else if (option == "-p") {
            if (i >= argc - 1) {
                std::cout << "Expected an argument to -p.\n";
                return -1;
            }
            port = argv[i + 1];
            hasPort = true;
            ++i; 

        } else {
            std::cout << "Unrecognized command line option: " << option << ".\n";
            return -1;
        }
    }

    if (!hasNumReplicas) {
        std::cout << "No value given for required -n parameter.\n";
        return -1;
    }
    if (!hasOutputPrefix) {
        std::cout << "No value given for required -o parameter.\n";
        return -1;
    }
    if (!hasPort) {
        std::cout << "No vaule given for required -p parameter.\n";
        return -1;
    }

    std::cout << "Verifier launched.\n";
    // initialize accumulator and verifier 

    // 



    std::cout << "Testing keyfiles for a " << numReplicas << "-replica Concord"
            " deployment...\n";

    std::vector<bftEngine::ReplicaConfig> configs(numReplicas);

    // Verify that all keyfiles exist before possibly wasting a significant
    // ammount of time parsing an incomplete set of files.
    std::vector<std::ifstream> inputFiles;
    for (uint16_t i = 0; i < numReplicas; ++i) {
        std::string filename = outputPrefix + std::to_string(i);
        inputFiles.push_back(std::ifstream(filename));
        if (!inputFiles.back().is_open()) {
            std::cout << "Verifier: FAILURE: Could not open keyfile "
                    << filename << ".\n";
            return -1;
        }
    }

    for (uint16_t i = 0; i < numReplicas; ++i) {
        std::string filename = outputPrefix + std::to_string(i);
        if (!inputReplicaKeyfile(inputFiles[i], filename, configs[i])) {
            std::cout << "Verifier: FAILURE: Failed to input keyfile "
                    << filename << "; this keyfile is invalid.\n";
            freeConfigs(configs);
            return -1;
        } else {
            std::cout << "Succesfully input keyfile " << filename << ".\n";
        }
    }

    std::cout << "All keyfiles were input successfully.\n";

    std::cout << "Verifying sanity of the cryptographic configuratigurations read"
            " from the keyfiles...\n";
    if (!validateFundamentalFields(configs)) {
        freeConfigs(configs);
        return -1;
    }
    if (!validateConfigStructIntegrity(configs)) {
        freeConfigs(configs);
        return -1;
    }
    std::cout << "Cryptographic configurations read appear to be sane.\n";
    std::cout << "Testing key functionality and agreement...\n";
    if (!testRSAKeys(configs)) {
        freeConfigs(configs);
        return -1;
    }

    std::cout << "Testing with the default keys. If change in keys, comment out the test!\n";

    testSigVerifierService(configs);

    std::cout << "Running server..." << std::endl;

    RunServer(configs);

    return 0;
}
